// Routeurs essayés dans l'ordre quand aucun endpoint n'est imposé dans la popup :
// le relais Flask de cette machine, puis le PC fixe par Tailscale (interface,
// port 5173, qui transmet à l'API Go). Sur le PC fixe, le premier répond ; sur
// un portable, le second.
const DEFAULT_STATE_ENDPOINTS = [
  "http://localhost:5000/api/vr/state",
  "http://100.83.59.61:5173/api/vr/state",
];
// Un routeur injoignable (PC éteint) ne doit pas bloquer l'envoi suivant.
const SEND_TIMEOUT_MS = 5000;

// resolveStateEndpoints lit l'URL imposée dans la popup (clé stateEndpoint).
// Vide ou illisible : les routeurs par défaut, en commençant par le dernier
// qui a répondu.
async function resolveStateEndpoints() {
  try {
    const { stateEndpoint, lastDelivery } = await chrome.storage.local.get(["stateEndpoint", "lastDelivery"]);
    const forced = typeof stateEndpoint === "string" ? stateEndpoint.trim() : "";
    if (forced) return [forced];
    const last = lastDelivery?.ok ? lastDelivery.endpoint : "";
    return DEFAULT_STATE_ENDPOINTS.includes(last)
      ? [last, ...DEFAULT_STATE_ENDPOINTS.filter((endpoint) => endpoint !== last)]
      : DEFAULT_STATE_ENDPOINTS;
  } catch {
    return DEFAULT_STATE_ENDPOINTS;
  }
}

async function post(endpoint, body) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), SEND_TIMEOUT_MS);
  try {
    const response = await fetch(endpoint, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body,
      signal: controller.signal,
    });
    return response.ok;
  } catch {
    return false;
  } finally {
    clearTimeout(timer);
  }
}

// sendState envoie l'état au premier routeur qui répond, et note lequel pour
// la popup. Aucun ne répond : la popup continue de fonctionner.
async function sendState(state) {
  const body = JSON.stringify(state);
  for (const endpoint of await resolveStateEndpoints()) {
    if (await post(endpoint, body)) {
      void chrome.storage.local.set({ lastDelivery: { ok: true, endpoint, at: Date.now() } });
      return;
    }
  }
  void chrome.storage.local.set({ lastDelivery: { ok: false, endpoint: "", at: Date.now() } });
}

chrome.runtime.onMessage.addListener((message, sender) => {
  if (!message || message.type !== "vr-state" || !message.state) return;

  const latestState = {
    ...message.state,
    receivedAt: Date.now(),
    frameId: sender.frameId,
    pageUrl: sender.url || "",
  };
  void chrome.storage.local.set({ latestState });
  void sendState(latestState);
});

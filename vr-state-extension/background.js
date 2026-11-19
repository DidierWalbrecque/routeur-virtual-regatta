const DEFAULT_STATE_ENDPOINT = "http://localhost:5000/api/vr/state";

// resolveStateEndpoint lit l'URL configurée dans la popup (clé stateEndpoint).
// Vide ou illisible, on retombe sur le défaut Flask :5000.
async function resolveStateEndpoint() {
  try {
    const { stateEndpoint } = await chrome.storage.local.get("stateEndpoint");
    const value = typeof stateEndpoint === "string" ? stateEndpoint.trim() : "";
    return value || DEFAULT_STATE_ENDPOINT;
  } catch {
    return DEFAULT_STATE_ENDPOINT;
  }
}

async function sendState(state) {
  const endpoint = await resolveStateEndpoint();
  try {
    await fetch(endpoint, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(state),
    });
  } catch {
    // Le routeur (Flask ou Go) peut être arrêté : la popup continue de fonctionner.
  }
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

const DEFAULT_STATE_ENDPOINT = "http://localhost:5000/api/vr/state";

let latestState;

const byId = (id) => document.getElementById(id);

function formatCoordinate(value, positiveHemisphere, negativeHemisphere, degreeWidth) {
  void degreeWidth;
  const numericValue = Number(value);
  if (!Number.isFinite(numericValue)) return "—";

  const absolute = Math.abs(numericValue);
  let degrees = Math.floor(absolute);
  const minutesValue = (absolute - degrees) * 60;
  let minutes = Math.floor(minutesValue);
  let seconds = Math.round((minutesValue - minutes) * 60 * 100) / 100;

  if (seconds >= 60) {
    seconds -= 60;
    minutes += 1;
  }
  if (minutes >= 60) {
    minutes -= 60;
    degrees += 1;
  }

  const hemisphere = numericValue < 0 ? negativeHemisphere : positiveHemisphere;
  const secondsText = seconds.toFixed(2).padStart(5, "0");
  return `${degrees}°${String(minutes).padStart(2, "0")}'${secondsText}"${hemisphere}`;
}

function formatPosition(position) {
  if (!position) return "—";
  const latitude = formatCoordinate(position.latitude, "N", "S", 2);
  const longitude = formatCoordinate(position.longitude, "E", "W", 3);
  if (latitude === "—" || longitude === "—") return "—";
  return `${latitude} - ${longitude}`;
}

function formatDate(value) {
  if (!value) return "—";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? String(value) : date.toLocaleString();
}

function render(state) {
  latestState = state;
  byId("status").textContent = state ? "Dernière donnée reçue." : "Aucune donnée reçue.";
  byId("state").hidden = !state;
  byId("copy").disabled = !state;
  if (!state) return;

  byId("display-name").textContent = state.displayName || "—";
  byId("race").textContent = state.raceId ? `${state.raceId}.${state.legNum}` : "—";
  byId("position").textContent = formatPosition(state.position);
  byId("stamina").textContent = state.stamina == null ? "—" : `${state.stamina.toFixed(2)} %`;
  byId("options").textContent = state.fullOptions ? "Full Pack" : (state.options?.join(", ") || "Aucune");
  byId("sails").textContent = state.sails?.map((sail) => sail.name).join(", ") || "Aucune";
  byId("navigation").textContent = [state.heading, state.speed].every((value) => value != null)
    ? `${state.heading.toFixed(1)}° / ${state.speed.toFixed(2)} nd`
    : "—";
  byId("sail").textContent = state.currentSail?.name || "—";
  byId("checkpoints").textContent = `${state.checkpoints?.length || 0} porte(s)`;
  byId("updated-at").textContent = formatDate(state.updatedAt);
}

async function refresh() {
  const result = await chrome.storage.local.get("latestState");
  render(result.latestState || null);
}

// --- Configuration de l'endpoint du routeur ---

function setEndpointStatus(message, kind) {
  const node = byId("endpoint-status");
  node.textContent = message;
  node.className = kind ? kind : "muted";
}

// normalizeEndpoint valide l'URL saisie : seuls http(s) sont acceptés. Vide
// signifie « revenir au défaut ». Retourne null si l'URL est invalide.
function normalizeEndpoint(raw) {
  const value = raw.trim();
  if (value === "") return "";
  let parsed;
  try {
    parsed = new URL(value);
  } catch {
    return null;
  }
  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") return null;
  return parsed.toString();
}

async function loadEndpoint() {
  try {
    const { stateEndpoint } = await chrome.storage.local.get("stateEndpoint");
    const value = typeof stateEndpoint === "string" ? stateEndpoint.trim() : "";
    byId("endpoint").value = value;
    setEndpointStatus(`Actif : ${value || DEFAULT_STATE_ENDPOINT}`, "muted");
  } catch {
    setEndpointStatus("Lecture de la configuration impossible.", "error");
  }
}

async function saveEndpoint(value) {
  const normalized = normalizeEndpoint(value);
  if (normalized === null) {
    setEndpointStatus("URL invalide (http:// ou https:// attendu).", "error");
    return;
  }
  try {
    if (normalized === "") {
      await chrome.storage.local.remove("stateEndpoint");
      byId("endpoint").value = "";
      setEndpointStatus(`Défaut rétabli : ${DEFAULT_STATE_ENDPOINT}`, "ok");
      return;
    }
    await chrome.storage.local.set({ stateEndpoint: normalized });
    byId("endpoint").value = normalized;
    setEndpointStatus(`Enregistré : ${normalized}`, "ok");
  } catch {
    setEndpointStatus("Enregistrement impossible.", "error");
  }
}

byId("copy").addEventListener("click", async () => {
  if (!latestState) return;
  await navigator.clipboard.writeText(JSON.stringify(latestState, null, 2));
  byId("status").textContent = "JSON copié dans le presse-papiers.";
});

byId("save-endpoint").addEventListener("click", () => {
  void saveEndpoint(byId("endpoint").value);
});

byId("reset-endpoint").addEventListener("click", () => {
  void saveEndpoint("");
});

byId("endpoint").addEventListener("keydown", (event) => {
  if (event.key === "Enter") {
    event.preventDefault();
    void saveEndpoint(byId("endpoint").value);
  }
});

void refresh();
void loadEndpoint();

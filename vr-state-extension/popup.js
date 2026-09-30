let latestState;

const byId = (id) => document.getElementById(id);

function formatCoordinate(value, positiveHemisphere, negativeHemisphere, degreeWidth) {
  const numericValue = Number(value);
  if (!Number.isFinite(numericValue)) return "—";

  let totalSeconds = Math.round(Math.abs(numericValue) * 3600);
  let degrees = Math.floor(totalSeconds / 3600);
  totalSeconds %= 3600;
  let minutes = Math.floor(totalSeconds / 60);
  let seconds = totalSeconds % 60;

  if (seconds === 60) {
    seconds = 0;
    minutes += 1;
  }
  if (minutes === 60) {
    minutes = 0;
    degrees += 1;
  }

  const hemisphere = numericValue < 0 ? negativeHemisphere : positiveHemisphere;
  return `${String(degrees).padStart(degreeWidth, "0")}°${hemisphere} ${String(minutes).padStart(2, "0")}'${String(seconds).padStart(2, "0")}"`;
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

byId("copy").addEventListener("click", async () => {
  if (!latestState) return;
  await navigator.clipboard.writeText(JSON.stringify(latestState, null, 2));
  byId("status").textContent = "JSON copié dans le presse-papiers.";
});

void refresh();

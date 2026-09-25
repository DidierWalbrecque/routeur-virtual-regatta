function elapsedMinutes(startedAt, instant) {
  const start = Date.parse(startedAt || "");
  const end = Date.parse(instant || "");
  if (!Number.isFinite(start) || !Number.isFinite(end)) return null;
  return Math.round((end - start) / 60000);
}

export function buildRouteDetailsModel(result) {
  const route = Array.isArray(result?.route) ? result.route : [];
  const startedAt = route[0]?.instant || "";
  const steps = route.map((point, index) => ({
    index,
    label: index === 0 ? "Départ" : point.terminal ? "Arrivée" : `Point ${index}`,
    instant: point.instant || "",
    elapsedMinutes: elapsedMinutes(startedAt, point.instant),
    latitude: point.position?.latitude ?? null,
    longitude: point.position?.longitude ?? null,
    heading: index === 0 || point.terminal ? null : point.heading,
    sail: index === 0 || point.terminal ? "—" : point.sail || "—",
    terminal: point.terminal === true,
  }));
  const lastStep = steps.at(-1);

  return {
    destinationReached: result?.destinationReached === true,
    statusText: result?.destinationReached === true
      ? "Destination atteinte."
      : "Destination non atteinte dans l’horizon choisi — meilleure route disponible affichée.",
    pointCount: route.length,
    durationMinutes: lastStep?.elapsedMinutes ?? null,
    steps,
  };
}

function formatInstant(instant) {
  if (!instant) return "—";
  const date = new Date(instant);
  if (Number.isNaN(date.getTime())) return instant;
  return date.toLocaleString("fr-FR", { dateStyle: "short", timeStyle: "short" });
}

function formatCoordinate(value) {
  return Number.isFinite(Number(value)) ? Number(value).toFixed(6) : "—";
}

function appendCell(row, value) {
  const cell = document.createElement("td");
  cell.textContent = String(value ?? "—");
  row.appendChild(cell);
}

export function renderRouteDetails(container, result) {
  const model = buildRouteDetailsModel(result);
  container.replaceChildren();

  const status = document.createElement("p");
  status.className = model.destinationReached ? "route-details-status reached" : "route-details-status partial";
  status.textContent = model.statusText;
  container.appendChild(status);

  const summary = document.createElement("p");
  summary.className = "route-details-summary";
  const duration = model.durationMinutes === null ? "—" : `${model.durationMinutes} min`;
  summary.textContent = `${model.pointCount} point(s) · durée calculée : ${duration}`;
  container.appendChild(summary);

  const tableWrapper = document.createElement("div");
  tableWrapper.className = "route-details-table-wrapper";
  const table = document.createElement("table");
  table.className = "route-details-table";
  const headers = ["Étape", "Heure", "Temps écoulé", "Latitude", "Longitude", "Cap", "Voile"];
  const head = document.createElement("thead");
  const headerRow = document.createElement("tr");
  headers.forEach((header) => appendCell(headerRow, header));
  head.appendChild(headerRow);
  table.appendChild(head);

  const body = document.createElement("tbody");
  model.steps.forEach((step) => {
    const row = document.createElement("tr");
    appendCell(row, step.label);
    appendCell(row, formatInstant(step.instant));
    appendCell(row, step.elapsedMinutes === null ? "—" : `${step.elapsedMinutes} min`);
    appendCell(row, formatCoordinate(step.latitude));
    appendCell(row, formatCoordinate(step.longitude));
    appendCell(row, step.heading === null || !Number.isFinite(Number(step.heading)) ? "—" : `${Number(step.heading).toFixed(0)}°`);
    appendCell(row, step.sail);
    body.appendChild(row);
  });
  table.appendChild(body);
  tableWrapper.appendChild(table);
  container.appendChild(tableWrapper);
}

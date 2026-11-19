import type { ReportRow } from "./route_details.ts";
import { formatElapsed, formatInstant, formatPosition, maneuverLabel } from "./route_details.ts";
import type { RouteResult } from "./types.ts";

function escapeXml(value: string): string {
  return value.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}

/** Route calculée au format GPX 1.1 : une trace, et un point de passage par manœuvre. */
export function routeToGpx(result: RouteResult, name = "Route calculée"): string {
  const points = (result.route || []).filter(
    (point) => Number.isFinite(point.position?.latitude) && Number.isFinite(point.position?.longitude),
  );
  const coordinates = (point: (typeof points)[number]) =>
    `lat="${point.position.latitude.toFixed(6)}" lon="${point.position.longitude.toFixed(6)}"`;
  const time = (instant: string) => {
    const date = new Date(instant);
    return Number.isNaN(date.getTime()) ? "" : `<time>${date.toISOString()}</time>`;
  };
  // Le moteur note la manœuvre sur le point où le pas se termine ; elle a lieu
  // au point de départ de ce pas, comme dans le tableau des détails.
  const waypoints = points.flatMap((point, index) => {
    if (!point.maneuver) return [];
    const at = points[Math.max(0, index - 1)];
    return [`  <wpt ${coordinates(at)}>${time(at.instant)}<name>${escapeXml(maneuverLabel(point.maneuver))}</name></wpt>`];
  });
  const track = points.map((point) => {
    const description = [
      Number.isFinite(point.heading) ? `cap ${Math.round(Number(point.heading))}°` : "",
      point.sail || "",
    ]
      .filter(Boolean)
      .join(" · ");
    return `      <trkpt ${coordinates(point)}>${time(point.instant)}${description ? `<desc>${escapeXml(description)}</desc>` : ""}</trkpt>`;
  });
  return [
    `<?xml version="1.0" encoding="UTF-8"?>`,
    `<gpx version="1.1" creator="Routeur Virtual Regatta" xmlns="http://www.topografix.com/GPX/1/1">`,
    ...waypoints,
    `  <trk>`,
    `    <name>${escapeXml(name)}</name>`,
    `    <trkseg>`,
    ...track,
    `    </trkseg>`,
    `  </trk>`,
    `</gpx>`,
    "",
  ].join("\n");
}

function decimal(value: number | null, digits: number): string {
  return value === null || !Number.isFinite(value) ? "" : value.toFixed(digits).replace(".", ",");
}

/** Tableau des détails en texte séparé par des tabulations, à coller dans un tableur. */
export function routeRowsToTsv(rows: ReportRow[]): string {
  const header = ["Date", "Temps écoulé", "Cap", "TWA", "Bord", "Position", "TWD", "TWS", "Vitesse", "Foils", "Coeff", "Voile", "Stamina", "Manœuvre"];
  const lines = rows.map((row) =>
    [
      formatInstant(row.instant),
      formatElapsed(row.elapsedMinutes),
      decimal(row.heading, 0),
      decimal(row.windAngle, 1),
      row.tack === "—" ? "" : row.tack,
      formatPosition(row.latitude, row.longitude),
      decimal(row.windDirection, 0),
      decimal(row.windKnots, 1),
      decimal(row.speedKnots, 1),
      decimal(row.foilPercent, 0),
      decimal(row.speedCoefficient, 2),
      row.sail === "—" ? "" : row.sail,
      decimal(row.stamina, 1),
      maneuverLabel(row.maneuver),
    ].join("\t"),
  );
  return [header.join("\t"), ...lines].join("\n");
}

/** Nom de fichier daté pour l'export : « route-2026-11-08-1430.gpx ». */
export function exportFileName(instant: string | undefined, extension: string): string {
  const date = new Date(instant || Date.now());
  const safe = Number.isNaN(date.getTime()) ? new Date() : date;
  const pad = (value: number) => String(value).padStart(2, "0");
  return `route-${safe.getFullYear()}-${pad(safe.getMonth() + 1)}-${pad(safe.getDate())}-${pad(safe.getHours())}${pad(safe.getMinutes())}.${extension}`;
}

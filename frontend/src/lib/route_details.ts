import type { RoutePoint, RouteResult } from "./types.ts";

export type ReportRow = {
  index: number;
  instant: string;
  elapsedMinutes: number | null;
  latitude: number | null;
  longitude: number | null;
  heading: number | null;
  windAngle: number | null;
  tack: string;
  windDirection: number | null;
  windKnots: number | null;
  speedKnots: number | null;
  foilPercent: number | null;
  speedCoefficient: number | null;
  sail: string;
  maneuver: string;
  stamina: number | null;
  terminal: boolean;
};

export type RouteStep = {
  index: number;
  label: string;
  instant: string;
  elapsedMinutes: number | null;
  latitude: number | null;
  longitude: number | null;
  heading: number | null | undefined;
  stamina: number | null;
  sail: string;
  terminal: boolean;
};

export type RouteDetailsModel = {
  destinationReached: boolean;
  statusText: string;
  pointCount: number;
  durationMinutes: number | null;
  maneuverCount: number;
  steps: RouteStep[];
  rows: ReportRow[];
};

function elapsedMinutes(startedAt: string, instant: string | undefined): number | null {
  const start = Date.parse(startedAt || "");
  const end = Date.parse(instant || "");
  if (!Number.isFinite(start) || !Number.isFinite(end)) return null;
  return Math.round((end - start) / 60000);
}

function optionalNumber(value: unknown): number | null {
  if (value === null || value === undefined || value === "") return null;
  const number = Number(value);
  return Number.isFinite(number) ? number : null;
}

function tackLetter(tack: string | undefined): string {
  if (tack === "starboard") return "T";
  if (tack === "port") return "B";
  return "—";
}

// Une ligne du rapport décrit le départ d'un pas : la position du point, puis
// le cap, la voile et le vent du pas qui part de ce point. Ces informations
// sont portées par le point suivant de la route (celui où le pas se termine).
function reportRow(route: RoutePoint[], index: number, startedAt: string): ReportRow {
  const point = route[index];
  const next = route[index + 1];
  const departing =
    next && next.terminal !== true && Number.isFinite(Number(next.heading)) ? next : null;
  return {
    index,
    instant: point.instant || "",
    elapsedMinutes: elapsedMinutes(startedAt, point.instant),
    latitude: point.position?.latitude ?? null,
    longitude: point.position?.longitude ?? null,
    heading: departing ? Number(departing.heading) : null,
    windAngle: departing ? optionalNumber(departing.twa) : null,
    tack: departing ? tackLetter(departing.tack) : "—",
    windDirection: departing ? optionalNumber(departing.twd) : null,
    windKnots: departing ? optionalNumber(departing.tws) : null,
    speedKnots: departing ? optionalNumber(departing.speedKnots) : null,
    foilPercent: departing ? optionalNumber(departing.foilPercent) : null,
    speedCoefficient: departing ? optionalNumber(departing.speedCoefficient) : null,
    sail: departing ? departing.sail || "—" : "—",
    maneuver: departing ? departing.maneuver || "" : "",
    stamina: optionalNumber(point.stamina),
    terminal: point.terminal === true,
  };
}

export function buildRouteDetailsModel(result: Partial<RouteResult> | null | undefined): RouteDetailsModel {
  const route: RoutePoint[] = Array.isArray(result?.route) ? result.route : [];
  const startedAt = route[0]?.instant || "";
  const steps: RouteStep[] = route.map((point, index) => ({
    index,
    label: index === 0 ? "Départ" : point.terminal ? "Arrivée" : `Point ${index}`,
    instant: point.instant || "",
    elapsedMinutes: elapsedMinutes(startedAt, point.instant),
    latitude: point.position?.latitude ?? null,
    longitude: point.position?.longitude ?? null,
    heading: index === 0 || point.terminal ? null : point.heading,
    stamina: Number.isFinite(Number(point.stamina)) ? Number(point.stamina) : null,
    sail: index === 0 || point.terminal ? "—" : point.sail || "—",
    terminal: point.terminal === true,
  }));
  const rows = route.map((_, index) => reportRow(route, index, startedAt));
  const lastStep = steps.at(-1);

  return {
    destinationReached: result?.destinationReached === true,
    statusText:
      result?.destinationReached === true
        ? "Destination atteinte."
        : "Destination non atteinte dans l’horizon choisi — meilleure route disponible affichée.",
    pointCount: route.length,
    durationMinutes: lastStep?.elapsedMinutes ?? null,
    maneuverCount: rows.filter((row) => row.maneuver).length,
    steps,
    rows,
  };
}

export function formatDms(value: number, positiveLetter: string, negativeLetter: string): string {
  const number = Number(value);
  if (!Number.isFinite(number)) return "—";
  const letter = number < 0 ? negativeLetter : positiveLetter;
  const absolute = Math.abs(number);
  let degrees = Math.floor(absolute);
  let minutes = Math.floor((absolute - degrees) * 60);
  let seconds = ((absolute - degrees) * 60 - minutes) * 60;
  // Évite d'afficher 60.00" après arrondi.
  if (Number(seconds.toFixed(2)) >= 60) {
    seconds = 0;
    minutes += 1;
  }
  if (minutes >= 60) {
    minutes = 0;
    degrees += 1;
  }
  return `${degrees}°${String(minutes).padStart(2, "0")}'${seconds.toFixed(2).padStart(5, "0")}"${letter}`;
}

export function formatPosition(latitude: number | null, longitude: number | null): string {
  if (!Number.isFinite(Number(latitude)) || !Number.isFinite(Number(longitude))) return "—";
  return `${formatDms(Number(latitude), "N", "S")} ${formatDms(Number(longitude), "E", "W")}`;
}

export function formatElapsed(minutes: number | null): string {
  if (minutes === null || !Number.isFinite(minutes)) return "—";
  const hours = Math.floor(minutes / 60);
  return `+${hours}:${String(minutes % 60).padStart(2, "0")}`;
}

export function formatInstant(instant: string): string {
  if (!instant) return "—";
  const date = new Date(instant);
  if (Number.isNaN(date.getTime())) return instant;
  const day = String(date.getDate()).padStart(2, "0");
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const hours = String(date.getHours()).padStart(2, "0");
  const minutes = String(date.getMinutes()).padStart(2, "0");
  return `${day}/${month} ${hours}:${minutes}`;
}

/** Nombre à la française : virgule décimale, « — » quand la valeur manque. */
export function formatNumber(value: number | null, digits: number): string {
  return value !== null && Number.isFinite(value) ? value.toFixed(digits).replace(".", ",") : "—";
}

// Les angles entiers s'affichent sans décimale, comme dans le jeu, les autres avec une seule.
export function formatAngle(value: number | null): string {
  if (value === null || !Number.isFinite(value)) return "—";
  return Math.abs(value - Math.round(value)) < 0.05 ? String(Math.round(value)) : formatNumber(value, 1);
}

/** Nom lisible d'une manœuvre envoyée par le moteur. */
export function maneuverLabel(maneuver: string): string {
  if (maneuver === "tack") return "Virement";
  if (maneuver === "gybe") return "Empannage";
  if (maneuver === "sail") return "Changement de voile";
  return maneuver;
}

/** Colonnes dont la valeur ne varie pas sur toute la route (masquées dans le tableau). */
export function constantColumns(rows: ReportRow[]): Set<"foils" | "coeff"> {
  const constant = new Set<"foils" | "coeff">();
  const sameEverywhere = (pick: (row: ReportRow) => number | null) =>
    new Set(rows.map(pick).filter((value) => value !== null).map((value) => Math.round(Number(value) * 100))).size <= 1;
  if (sameEverywhere((row) => row.foilPercent)) constant.add("foils");
  if (sameEverywhere((row) => row.speedCoefficient)) constant.add("coeff");
  return constant;
}

export const REPORT_HEADERS: { label: string; title?: string }[] = [
  { label: "Date" },
  { label: "Temps écoulé" },
  { label: "Cap", title: "Cap à programmer (degrés)" },
  { label: "TWA", title: "Angle du vent réel (degrés)" },
  { label: "Bord", title: "T = tribord, B = bâbord" },
  { label: "Position" },
  { label: "TWD", title: "Direction du vent (degrés)" },
  { label: "TWS", title: "Force du vent (nœuds)" },
  { label: "Vitesse", title: "Vitesse du bateau (nœuds)" },
  { label: "Foils", title: "Activation des foils (%)" },
  { label: "Coeff", title: "Coefficient de vitesse appliqué après une manœuvre" },
  { label: "Voile" },
  { label: "Stamina" },
];

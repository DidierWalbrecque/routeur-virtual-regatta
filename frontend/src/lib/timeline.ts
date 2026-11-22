import { BEAUFORT, beaufortForce } from "./beaufort.ts";
import type { Isochrone, Point, RoutePoint } from "./types.ts";

/**
 * Frise de temps du cockpit : un seul curseur d'heure qui fait avancer le
 * bateau sur la route, l'isochrone affichée et le fond de vent.
 */

const HOUR_MS = 3_600_000;

/** Sans route, la frise parcourt les prévisions : de l'heure en cours à +240 h. */
export const FREE_TIMELINE_HOURS = 240;

/** Bornes de la frise, en millisecondes. */
export type TimelineRange = { start: number; end: number };

/** Instant d'un point de route en millisecondes, ou NaN. */
function instantOf(point: RoutePoint | undefined): number {
  return point ? Date.parse(point.instant) : Number.NaN;
}

/** Du premier au dernier point de la route, ou null sans route exploitable. */
export function routeRange(route: RoutePoint[]): TimelineRange | null {
  const start = instantOf(route[0]);
  const end = instantOf(route.at(-1));
  if (!Number.isFinite(start) || !Number.isFinite(end) || end <= start) return null;
  return { start, end };
}

/** Début de l'heure contenant `time`. */
export function floorHour(time: number): number {
  return Math.floor(time / HOUR_MS) * HOUR_MS;
}

/** Frise sans route : de l'heure en cours aux 240 h de prévision. */
export function freeRange(now: number): TimelineRange {
  const start = floorHour(now);
  return { start, end: start + FREE_TIMELINE_HOURS * HOUR_MS };
}

export function clampTime(range: TimelineRange, time: number): number {
  return Math.min(range.end, Math.max(range.start, time));
}

/** Position de `time` dans la frise, de 0 à 1. */
export function fractionOf(range: TimelineRange, time: number): number {
  const span = range.end - range.start;
  return span > 0 ? (clampTime(range, time) - range.start) / span : 0;
}

/** Longitude entre a et b, par le plus court chemin (passage de l'antiméridien compris). */
export function interpolateLongitude(a: number, b: number, fraction: number): number {
  let delta = b - a;
  if (delta > 180) delta -= 360;
  if (delta < -180) delta += 360;
  const value = a + delta * fraction;
  return ((((value + 180) % 360) + 360) % 360) - 180;
}

/** Indice du dernier point de route atteint à `time` (0 avant le départ). */
export function stepIndexAt(route: RoutePoint[], time: number): number {
  let low = 0;
  let high = route.length - 1;
  if (high < 0) return -1;
  while (low < high) {
    const middle = Math.ceil((low + high) / 2);
    if (instantOf(route[middle]) <= time) low = middle;
    else high = middle - 1;
  }
  return low;
}

/** Position du bateau à `time`, interpolée entre deux points de route. */
export function positionAt(route: RoutePoint[], time: number): Point | null {
  if (route.length === 0) return null;
  const index = stepIndexAt(route, time);
  const from = route[index];
  const to = route[index + 1];
  if (!to) return from.position;
  const start = instantOf(from);
  const span = instantOf(to) - start;
  const fraction = span > 0 ? Math.min(1, Math.max(0, (time - start) / span)) : 0;
  return {
    latitude: from.position.latitude + (to.position.latitude - from.position.latitude) * fraction,
    longitude: interpolateLongitude(from.position.longitude, to.position.longitude, fraction),
  };
}

/**
 * Pas de route parcouru à partir du point `index` : le cap, la voile, le vent
 * et la manœuvre d'un pas sont portés par le point où il se termine (comme
 * dans le tableau de la route). Sur le dernier point, le dernier pas connu.
 */
export function legAt(route: RoutePoint[], index: number): RoutePoint | null {
  return route[index + 1] ?? route[index] ?? null;
}

/** Repère d'une manœuvre sur la frise, au début du pas qui la fait. */
export type TimelineMark = { index: number; fraction: number; maneuver: string; time: number };

export function maneuverMarks(route: RoutePoint[], range: TimelineRange): TimelineMark[] {
  const marks: TimelineMark[] = [];
  for (let index = 0; index < route.length - 1; index += 1) {
    const maneuver = route[index + 1].maneuver;
    const time = instantOf(route[index]);
    if (!maneuver || !Number.isFinite(time)) continue;
    marks.push({ index, fraction: fractionOf(range, time), maneuver, time });
  }
  return marks;
}

/** Tronçon de la frise où le vent garde la même force Beaufort. */
export type WindBand = { from: number; to: number; force: number; color: string };

/**
 * Force du vent le long de la route : un tronçon par suite de points de même
 * force, en fractions de la frise. Les points sans vent connu sont ignorés.
 */
export function windBands(route: RoutePoint[], range: TimelineRange): WindBand[] {
  const bands: WindBand[] = [];
  for (let index = 0; index < route.length - 1; index += 1) {
    // Le vent d'un pas est porté par son point d'arrivée.
    const tws = route[index + 1].tws;
    if (tws === undefined || !Number.isFinite(tws)) continue;
    const force = Math.max(0, beaufortForce(tws));
    const from = fractionOf(range, instantOf(route[index]));
    const to = fractionOf(range, instantOf(route[index + 1]));
    const last = bands.at(-1);
    if (last && last.force === force && Math.abs(last.to - from) < 1e-9) {
      last.to = to;
    } else {
      bands.push({ from, to, force, color: BEAUFORT[force].color });
    }
  }
  return bands;
}

/** Indice de la dernière isochrone atteinte à `time`, ou -1 avant la première. */
export function isochroneIndexAt(isochrones: Isochrone[], time: number): number {
  let found = -1;
  for (let index = 0; index < isochrones.length; index += 1) {
    if (Date.parse(isochrones[index].instant) <= time) found = index;
    else break;
  }
  return found;
}

/** Dernier élément dont l'indice de départ est atteint (ordre en cours), ou null. */
export function activeAt<T extends { index: number }>(items: T[], stepIndex: number): T | null {
  let active: T | null = null;
  for (const item of items) {
    if (item.index <= stepIndex) active = item;
    else break;
  }
  return active;
}

/** Repère de jour : chaque minuit local compris dans la frise. */
export type DayTick = { fraction: number; time: number };

export function dayTicks(range: TimelineRange): DayTick[] {
  const ticks: DayTick[] = [];
  const day = new Date(range.start);
  day.setHours(24, 0, 0, 0);
  while (day.getTime() < range.end) {
    ticks.push({ fraction: fractionOf(range, day.getTime()), time: day.getTime() });
    day.setDate(day.getDate() + 1);
  }
  return ticks;
}

/** « +14 h 20 », « +2 j 3 h » depuis le début de la frise. */
export function formatOffset(minutes: number): string {
  const total = Math.max(0, Math.round(minutes));
  const days = Math.floor(total / 1440);
  const hours = Math.floor((total % 1440) / 60);
  const rest = total % 60;
  if (days > 0) return `+${days} j ${hours} h`;
  if (hours > 0) return rest ? `+${hours} h ${String(rest).padStart(2, "0")}` : `+${hours} h`;
  return `+${rest} min`;
}

/** Dégradé CSS de gauche à droite : la couleur Beaufort au milieu de chaque tronçon. */
export function windGradient(bands: WindBand[]): string {
  if (bands.length === 0) return "none";
  if (bands.length === 1) return `linear-gradient(${bands[0].color}, ${bands[0].color})`;
  const stops = bands.map((band) => `${band.color} ${(((band.from + band.to) / 2) * 100).toFixed(2)}%`);
  return `linear-gradient(to right, ${stops.join(", ")})`;
}

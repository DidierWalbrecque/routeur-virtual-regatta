import type { GameOrder, RouteSummary } from "./game_orders.ts";
import type { ReportRow } from "./route_details.ts";

/**
 * Vue « carnet de bord » d'une route calculée : l'arrivée en grand, une phrase
 * de résumé, la courbe de stamina et les ordres groupés par jour.
 */

const WEEKDAY = new Intl.DateTimeFormat("fr-FR", { weekday: "short" });
const LONG_WEEKDAY = new Intl.DateTimeFormat("fr-FR", { weekday: "long" });

function pad(value: number): string {
  return String(value).padStart(2, "0");
}

/** « lun. 23/11 · 13:04 » en heure locale. */
export function formatLongInstant(instant: string): string {
  const date = new Date(instant);
  if (!instant || Number.isNaN(date.getTime())) return "—";
  return `${WEEKDAY.format(date)} ${pad(date.getDate())}/${pad(date.getMonth() + 1)} · ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

/** « 1 j 3 h », « 5 h 20 », « 40 min ». */
export function formatDuration(minutes: number | null): string {
  if (minutes === null || !Number.isFinite(minutes)) return "—";
  const total = Math.round(minutes);
  if (total < 60) return `${total} min`;
  const days = Math.floor(total / 1440);
  const hours = Math.floor((total % 1440) / 60);
  const rest = total % 60;
  if (days > 0) return hours ? `${days} j ${hours} h` : `${days} j`;
  return rest ? `${hours} h ${pad(rest)}` : `${hours} h`;
}

/** « 1 j de route · 695 NM · 3 manœuvres · GFS 06Z du 22/11 ». */
export function routeHeadline(summary: RouteSummary, runLabel: string): string {
  const distance = summary.distanceNM < 100 ? summary.distanceNM.toFixed(1).replace(".", ",") : String(Math.round(summary.distanceNM));
  const maneuvers = `${summary.maneuverCount} manœuvre${summary.maneuverCount > 1 ? "s" : ""}`;
  return [`${formatDuration(summary.durationMinutes)} de route`, `${distance} NM`, maneuvers, runLabel ? `GFS ${runLabel}` : ""]
    .filter(Boolean)
    .join(" · ");
}

/** Point de la courbe de stamina : position dans le temps (0 à 1) et valeur. */
export type StaminaPoint = { fraction: number; value: number };

function rowTime(row: ReportRow): number {
  return Date.parse(row.instant);
}

/** Bornes de temps de la route (premier et dernier point datés), ou null. */
function timeRange(rows: ReportRow[]): { start: number; span: number } | null {
  const times = rows.map(rowTime).filter(Number.isFinite);
  if (times.length < 2) return null;
  const start = Math.min(...times);
  const span = Math.max(...times) - start;
  return span > 0 ? { start, span } : null;
}

/** Stamina de chaque point de la route, placée dans le temps. */
export function staminaSeries(rows: ReportRow[]): StaminaPoint[] {
  const range = timeRange(rows);
  if (!range) return [];
  return rows
    .filter((row) => row.stamina !== null && Number.isFinite(rowTime(row)))
    .map((row) => ({ fraction: (rowTime(row) - range.start) / range.span, value: row.stamina as number }));
}

/** Manœuvres placées dans le temps (0 à 1), au début du pas qui les fait. */
export function maneuverFractions(rows: ReportRow[]): number[] {
  const range = timeRange(rows);
  if (!range) return [];
  return rows.filter((row) => row.maneuver && Number.isFinite(rowTime(row))).map((row) => (rowTime(row) - range.start) / range.span);
}

/**
 * Tracé SVG de la courbe dans un cadre `width` × `height`. L'axe vertical va
 * d'un peu sous la plus basse valeur à 100 %, pour que les chutes se voient.
 */
export function staminaPath(series: StaminaPoint[], width: number, height: number): { path: string; floor: number } {
  if (series.length === 0) return { path: "", floor: 0 };
  const lowest = Math.min(...series.map((point) => point.value));
  const floor = Math.max(0, Math.floor((lowest - 5) / 10) * 10);
  const span = Math.max(1, 100 - floor);
  const path = series
    .map((point, index) => {
      const x = (point.fraction * width).toFixed(1);
      const y = (height - ((Math.min(100, point.value) - floor) / span) * height).toFixed(1);
      return `${index === 0 ? "M" : "L"}${x} ${y}`;
    })
    .join(" ");
  return { path, floor };
}

/** Ordres d'une même journée (heure locale). */
export type OrderDay = { key: string; label: string; orders: GameOrder[] };

/** Regroupe les ordres par jour : « Dimanche 22/11 », « Lundi 23/11 ». */
export function groupOrdersByDay(orders: GameOrder[]): OrderDay[] {
  const days: OrderDay[] = [];
  for (const order of orders) {
    const date = new Date(order.instant);
    const valid = !Number.isNaN(date.getTime());
    const key = valid ? `${date.getFullYear()}-${date.getMonth()}-${date.getDate()}` : "?";
    let day = days.at(-1);
    if (!day || day.key !== key) {
      const weekday = valid ? LONG_WEEKDAY.format(date) : "";
      const label = valid ? `${weekday.charAt(0).toUpperCase()}${weekday.slice(1)} ${pad(date.getDate())}/${pad(date.getMonth() + 1)}` : "Date inconnue";
      day = { key, label, orders: [] };
      days.push(day);
    }
    day.orders.push(order);
  }
  return days;
}

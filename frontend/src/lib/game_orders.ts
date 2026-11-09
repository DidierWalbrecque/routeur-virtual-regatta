import { approximateDistanceNM } from "./geo.ts";
import { formatInstant, maneuverLabel, type ReportRow } from "./route_details.ts";

/** Réglage à donner dans le jeu : angle au vent (TWA) ou cap. */
export type OrderMode = "twa" | "heading";

/** Un ordre à donner dans le jeu, valable jusqu'à l'ordre suivant. */
export type GameOrder = {
  /** Indice de la ligne de route où l'ordre commence. */
  index: number;
  instant: string;
  elapsedMinutes: number | null;
  latitude: number | null;
  longitude: number | null;
  /** TWA arrondie, en degrés (signe ignoré : le bord est donné par `tack`). */
  twa: number | null;
  /** « T » tribord, « B » bâbord, « — » inconnu. */
  tack: string;
  heading: number | null;
  sail: string;
  /** Manœuvre qui ouvre l'ordre (virement, empannage, changement de voile), ou "". */
  maneuver: string;
  /** Durée pendant laquelle l'ordre reste valable, en minutes. */
  durationMinutes: number | null;
};

/**
 * En deçà de cet écart (en degrés) avec le réglage en cours, le changement de
 * TWA ou de cap ne vaut pas un nouvel ordre : le joueur garde son réglage.
 */
export const ORDER_THRESHOLD_DEGREES = 5;

/** Un ordre de simple dérive plus court que cette durée est fondu dans le précédent. */
export const MIN_DRIFT_ORDER_MINUTES = 30;

function angleGap(a: number, b: number): number {
  const gap = Math.abs(a - b) % 360;
  return gap > 180 ? 360 - gap : gap;
}

/**
 * Condense la route en ordres à donner dans le jeu. Un nouvel ordre commence à
 * chaque manœuvre, à chaque changement de voile ou de bord, et quand le réglage
 * suivi (TWA ou cap) s'écarte d'au moins `threshold` degrés de l'ordre en cours.
 */
export function buildGameOrders(
  rows: ReportRow[],
  mode: OrderMode,
  threshold = ORDER_THRESHOLD_DEGREES,
  minDurationMinutes = MIN_DRIFT_ORDER_MINUTES,
): GameOrder[] {
  const orders: GameOrder[] = [];
  for (const row of rows) {
    if (row.heading === null || row.terminal) continue;
    const twa = row.windAngle === null ? null : Math.round(Math.abs(row.windAngle));
    const heading = Math.round(row.heading) % 360;
    const current = orders.at(-1);
    const value = mode === "twa" ? twa : heading;
    const currentValue = current ? (mode === "twa" ? current.twa : current.heading) : null;
    const drifted = value !== null && currentValue !== null && angleGap(value, currentValue) >= threshold;
    const startsOrder =
      !current || Boolean(row.maneuver) || row.sail !== current.sail || row.tack !== current.tack || drifted;
    if (!startsOrder) continue;
    orders.push({
      index: row.index,
      instant: row.instant,
      elapsedMinutes: row.elapsedMinutes,
      latitude: row.latitude,
      longitude: row.longitude,
      twa,
      tack: row.tack,
      heading,
      sail: row.sail,
      maneuver: row.maneuver,
      durationMinutes: null,
    });
  }
  const end = rows.at(-1)?.elapsedMinutes ?? null;
  setDurations(orders, end);

  // Un ordre de simple dérive (sans manœuvre ni changement de voile ou de bord)
  // qui ne dure pas est du bruit du calcul : la TWA s'écarte puis revient. Il est
  // fondu dans l'ordre précédent, puis on recommence tant que la liste change.
  const isDrift = (order: GameOrder, previous: GameOrder) =>
    !order.maneuver && order.sail === previous.sail && order.tack === previous.tack;
  let changed = true;
  while (changed) {
    changed = false;
    for (let position = 1; position < orders.length; position += 1) {
      const order = orders[position];
      const previous = orders[position - 1];
      if (!isDrift(order, previous)) continue;
      const value = mode === "twa" ? order.twa : order.heading;
      const previousValue = mode === "twa" ? previous.twa : previous.heading;
      const short = order.durationMinutes !== null && order.durationMinutes < minDurationMinutes;
      const close = value !== null && previousValue !== null && angleGap(value, previousValue) < threshold;
      if (short || close) {
        orders.splice(position, 1);
        setDurations(orders, end);
        changed = true;
        break;
      }
    }
  }
  return orders;
}

/** Durée de chaque ordre : jusqu'au suivant, ou jusqu'à la fin de la route. */
function setDurations(orders: GameOrder[], end: number | null) {
  orders.forEach((order, position) => {
    const next = orders[position + 1]?.elapsedMinutes ?? end;
    order.durationMinutes = next !== null && order.elapsedMinutes !== null ? next - order.elapsedMinutes : null;
  });
}

/** Réglage principal d'un ordre, tel qu'on le saisit dans le jeu : « TWA 57° T » ou « Cap 189° ». */
export function orderSetting(order: GameOrder, mode: OrderMode): string {
  if (mode === "twa") {
    if (order.twa === null) return order.heading === null ? "—" : `Cap ${order.heading}°`;
    return `TWA ${order.twa}°${order.tack === "—" ? "" : ` ${order.tack}`}`;
  }
  return order.heading === null ? "—" : `Cap ${order.heading}°`;
}

/** Ordres en texte, une ligne par ordre, à coller dans des notes ou une messagerie. */
export function gameOrdersToText(orders: GameOrder[], mode: OrderMode): string {
  return orders
    .map((order) =>
      [
        formatInstant(order.instant),
        orderSetting(order, mode),
        order.sail === "—" ? "" : order.sail,
        order.maneuver ? maneuverLabel(order.maneuver) : "",
      ]
        .filter(Boolean)
        .join(" · "),
    )
    .join("\n");
}

/** Chiffres clés d'une route calculée. */
export type RouteSummary = {
  /** Heure du dernier point (arrivée, ou fin de l'horizon). */
  endInstant: string;
  durationMinutes: number | null;
  distanceNM: number;
  maneuverCount: number;
  finalStamina: number | null;
};

export function summarizeRoute(rows: ReportRow[], maneuverCount: number, durationMinutes: number | null): RouteSummary {
  let distanceNM = 0;
  for (let index = 1; index < rows.length; index += 1) {
    const from = rows[index - 1];
    const to = rows[index];
    if (from.latitude === null || from.longitude === null || to.latitude === null || to.longitude === null) continue;
    distanceNM += approximateDistanceNM(
      { latitude: from.latitude, longitude: from.longitude },
      { latitude: to.latitude, longitude: to.longitude },
    );
  }
  const finalStamina = [...rows].reverse().find((row) => row.stamina !== null)?.stamina ?? null;
  return {
    endInstant: rows.at(-1)?.instant ?? "",
    durationMinutes,
    distanceNM,
    maneuverCount,
    finalStamina,
  };
}

import { formatDuration } from "./horizon.ts";

/** Résultat d'un départ candidat, tel que transmis par /api/departure-window/stream. */
export type DepartureOutcome = {
  index: number;
  departure: string;
  reached: boolean;
  arrival?: string;
  elapsedHours: number;
  gatesCrossed: number;
  nextTarget: string;
  distanceNM: number;
  position: { latitude: number; longitude: number };
};

/** Événement « start » : les départs qui vont être comparés. */
export type DepartureWindowStart = {
  departures: string[];
  comparisonHours: number;
  totalGates: number;
  coveredUntil: string;
  windowClose?: string;
};

/** Durées comparées proposées, en heures. */
export const COMPARISON_HOURS = [24, 48, 72, 96, 120] as const;
/** Écarts proposés entre deux départs, en heures. */
export const DEPARTURE_STEP_HOURS = [3, 6, 12, 24] as const;

/**
 * Même classement que le serveur : arrivé avant, sinon plus de portes
 * franchies, sinon plus près de la porte suivante ; à égalité, le plus tôt.
 */
export function departureBetter(a: DepartureOutcome, b: DepartureOutcome): boolean {
  if (a.reached !== b.reached) return a.reached;
  if (a.reached && a.elapsedHours !== b.elapsedHours) return a.elapsedHours < b.elapsedHours;
  if (!a.reached) {
    if (a.gatesCrossed !== b.gatesCrossed) return a.gatesCrossed > b.gatesCrossed;
    if (a.distanceNM !== b.distanceNM) return a.distanceNM < b.distanceNM;
  }
  return Date.parse(a.departure) < Date.parse(b.departure);
}

/** Indice du meilleur départ, -1 sans résultat. */
export function bestDepartureIndex(outcomes: DepartureOutcome[]): number {
  let best = -1;
  outcomes.forEach((outcome, index) => {
    if (best < 0 || departureBetter(outcome, outcomes[best])) best = index;
  });
  return best;
}

/**
 * Où en est la route de ce départ au bout de la durée comparée. gateNames
 * donne le nom affiché de chaque porte restante, dans l'ordre de passage : le
 * serveur ne connaît que le nom brut (« Checkpoint 2.8 » pour une porte sans
 * nom).
 */
export function departureSummary(outcome: DepartureOutcome, totalGates: number, gateNames: string[] = []): string {
  if (outcome.reached) return `arrivée en ${formatDuration(outcome.elapsedHours * 60)}`;
  const gates = `${outcome.gatesCrossed}/${totalGates} porte${totalGates > 1 ? "s" : ""}`;
  return `${gates} · ${Math.round(outcome.distanceNM)} NM de ${departureTarget(outcome, totalGates, gateNames, true)}`;
}

/**
 * Cible visée au bout de la durée comparée : « porte 2 (tribord) »,
 * « arrivée », ou le nom brut du serveur faute de mieux. withArticle donne la
 * forme à placer après « de » (« la porte 2… », « l'arrivée »).
 */
export function departureTarget(outcome: DepartureOutcome, totalGates: number, gateNames: string[] = [], withArticle = false): string {
  if (outcome.gatesCrossed >= totalGates) return withArticle ? "l'arrivée" : "arrivée";
  const name = gateNames[outcome.gatesCrossed];
  if (!name) return outcome.nextTarget;
  return `${withArticle ? "la porte" : "porte"} ${outcome.gatesCrossed + 1} (${name})`;
}

/**
 * Écart avec le meilleur départ, en milles, quand les deux visent la même
 * porte ; null sinon (portes différentes ou arrivée).
 */
export function departureGapNM(outcome: DepartureOutcome, best: DepartureOutcome): number | null {
  if (outcome.reached || best.reached || outcome.gatesCrossed !== best.gatesCrossed) return null;
  return Math.max(0, outcome.distanceNM - best.distanceNM);
}

/** Nombre de départs affichés par page du tableau. */
export const DEPARTURES_PER_PAGE = 10;

/** Nombre de pages pour count départs (au moins 1). */
export function pageCount(count: number, perPage = DEPARTURES_PER_PAGE): number {
  return Math.max(1, Math.ceil(count / perPage));
}

/** Page (à partir de 0) qui contient le départ d'indice index. */
export function pageOf(index: number, perPage = DEPARTURES_PER_PAGE): number {
  return index < 0 ? 0 : Math.floor(index / perPage);
}

/** Page ramenée dans les bornes, pour count départs. */
export function clampPage(page: number, count: number, perPage = DEPARTURES_PER_PAGE): number {
  return Math.min(Math.max(0, page), pageCount(count, perPage) - 1);
}

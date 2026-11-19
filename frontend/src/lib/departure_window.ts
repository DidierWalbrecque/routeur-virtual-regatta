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

/**
 * Retard sur le meilleur départ quand il ne se mesure pas en milles : portes
 * de moins franchies, ou arrivée manquée alors que le meilleur arrive. Null
 * quand les deux visent la même cible (le retard est alors departureGapNM).
 */
export function departureLagLabel(outcome: DepartureOutcome, best: DepartureOutcome): string | null {
  if (best.reached && !outcome.reached) return "n'arrive pas";
  const missing = best.gatesCrossed - outcome.gatesCrossed;
  if (outcome.reached || missing <= 0) return null;
  return `${missing} porte${missing > 1 ? "s" : ""} de retard`;
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

/** Barre d'un départ dans le graphique de la fenêtre de départ. */
export type DepartureBar = {
  /** Hauteur relative, de 0 à 1 ; 1 pour le meilleur départ. */
  value: number;
  /**
   * Faux quand le départ ne se compare pas au meilleur en milles : arrivée
   * pour l'un et pas pour l'autre, ou une porte de moins franchie.
   */
  comparable: boolean;
};

/** Hauteur minimale d'une barre comparable : le pire départ reste visible. */
const WORST_BAR = 0.15;
/** Hauteur d'un départ qui ne se compare pas au meilleur. */
const OFF_BAR = 0.06;

/** Part des départs comparables dont l'écart reste lisible sur l'échelle. */
const SCALE_QUANTILE = 0.8;

/**
 * Hauteur de chaque barre du graphique : l'avance prise par rapport aux autres
 * départs qui visent la même cible que le meilleur. Le meilleur monte à 1 ;
 * les autres descendent en proportion de leur retard, en milles (ou en heures
 * quand l'arrivée est atteinte).
 *
 * L'échelle s'arrête au retard des 80 % de départs les moins en retard : un
 * seul départ très en retard (928 NM contre 250 à 450 pour les autres sur la
 * capture du Jules Verne) écraserait sinon toutes les autres barres en haut du
 * graphique. Les départs au-delà restent au plancher, à 0,15 ; le tableau
 * donne leur retard exact.
 */
export function departureBars(outcomes: DepartureOutcome[]): DepartureBar[] {
  const bestIndex = bestDepartureIndex(outcomes);
  if (bestIndex < 0) return [];
  const best = outcomes[bestIndex];
  const lag = (outcome: DepartureOutcome): number | null => {
    if (best.reached) return outcome.reached ? outcome.elapsedHours - best.elapsedHours : null;
    return departureGapNM(outcome, best);
  };
  const lags = outcomes.map(lag);
  const sorted = lags.filter((value): value is number => value !== null).sort((a, b) => a - b);
  let scale = sorted[Math.floor(SCALE_QUANTILE * (sorted.length - 1))] ?? 0;
  if (scale <= 0) scale = sorted[sorted.length - 1] ?? 0;
  return lags.map((value) => {
    if (value === null) return { value: OFF_BAR, comparable: false };
    if (scale <= 0) return { value: 1, comparable: true };
    return { value: Math.max(WORST_BAR, 1 - (1 - WORST_BAR) * (value / scale)), comparable: true };
  });
}

export type HorizonChoice = "arrival" | "4h" | "24h" | "2d" | "3d" | "5d";
export type CalculationMode = "quick" | "precise";

export const HORIZON_OPTIONS: { value: HorizonChoice; label: string }[] = [
  { value: "arrival", label: "Jusqu'à l'arrivée" },
  { value: "4h", label: "4 heures" },
  { value: "24h", label: "24 heures" },
  { value: "2d", label: "2 jours" },
  { value: "3d", label: "3 jours" },
  { value: "5d", label: "5 jours" },
];

const MINUTES_PER_DAY = 24 * 60;
/** Plafond de « jusqu'à l'arrivée » : au-delà, les prévisions ne sont plus fiables. */
export const MAX_HORIZON_MINUTES = 5 * MINUTES_PER_DAY;

/** Horizon maximal par mode : le mode précis s'arrête à 3 jours, le rapide va à 5. */
export const MAX_MINUTES_BY_MODE: Record<CalculationMode, number> = {
  precise: 3 * MINUTES_PER_DAY,
  quick: 5 * MINUTES_PER_DAY,
};

const REQUESTED_MINUTES: Record<Exclude<HorizonChoice, "arrival">, number> = {
  "4h": 4 * 60,
  "24h": MINUTES_PER_DAY,
  "2d": 2 * MINUTES_PER_DAY,
  "3d": 3 * MINUTES_PER_DAY,
  "5d": 5 * MINUTES_PER_DAY,
};

export type HorizonResult =
  | { ok: true; minutes: number; limited: boolean; requestedMinutes: number }
  | { ok: false; error: string };

/** Choix d'horizon proposés pour un mode. */
export function horizonOptionsFor(mode: CalculationMode): { value: HorizonChoice; label: string }[] {
  return HORIZON_OPTIONS.filter(
    (option) => option.value === "arrival" || REQUESTED_MINUTES[option.value] <= MAX_MINUTES_BY_MODE[mode],
  );
}

/** Ramène un horizon trop long pour le mode au plus grand horizon permis. */
export function horizonAllowedIn(choice: HorizonChoice, mode: CalculationMode): HorizonChoice {
  return horizonOptionsFor(mode).some((option) => option.value === choice) ? choice : "3d";
}

export function isHorizonChoice(value: unknown): value is HorizonChoice {
  return HORIZON_OPTIONS.some((option) => option.value === value);
}

/** Au moins 3 jours de prévisions, pour pouvoir changer d'horizon sans retélécharger. */
const MIN_FORECAST_HOURS = 72;
/** Marge pour l'écart entre l'émission du run et le départ. */
const RUN_AGE_MARGIN_HOURS = 12;

/** Échéances de prévision (en heures, de 3 en 3) à télécharger pour l'horizon choisi. */
export function forecastHoursFor(choice: HorizonChoice): number[] {
  const requestedHours = choice === "arrival" ? 0 : REQUESTED_MINUTES[choice] / 60;
  const last = Math.max(MIN_FORECAST_HOURS, Math.ceil((requestedHours + RUN_AGE_MARGIN_HOURS) / 3) * 3);
  return Array.from({ length: last / 3 + 1 }, (_, index) => index * 3);
}

/**
 * Durée du routage en minutes. Elle ne dépasse jamais ce que couvrent les
 * prévisions à partir du départ ; `limited` indique qu'elle a été raccourcie.
 */
export function horizonMinutes(
  choice: HorizonChoice,
  startMs: number,
  coveredUntilMs: number,
  maxMinutes: number = MAX_HORIZON_MINUTES,
): HorizonResult {
  const coverageMinutes = Math.floor((coveredUntilMs - startMs) / 60000);
  if (!Number.isFinite(coverageMinutes) || coverageMinutes < 10) {
    return { ok: false, error: "Les prévisions ne couvrent pas l'heure de départ : actualisez les GRIB." };
  }
  const requestedMinutes = choice === "arrival" ? maxMinutes : REQUESTED_MINUTES[choice];
  const minutes = Math.min(requestedMinutes, coverageMinutes, maxMinutes);
  // « Jusqu'à l'arrivée » prend tout ce que les prévisions permettent : ce n'est
  // pas une limitation en soi.
  const limited = choice !== "arrival" && minutes < requestedMinutes;
  return { ok: true, minutes, limited, requestedMinutes };
}

export function formatDuration(minutes: number): string {
  const days = Math.floor(minutes / MINUTES_PER_DAY);
  const hours = Math.floor((minutes % MINUTES_PER_DAY) / 60);
  if (days > 0) return hours > 0 ? `${days} j ${hours} h` : `${days} j`;
  return `${Math.floor(minutes / 60)} h`;
}

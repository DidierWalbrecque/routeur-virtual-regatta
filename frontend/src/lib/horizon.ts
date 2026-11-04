export type HorizonChoice = "arrival" | "4h" | "24h" | "2d" | "3d" | "5d" | "7d" | "10d" | "16d";
export type CalculationMode = "quick" | "precise";

export const HORIZON_OPTIONS: { value: HorizonChoice; label: string }[] = [
  { value: "arrival", label: "Jusqu'à l'arrivée" },
  { value: "4h", label: "4 heures" },
  { value: "24h", label: "24 heures" },
  { value: "2d", label: "2 jours" },
  { value: "3d", label: "3 jours" },
  { value: "5d", label: "5 jours" },
  { value: "7d", label: "7 jours" },
  { value: "10d", label: "10 jours" },
  { value: "16d", label: "16 jours" },
];

const MINUTES_PER_DAY = 24 * 60;

/** Échéance maximale fournie par le GFS 0.25° de NOMADS (16 jours). */
export const GFS_MAX_FORECAST_HOURS = 384;

/** Échéances GRIB téléchargées, indépendantes de l'horizon de routage : tout
 * le GFS 0.25° en 3-horaire, de f000 à f384 (cadence native). Le routage est
 * ensuite plafonné à ce que ces fichiers couvrent. */
export const GRIB_FORECAST_HOURS: number[] = Array.from(
  { length: GFS_MAX_FORECAST_HOURS / 3 + 1 },
  (_, index) => index * 3,
);

/** Plafond absolu de « jusqu'à l'arrivée » : la limite du GFS (16 jours). Au-delà
 * de ~7 jours la prévision devient peu fiable — l'horizon reste proposé, mais à
 * utiliser en connaissance de cause. */
export const MAX_HORIZON_MINUTES = 10 * MINUTES_PER_DAY;

/** Horizon maximal par mode : le mode précis (plus lourd) s'arrête à 7 jours, le
 * rapide va jusqu'à 16 jours pour couvrir une transat entière. */
export const MAX_MINUTES_BY_MODE: Record<CalculationMode, number> = {
  precise: 4 * MINUTES_PER_DAY,
  quick: 10 * MINUTES_PER_DAY,
};

const REQUESTED_MINUTES: Record<Exclude<HorizonChoice, "arrival">, number> = {
  "4h": 4 * 60,
  "24h": MINUTES_PER_DAY,
  "2d": 2 * MINUTES_PER_DAY,
  "3d": 3 * MINUTES_PER_DAY,
  "5d": 5 * MINUTES_PER_DAY,
  "7d": 7 * MINUTES_PER_DAY,
  "10d": 10 * MINUTES_PER_DAY,
  "16d": 16 * MINUTES_PER_DAY,
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
  return horizonOptionsFor(mode).some((option) => option.value === choice) ? choice : "2d";
}

export function isHorizonChoice(value: unknown): value is HorizonChoice {
  return HORIZON_OPTIONS.some((option) => option.value === value);
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

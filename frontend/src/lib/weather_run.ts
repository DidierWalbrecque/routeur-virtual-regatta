const RUN_PATTERN = /^(\d{4})(\d{2})(\d{2})(\d{2})$/;
const HOUR_PATTERN = /\.f(\d{3})\.grb2$/i;

export type WeatherRunModel = {
  title: string;
  runText: string;
  horizonHours: number;
  ageHours: number;
  elapsedPercent: number;
  coveredUntil: string;
  fileCount: number;
};

function parseRun(run: string) {
  const match = String(run || "").match(RUN_PATTERN);
  if (!match) return null;
  const [, year, month, day, hour] = match;
  const instant = Date.UTC(Number(year), Number(month) - 1, Number(day), Number(hour));
  return Number.isFinite(instant) ? { instant, year, month, day, hour } : null;
}

// Décrit le run GFS utilisé : date, horizon de prévision (plus grande échéance
// fNNN des fichiers) et part de cet horizon déjà écoulée depuis le début du run.
export function buildWeatherRunModel(
  run: string,
  files: string[],
  now: number = Date.now(),
): WeatherRunModel | null {
  const parsed = parseRun(run);
  if (!parsed) return null;
  const hours = (Array.isArray(files) ? files : [])
    .map((file) => String(file).match(HOUR_PATTERN))
    .filter((match): match is RegExpMatchArray => match !== null)
    .map((match) => Number(match[1]));
  const horizonHours = hours.length ? Math.max(...hours) : 0;
  const ageHours = Math.max(0, (now - parsed.instant) / 3600000);
  const elapsedHours = Math.min(ageHours, horizonHours);
  const elapsedPercent = horizonHours > 0 ? Math.round((elapsedHours / horizonHours) * 100) : 0;
  return {
    title: "GFS 0.25",
    runText: `${parsed.day}/${parsed.month}/${parsed.year} ${parsed.hour}Z`,
    horizonHours,
    ageHours: Math.floor(ageHours),
    elapsedPercent,
    coveredUntil: new Date(parsed.instant + horizonHours * 3600000).toISOString(),
    fileCount: hours.length,
  };
}

/** « 2026110718 » → « 18Z du 07/11 », la forme courte utilisée dans l'interface. */
export function shortRunLabel(run: string): string {
  const parsed = parseRun(run);
  return parsed ? `${parsed.hour}Z du ${parsed.day}/${parsed.month}` : run;
}

/**
 * Délais de publication habituels d'un run GFS 0.25 sur NOMADS, depuis l'heure
 * du run : les premières échéances vers +3 h 30, le run complet vers +4 h 45.
 */
export const RUN_FIRST_FILES_HOURS = 3.5;
export const RUN_COMPLETE_HOURS = 4.75;
const RUN_INTERVAL_HOURS = 6;

/** Run suivant le run actif et ses heures de publication attendues (ms). */
export type NextRunEta = { hour: string; label: string; firstAt: number; completeAt: number };

export function nextRunEta(run: string): NextRunEta | null {
  const parsed = parseRun(run);
  if (!parsed) return null;
  const instant = parsed.instant + RUN_INTERVAL_HOURS * 3_600_000;
  const date = new Date(instant);
  const hour = String(date.getUTCHours()).padStart(2, "0");
  const day = String(date.getUTCDate()).padStart(2, "0");
  const month = String(date.getUTCMonth() + 1).padStart(2, "0");
  return {
    hour,
    label: `${hour}Z du ${day}/${month}`,
    firstAt: instant + RUN_FIRST_FILES_HOURS * 3_600_000,
    completeAt: instant + RUN_COMPLETE_HOURS * 3_600_000,
  };
}

/** « 16 h 30 » en heure locale. */
export function formatHourMinute(time: number): string {
  const date = new Date(time);
  return `${date.getHours()} h ${String(date.getMinutes()).padStart(2, "0")}`;
}

/** « 15 h 30 UTC ». */
export function formatUtcHourMinute(time: number): string {
  const date = new Date(time);
  return `${date.getUTCHours()} h ${String(date.getUTCMinutes()).padStart(2, "0")} UTC`;
}

/**
 * Texte court de la puce du prochain run : « 12Z vers 15 h 30 UTC » avant la
 * publication (heure des premières échéances), « 12Z imminent » une fois l'heure passée, « 12Z en retard »
 * quand il aurait dû être complet depuis plus d'une heure.
 */
export function nextRunText(eta: NextRunEta, now: number): string {
  if (now < eta.firstAt) return `${eta.hour}Z vers ${formatUtcHourMinute(eta.firstAt)}`;
  if (now < eta.completeAt + 3_600_000) return `${eta.hour}Z imminent`;
  return `${eta.hour}Z en retard`;
}

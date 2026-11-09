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

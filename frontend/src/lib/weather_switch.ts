/** Run GFS récent encore partiel, complété par le run précédent (renvoyé par le serveur). */
export type WeatherSwitch = {
  latestRun: string;
  fallbackRun: string;
  downloaded: number;
  expected: number;
  switchAt: string;
  affectsRoute: boolean;
};

/** « 2026110718 » → « 18Z du 07/11 ». */
export function formatRunLabel(run: string): string {
  if (!/^\d{10}$/.test(run)) return run;
  return `${run.slice(8, 10)}Z du ${run.slice(6, 8)}/${run.slice(4, 6)}`;
}

/** Date UTC lisible, « 08/11 06:00 UTC ». */
function formatUtc(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  const pad = (value: number) => String(value).padStart(2, "0");
  return `${pad(date.getUTCDate())}/${pad(date.getUTCMonth() + 1)} ${pad(date.getUTCHours())}:${pad(date.getUTCMinutes())} UTC`;
}

/** Phrase expliquant quels vents utilise le calcul. */
export function weatherSwitchMessage(weather: WeatherSwitch): string {
  const latest = `run ${formatRunLabel(weather.latestRun)} (${weather.downloaded}/${weather.expected} échéances téléchargées)`;
  if (!weather.affectsRoute) return `Vent du ${latest}, qui couvre toute la route.`;
  return `Vent du ${latest} jusqu'au ${formatUtc(weather.switchAt)}, puis du run ${formatRunLabel(weather.fallbackRun)}, plus ancien, pour la suite de la route.`;
}

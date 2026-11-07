/** Avancement d'un calcul de route : fronts d'isochrones terminés sur le total prévu. */
export type CalculationProgress = { completed: number; total: number };

/** Part du calcul déjà faite, entre 0 et 1 (0 tant que le total est inconnu). */
export function progressFraction(progress: CalculationProgress | null): number {
  if (!progress || !(progress.total > 0)) return 0;
  return Math.min(1, Math.max(0, progress.completed / progress.total));
}

/** Durée au format m:ss, ou h:mm:ss au-delà d'une heure. */
export function formatClock(milliseconds: number): string {
  const seconds = Math.max(0, Math.floor(milliseconds / 1000));
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const rest = String(seconds % 60).padStart(2, "0");
  return hours > 0 ? `${hours}:${String(minutes).padStart(2, "0")}:${rest}` : `${minutes}:${rest}`;
}

/**
 * Temps restant estimé au rythme moyen des fronts déjà calculés. Rien n'est
 * proposé avant quelques fronts et quelques secondes : l'estimation serait trop
 * instable.
 */
export function remainingEstimate(progress: CalculationProgress | null, elapsedMilliseconds: number): string | null {
  if (!progress || progress.completed < 3 || elapsedMilliseconds < 5000 || progress.completed >= progress.total) return null;
  const remaining = (elapsedMilliseconds / progress.completed) * (progress.total - progress.completed);
  if (remaining < 60_000) return "moins d'une minute";
  const minutes = Math.round(remaining / 60_000);
  return minutes >= 60 ? `environ ${Math.floor(minutes / 60)} h ${String(minutes % 60).padStart(2, "0")}` : `environ ${minutes} min`;
}

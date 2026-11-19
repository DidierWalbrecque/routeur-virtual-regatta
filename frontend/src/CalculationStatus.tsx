import { useEffect, useState } from "react";
import { formatClock, progressFraction, remainingEstimate, type CalculationProgress } from "./lib/calculation_progress.ts";

type Props = {
  progress: CalculationProgress | null;
  startedAt: number;
  onCancel: () => void;
  /** Origine du vent quand le run le plus récent est encore partiel. */
  weatherNote?: string;
  weatherWarning?: boolean;
};

/** Fenêtre d'avancement du calcul de route, avec un bouton pour l'annuler. */
export function CalculationStatus({ progress, startedAt, onCancel, weatherNote = "", weatherWarning = false }: Props) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, []);

  // Échap annule le calcul, comme le bouton.
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") onCancel();
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [onCancel]);

  const elapsed = now - startedAt;
  const fraction = progressFraction(progress);
  const percent = Math.round(fraction * 100);
  const remaining = remainingEstimate(progress, elapsed);
  const known = Boolean(progress && progress.total > 0);

  return (
    <div id="route-calculation-status" className="route-calculation-status" role="status" aria-live="polite">
      <div className="route-calculation-header">
        <span className="material-symbols-outlined route-calculation-spinner" aria-hidden="true">
          progress_activity
        </span>
        <strong>Calcul de la route</strong>
        <span className="route-calculation-elapsed" title="Temps écoulé">
          {formatClock(elapsed)}
        </span>
      </div>
      <div
        className={known ? "route-calculation-bar" : "route-calculation-bar is-indeterminate"}
        role="progressbar"
        aria-label="Avancement du calcul"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={known ? percent : undefined}
      >
        <div style={known ? { width: `${percent}%` } : undefined} />
      </div>
      <div className="route-calculation-meta">
        {known && progress ? (
          <>
            <span>
              Front {progress.completed} / {progress.total} · {percent} %
            </span>
            {remaining && <span>Reste {remaining}</span>}
          </>
        ) : (
          <span>Préparation : météo, polaire et côtes…</span>
        )}
      </div>
      {weatherNote && (
        <p className={weatherWarning ? "route-calculation-weather is-warning" : "route-calculation-weather"}>
          <span className="material-symbols-outlined" aria-hidden="true">
            {weatherWarning ? "warning" : "cloud_sync"}
          </span>
          {weatherNote}
        </p>
      )}
      <button type="button" className="route-calculation-cancel" onClick={onCancel}>
        <span className="material-symbols-outlined" aria-hidden="true">
          close
        </span>
        Annuler le routage
        <kbd>Échap</kbd>
      </button>
    </div>
  );
}

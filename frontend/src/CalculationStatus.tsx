import { useEffect, useState } from "react";
import { formatClock, progressFraction, remainingEstimate, type CalculationProgress } from "./lib/calculation_progress.ts";

type Props = {
  progress: CalculationProgress | null;
  startedAt: number;
  onCancel: () => void;
  /** Origine du vent quand le run le plus récent est encore partiel. */
  weatherNote?: string;
  weatherWarning?: boolean;
  /** Textes de la fenêtre : par défaut, ceux du calcul de route. */
  id?: string;
  title?: string;
  /** Unité comptée dans l'avancement (« Front », « Départ »). */
  unitLabel?: string;
  preparingText?: string;
  cancelLabel?: string;
  /** Échap annule ce calcul ; à couper quand une autre fenêtre le gère déjà. */
  cancelOnEscape?: boolean;
  /** Décale la fenêtre sous celle du calcul de route. */
  stacked?: boolean;
};

/** Fenêtre d'avancement du calcul de route, avec un bouton pour l'annuler. */
export function CalculationStatus({
  progress,
  startedAt,
  onCancel,
  weatherNote = "",
  weatherWarning = false,
  id = "route-calculation-status",
  title = "Calcul de la route",
  unitLabel = "Front",
  preparingText = "Préparation : météo, polaire et côtes…",
  cancelLabel = "Annuler le routage",
  cancelOnEscape = true,
  stacked = false,
}: Props) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, []);

  // Échap annule le calcul, comme le bouton.
  useEffect(() => {
    if (!cancelOnEscape) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") onCancel();
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [onCancel, cancelOnEscape]);

  const elapsed = now - startedAt;
  const fraction = progressFraction(progress);
  const percent = Math.round(fraction * 100);
  const remaining = remainingEstimate(progress, elapsed);
  const known = Boolean(progress && progress.total > 0);

  return (
    <div id={id} className={stacked ? "route-calculation-status is-stacked" : "route-calculation-status"} role="status" aria-live="polite">
      <div className="route-calculation-header">
        <span className="material-symbols-outlined route-calculation-spinner" aria-hidden="true">
          progress_activity
        </span>
        <strong>{title}</strong>
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
              {unitLabel} {progress.completed} / {progress.total} · {percent} %
            </span>
            {remaining && <span>Reste {remaining}</span>}
          </>
        ) : (
          <span>{preparingText}</span>
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
        {cancelLabel}
        {cancelOnEscape && <kbd>Échap</kbd>}
      </button>
    </div>
  );
}

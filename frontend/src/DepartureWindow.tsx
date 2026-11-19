import { useEffect, useRef, useState } from "react";
import {
  bestDepartureIndex,
  COMPARISON_HOURS,
  DEPARTURE_STEP_HOURS,
  departureGapNM,
  departureSummary,
  type DepartureOutcome,
  type DepartureWindowStart,
} from "./lib/departure_window.ts";

export type DepartureWindowHandlers = {
  onStart: (start: DepartureWindowStart) => void;
  onOutcome: (outcome: DepartureOutcome) => void;
};

type Props = {
  /** Lance la comparaison ; se termine à la fin du flux ou lève une erreur. */
  onRun: (comparisonHours: number, stepHours: number, handlers: DepartureWindowHandlers, signal: AbortSignal) => Promise<void>;
  /** Reporte l'heure de départ choisie dans le formulaire. */
  onPick: (departure: Date) => void;
  /** Noms affichés des portes restantes, dans l'ordre de passage. */
  gateNames: string[];
};

const dateFormat: Intl.DateTimeFormatOptions = { weekday: "short", day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" };

function formatDeparture(value: string): string {
  return new Date(value).toLocaleString("fr-FR", dateFormat);
}

/**
 * Comparaison des heures de départ d'un record : chaque départ possible est
 * routé pendant la même durée, le plus avancé est mis en avant.
 */
export function DepartureWindow(props: Props) {
  const [comparisonHours, setComparisonHours] = useState(72);
  // Un départ toutes les 12 h : environ 25 départs sur 16 jours de prévisions,
  // à une dizaine de secondes chacun.
  const [stepHours, setStepHours] = useState(12);
  const [start, setStart] = useState<DepartureWindowStart | null>(null);
  const [outcomes, setOutcomes] = useState<DepartureOutcome[]>([]);
  const [running, setRunning] = useState(false);
  const [error, setError] = useState("");
  const [picked, setPicked] = useState("");
  const controller = useRef<AbortController | null>(null);

  useEffect(() => () => controller.current?.abort(), []);

  const run = async () => {
    controller.current?.abort();
    const current = new AbortController();
    controller.current = current;
    setStart(null);
    setOutcomes([]);
    setError("");
    setPicked("");
    setRunning(true);
    try {
      await props.onRun(
        comparisonHours,
        stepHours,
        {
          onStart: setStart,
          onOutcome: (outcome) => setOutcomes((previous) => [...previous, outcome]),
        },
        current.signal,
      );
    } catch (failure) {
      if (!current.signal.aborted) setError((failure as Error).message);
    } finally {
      if (controller.current === current) setRunning(false);
    }
  };

  const cancel = () => {
    controller.current?.abort();
    setRunning(false);
  };

  const best = bestDepartureIndex(outcomes);
  const total = start?.departures.length ?? 0;
  const closing = start?.windowClose ? new Date(start.windowClose).toLocaleString("fr-FR", dateFormat) : null;

  return (
    <section className="form-section departure-window" aria-labelledby="route-section-departure">
      <h3 id="route-section-departure">Meilleure heure de départ</h3>
      <p className="field-note">
        Chaque départ possible est routé pendant la même durée, puis les positions atteintes sont comparées : arrivée, portes franchies,
        distance à la porte suivante. Comptez une dizaine de secondes par départ.
      </p>
      <div className="form-grid">
        <div className="field">
          <label htmlFor="departure-comparison">Durée comparée</label>
          <select id="departure-comparison" value={comparisonHours} disabled={running} onChange={(event) => setComparisonHours(Number(event.target.value))}>
            {COMPARISON_HOURS.map((hours) => (
              <option key={hours} value={hours}>
                {hours} h
              </option>
            ))}
          </select>
        </div>
        <div className="field">
          <label htmlFor="departure-step">Un départ toutes les</label>
          <select id="departure-step" value={stepHours} disabled={running} onChange={(event) => setStepHours(Number(event.target.value))}>
            {DEPARTURE_STEP_HOURS.map((hours) => (
              <option key={hours} value={hours}>
                {hours} h
              </option>
            ))}
          </select>
        </div>
      </div>
      {running ? (
        <button type="button" className="secondary-button" onClick={cancel}>
          <span className="material-symbols-outlined" aria-hidden="true">
            stop_circle
          </span>
          Arrêter ({outcomes.length}/{total || "…"})
        </button>
      ) : (
        <button type="button" className="secondary-button" onClick={() => void run()}>
          <span className="material-symbols-outlined" aria-hidden="true">
            schedule
          </span>
          Comparer les départs
        </button>
      )}
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      {start && (
        <p className="field-note">
          {total} départ{total > 1 ? "s" : ""} comparé{total > 1 ? "s" : ""} sur {start.comparisonHours} h, prévisions jusqu'au{" "}
          {new Date(start.coveredUntil).toLocaleString("fr-FR", dateFormat)}
          {closing ? ` · fermeture du leg le ${closing}` : ""}.
        </p>
      )}
      {outcomes.length > 0 && (
        <ol className="departure-list">
          {outcomes.map((outcome, index) => {
            const gap = best >= 0 && index !== best ? departureGapNM(outcome, outcomes[best]) : null;
            const classes = ["departure-row", index === best ? "is-best" : "", outcome.departure === picked ? "is-picked" : ""].filter(Boolean).join(" ");
            return (
              <li key={outcome.departure}>
                <button
                  type="button"
                  className={classes}
                  title="Reprendre cette heure de départ dans le formulaire"
                  onClick={() => {
                    setPicked(outcome.departure);
                    props.onPick(new Date(outcome.departure));
                  }}
                >
                  <span className="departure-time">{formatDeparture(outcome.departure)}</span>
                  <span className="departure-summary">
                    {departureSummary(outcome, start?.totalGates ?? 0, props.gateNames)}
                    {gap !== null && gap >= 1 ? ` · +${Math.round(gap)} NM` : ""}
                  </span>
                  {index === best && <span className="departure-badge">{running ? "en tête" : "meilleur"}</span>}
                </button>
              </li>
            );
          })}
        </ol>
      )}
    </section>
  );
}

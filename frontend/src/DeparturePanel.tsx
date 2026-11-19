import { useEffect, useRef, useState } from "react";
import {
  bestDepartureIndex,
  clampPage,
  COMPARISON_HOURS,
  DEPARTURES_PER_PAGE,
  DEPARTURE_STEP_HOURS,
  departureGapNM,
  departureTarget,
  pageCount,
  pageOf,
  type DepartureOutcome,
  type DepartureWindowStart,
} from "./lib/departure_window.ts";
import { formatDuration } from "./lib/horizon.ts";

export type DepartureWindowHandlers = {
  onStart: (start: DepartureWindowStart) => void;
  onOutcome: (outcome: DepartureOutcome) => void;
};

export type DepartureWindowRun = (
  comparisonHours: number,
  stepHours: number,
  handlers: DepartureWindowHandlers,
  signal: AbortSignal,
) => Promise<void>;

export type DepartureWindowModel = ReturnType<typeof useDepartureWindow>;

/**
 * État de la comparaison des départs. Il vit dans App : le panneau peut être
 * fermé pendant le calcul (plusieurs minutes), qui continue avec sa fenêtre
 * d'avancement et garde ses résultats.
 */
export function useDepartureWindow(onRun: DepartureWindowRun, onFinished: () => void) {
  const [comparisonHours, setComparisonHours] = useState(72);
  // Un départ toutes les 12 h : environ 25 départs sur 16 jours de prévisions,
  // à une dizaine de secondes chacun.
  const [stepHours, setStepHours] = useState(12);
  const [start, setStart] = useState<DepartureWindowStart | null>(null);
  const [outcomes, setOutcomes] = useState<DepartureOutcome[]>([]);
  const [running, setRunning] = useState(false);
  const [startedAt, setStartedAt] = useState(0);
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
    setStartedAt(Date.now());
    setRunning(true);
    try {
      await onRun(
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
      if (controller.current === current) {
        setRunning(false);
        onFinished();
      }
    }
  };

  const cancel = () => {
    controller.current?.abort();
    setRunning(false);
    onFinished();
  };

  const progress = start ? { completed: outcomes.length, total: start.departures.length } : null;
  return { comparisonHours, setComparisonHours, stepHours, setStepHours, start, outcomes, running, startedAt, progress, error, picked, setPicked, run, cancel };
}

const dateFormat: Intl.DateTimeFormatOptions = { weekday: "short", day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" };

function formatDate(value: string): string {
  return new Date(value).toLocaleString("fr-FR", dateFormat);
}

type Props = {
  model: DepartureWindowModel;
  /** Noms affichés des portes restantes, dans l'ordre de passage. */
  gateNames: string[];
  /** Reporte l'heure de départ dans le formulaire et l'ouvre. */
  onUse: (departure: Date) => void;
  onClose: () => void;
};

/**
 * Panneau de la meilleure heure de départ d'un record : réglages, lancement,
 * puis tableau des départs comparés.
 */
export function DeparturePanel({ model, gateNames, onUse, onClose }: Props) {
  const { comparisonHours, setComparisonHours, stepHours, setStepHours, start, outcomes, running, error, picked, setPicked } = model;
  const best = bestDepartureIndex(outcomes);
  const total = start?.departures.length ?? 0;
  const totalGates = start?.totalGates ?? 0;
  const selected = outcomes.find((outcome) => outcome.departure === picked) ?? (best >= 0 ? outcomes[best] : null);
  const selectedIndex = selected ? outcomes.indexOf(selected) : -1;

  // Le tableau s'ouvre sur la page du départ choisi (le meilleur par défaut).
  const [page, setPage] = useState(() => pageOf(selectedIndex));
  useEffect(() => {
    if (outcomes.length === 0) setPage(0);
  }, [outcomes.length]);
  const pages = pageCount(outcomes.length);
  const currentPage = clampPage(page, outcomes.length);
  const first = currentPage * DEPARTURES_PER_PAGE;
  const visible = outcomes.slice(first, first + DEPARTURES_PER_PAGE);
  const bestPage = pageOf(best);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !running) onClose();
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [onClose, running]);

  return (
    <aside id="departure-panel" className="route-details-panel departure-panel" role="dialog" aria-modal="false" aria-labelledby="departure-title">
      <div className="route-form-header route-details-header">
        <div>
          <h2 id="departure-title">Meilleure heure de départ</h2>
          <p className="route-details-status">
            Chaque départ possible est routé pendant la même durée. Le classement retient l'arrivée, puis les portes franchies, puis la
            distance à la porte suivante.
          </p>
        </div>
        <div className="route-details-actions">
          <button type="button" className="close-button" title="Fermer" aria-label="Fermer" onClick={onClose}>
            <span className="material-symbols-outlined" aria-hidden="true">
              close
            </span>
          </button>
        </div>
      </div>

      <div className="departure-body">
        <div className="departure-settings">
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
          <button type="button" className="form-submit departure-run" disabled={running} onClick={() => void model.run()}>
            <span className="material-symbols-outlined" aria-hidden="true">
              schedule
            </span>
            {running ? "Comparaison en cours…" : outcomes.length > 0 ? "Relancer la comparaison" : "Comparer les départs"}
          </button>
        </div>
        <p className="field-note">Comptez une dizaine de secondes par départ.</p>

        {error && (
          <p className="form-error" role="alert">
            {error}
          </p>
        )}

        {start && (
          <p className="field-note">
            {total} départ{total > 1 ? "s" : ""} sur {start.comparisonHours} h de route · prévisions jusqu'au {formatDate(start.coveredUntil)}
            {start.windowClose ? ` · fermeture du leg le ${formatDate(start.windowClose)}` : ""}
          </p>
        )}

        {outcomes.length > 0 && (
          <div className="route-details-table-wrapper">
            <table className="route-details-table route-report-table departure-table">
              <thead>
                <tr>
                  <th>Départ</th>
                  <th title={`Position au bout de ${start?.comparisonHours ?? comparisonHours} h de route`}>Portes</th>
                  <th>Porte suivante</th>
                  <th className="numeric" title="Distance au point le plus proche de la porte suivante">
                    Distance
                  </th>
                  <th className="numeric" title="Retard sur le meilleur départ, quand les deux visent la même porte">
                    Écart
                  </th>
                </tr>
              </thead>
              <tbody>
                {visible.map((outcome, offset) => {
                  const index = first + offset;
                  const gap = best >= 0 && index !== best ? departureGapNM(outcome, outcomes[best]) : null;
                  const isSelected = selected?.departure === outcome.departure;
                  const classes = ["route-report-row", index === best ? "departure-best" : "", isSelected ? "route-report-selected" : ""].filter(Boolean);
                  const choose = () => setPicked(outcome.departure);
                  return (
                    <tr
                      key={outcome.departure}
                      className={classes.join(" ")}
                      tabIndex={0}
                      aria-selected={isSelected}
                      onClick={choose}
                      onKeyDown={(event) => {
                        if (event.key === "Enter" || event.key === " ") {
                          event.preventDefault();
                          choose();
                        }
                      }}
                    >
                      <td>
                        {formatDate(outcome.departure)}
                        {index === best && <span className="departure-badge">{running ? "en tête" : "meilleur"}</span>}
                      </td>
                      <td>{outcome.reached ? "arrivée" : `${outcome.gatesCrossed}/${totalGates}`}</td>
                      <td>{outcome.reached ? `en ${formatDuration(outcome.elapsedHours * 60)}` : departureTarget(outcome, totalGates, gateNames)}</td>
                      <td className="numeric">{outcome.reached ? "—" : `${Math.round(outcome.distanceNM)} NM`}</td>
                      <td className="numeric">{index === best ? "—" : gap === null ? "autre porte" : `+${Math.round(gap)} NM`}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}

        {pages > 1 && (
          <nav className="departure-pager" aria-label="Pages des départs">
            <span className="departure-pager-range">
              Départs {first + 1}–{first + visible.length} sur {outcomes.length}
            </span>
            <div className="departure-pager-buttons">
              <button
                type="button"
                className="departure-page-button"
                aria-label="Page précédente"
                disabled={currentPage === 0}
                onClick={() => setPage(currentPage - 1)}
              >
                <span className="material-symbols-outlined" aria-hidden="true">
                  chevron_left
                </span>
              </button>
              {Array.from({ length: pages }, (_, index) => (
                <button
                  key={index}
                  type="button"
                  className={index === currentPage ? "departure-page-button is-current" : "departure-page-button"}
                  aria-label={`Page ${index + 1}${index === bestPage && best >= 0 ? ", meilleur départ" : ""}`}
                  aria-current={index === currentPage ? "page" : undefined}
                  onClick={() => setPage(index)}
                >
                  {index + 1}
                  {index === bestPage && best >= 0 && <span className="departure-page-best" aria-hidden="true" />}
                </button>
              ))}
              <button
                type="button"
                className="departure-page-button"
                aria-label="Page suivante"
                disabled={currentPage >= pages - 1}
                onClick={() => setPage(currentPage + 1)}
              >
                <span className="material-symbols-outlined" aria-hidden="true">
                  chevron_right
                </span>
              </button>
            </div>
            {best >= 0 && bestPage !== currentPage && (
              <button type="button" className="departure-pager-best" onClick={() => setPage(bestPage)}>
                Voir le meilleur départ
              </button>
            )}
          </nav>
        )}
      </div>

      {selected && !running && (
        <footer className="departure-footer">
          <button type="button" className="form-submit" onClick={() => onUse(new Date(selected.departure))}>
            <span className="material-symbols-outlined" aria-hidden="true">
              alt_route
            </span>
            Calculer la route avec le départ du {formatDate(selected.departure)}
          </button>
        </footer>
      )}
    </aside>
  );
}

import { useEffect, useRef, useState } from "react";
import {
  bestDepartureIndex,
  clampPage,
  COMPARISON_HOURS,
  DEPARTURES_PER_PAGE,
  DEPARTURE_STEP_HOURS,
  departureBars,
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

const longDateFormat: Intl.DateTimeFormatOptions = { weekday: "long", day: "2-digit", month: "2-digit" };
const timeFormat: Intl.DateTimeFormatOptions = { hour: "2-digit", minute: "2-digit" };
const dayFormat: Intl.DateTimeFormatOptions = { weekday: "short", day: "numeric" };

function formatDate(value: string): string {
  return new Date(value).toLocaleString("fr-FR", dateFormat);
}

/** « lundi 23/11 à 01:00 » */
function formatLongDate(value: string): string {
  const date = new Date(value);
  return `${date.toLocaleDateString("fr-FR", longDateFormat)} à ${date.toLocaleTimeString("fr-FR", timeFormat)}`;
}

/** Jour local d'un instant, pour repérer les changements de jour sur l'axe. */
function localDay(value: string): string {
  return new Date(value).toDateString();
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
 * Panneau de la meilleure heure de départ d'un record : réglages, graphique de
 * la fenêtre de départ (un trait par départ, d'autant plus haut qu'il prend
 * d'avance), puis tableau paginé des départs comparés.
 */
export function DeparturePanel({ model, gateNames, onUse, onClose }: Props) {
  const { comparisonHours, setComparisonHours, stepHours, setStepHours, start, outcomes, running, error, picked, setPicked } = model;
  const best = bestDepartureIndex(outcomes);
  const total = start?.departures.length ?? 0;
  const totalGates = start?.totalGates ?? 0;
  const hours = start?.comparisonHours ?? comparisonHours;
  const selected = outcomes.find((outcome) => outcome.departure === picked) ?? (best >= 0 ? outcomes[best] : null);
  const selectedIndex = selected ? outcomes.indexOf(selected) : -1;
  const bars = departureBars(outcomes);

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

  const choose = (index: number) => {
    setPicked(outcomes[index].departure);
    setPage(pageOf(index));
  };

  const where = (outcome: DepartureOutcome) =>
    outcome.reached
      ? `arrive en ${formatDuration(outcome.elapsedHours * 60)}`
      : `est à ${Math.round(outcome.distanceNM)} NM de ${departureTarget(outcome, totalGates, gateNames, true)} après ${hours} h`;

  /** La phrase sous le graphique : où en est le départ choisi, et face au meilleur. */
  const verdict = (() => {
    if (!selected || best < 0) return null;
    const date = formatLongDate(selected.departure);
    if (selectedIndex === best) {
      return running ? `En tête pour l'instant : le départ du ${date} ${where(selected)}.` : `Le départ du ${date} est le meilleur : il ${where(selected)}.`;
    }
    const gap = departureGapNM(selected, outcomes[best]);
    const behind = gap === null ? "sur une autre cible que le meilleur" : `${Math.round(gap)} NM derrière le meilleur`;
    return `Le départ du ${date} ${where(selected)}, ${behind}.`;
  })();

  // Axe du temps : chaque départ prévu, calculé ou encore attendu.
  const slots = (start?.departures ?? []).map((departure, index) => ({
    departure,
    index,
    outcomeIndex: outcomes.findIndex((outcome) => outcome.departure === departure),
  }));

  return (
    <aside id="departure-panel" className="route-details-panel departure-panel" role="dialog" aria-modal="false" aria-labelledby="departure-title">
      <div className="route-form-header route-details-header">
        <div>
          <h2 id="departure-title">Meilleure heure de départ</h2>
          <p className="route-details-summary">Chaque départ possible est routé pendant la même durée, puis comparé aux autres sur la position atteinte.</p>
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
              {COMPARISON_HOURS.map((value) => (
                <option key={value} value={value}>
                  {value} h
                </option>
              ))}
            </select>
          </div>
          <div className="field">
            <label htmlFor="departure-step">Un départ toutes les</label>
            <select id="departure-step" value={stepHours} disabled={running} onChange={(event) => setStepHours(Number(event.target.value))}>
              {DEPARTURE_STEP_HOURS.map((value) => (
                <option key={value} value={value}>
                  {value} h
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
        {error && (
          <p className="form-error" role="alert">
            {error}
          </p>
        )}

        {!start && !error && (
          <p className="departure-empty">
            Choisissez la durée comparée et l'écart entre deux départs, puis lancez la comparaison. Comptez une dizaine de secondes par départ.
          </p>
        )}

        {start && (
          <figure className="departure-window">
            <div className="departure-chart" role="group" aria-label={`Fenêtre de départ : avance prise après ${hours} h de route, un trait par départ`}>
              {slots.map((slot) => {
                const done = slot.outcomeIndex >= 0;
                const outcome = done ? outcomes[slot.outcomeIndex] : null;
                const bar = done ? bars[slot.outcomeIndex] : null;
                const newDay = slot.index === 0 || localDay(slot.departure) !== localDay(slots[slot.index - 1].departure);
                const classes = [
                  "departure-bar",
                  !done ? "is-pending" : "",
                  bar && !bar.comparable ? "is-off" : "",
                  done && slot.outcomeIndex === best ? "is-best" : "",
                  done && slot.outcomeIndex === selectedIndex ? "is-selected" : "",
                ].filter(Boolean);
                const label = outcome
                  ? `${formatDate(slot.departure)} : ${outcome.reached ? "arrivée" : `${Math.round(outcome.distanceNM)} NM de ${departureTarget(outcome, totalGates, gateNames)}`}`
                  : `${formatDate(slot.departure)} : en attente`;
                return (
                  <div className="departure-slot" key={slot.departure}>
                    <button
                      type="button"
                      className={classes.join(" ")}
                      disabled={!done}
                      aria-pressed={done ? slot.outcomeIndex === selectedIndex : undefined}
                      aria-label={label}
                      title={label}
                      onClick={() => choose(slot.outcomeIndex)}
                    >
                      <span style={{ height: `${Math.round((bar?.value ?? 1) * 100)}%` }} />
                    </button>
                    <span className="departure-day" aria-hidden="true">
                      {newDay && (
                        <>
                          <span className="departure-day-long">{new Date(slot.departure).toLocaleDateString("fr-FR", dayFormat)}</span>
                          <span className="departure-day-short">{new Date(slot.departure).getDate()}</span>
                        </>
                      )}
                    </span>
                  </div>
                );
              })}
            </div>
            <figcaption>
              <p className={verdict ? "departure-verdict" : "departure-verdict is-waiting"}>{verdict ?? "Premier départ en cours de calcul…"}</p>
              <p className="departure-context">
                {total} départ{total > 1 ? "s" : ""} sur {hours} h de route. Prévisions jusqu'au {formatDate(start.coveredUntil)}
                {start.windowClose ? `, fermeture du leg le ${formatDate(start.windowClose)}` : ""}.
              </p>
            </figcaption>
          </figure>
        )}

        {outcomes.length > 0 && (
          <div className="route-details-table-wrapper">
            <table className="route-details-table route-report-table departure-table">
              <thead>
                <tr>
                  <th>Départ</th>
                  <th title={`Portes franchies au bout de ${hours} h de route`}>Portes</th>
                  <th>Cible suivante</th>
                  <th className="numeric" title="Distance au point le plus proche de la cible suivante">
                    Distance
                  </th>
                  <th className="numeric" title="Retard sur le meilleur départ, quand les deux visent la même cible">
                    Retard
                  </th>
                </tr>
              </thead>
              <tbody>
                {visible.map((outcome, offset) => {
                  const index = first + offset;
                  const gap = best >= 0 && index !== best ? departureGapNM(outcome, outcomes[best]) : null;
                  const isSelected = index === selectedIndex;
                  const classes = ["route-report-row", index === best ? "departure-best" : "", isSelected ? "route-report-selected" : ""].filter(Boolean);
                  return (
                    <tr
                      key={outcome.departure}
                      className={classes.join(" ")}
                      tabIndex={0}
                      aria-selected={isSelected}
                      onClick={() => choose(index)}
                      onKeyDown={(event) => {
                        if (event.key === "Enter" || event.key === " ") {
                          event.preventDefault();
                          choose(index);
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
                      <td className="numeric">{index === best ? "—" : gap === null ? "autre cible" : `${Math.round(gap)} NM`}</td>
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
              Départs {first + 1} à {first + visible.length} sur {outcomes.length}
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

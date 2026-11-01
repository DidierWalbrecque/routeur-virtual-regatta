import { useEffect, useMemo, useRef } from "react";
import {
  REPORT_HEADERS,
  buildRouteDetailsModel,
  formatAngle,
  formatElapsed,
  formatInstant,
  formatNumber,
  formatPosition,
} from "./lib/route_details.ts";
import type { Point, RouteResult } from "./lib/types.ts";

type Props = {
  result: RouteResult;
  /** Indice de la ligne choisie, ou null. */
  selectedIndex: number | null;
  onSelect: (index: number, point: Point) => void;
  onClose: () => void;
};

export function RouteDetailsPanel({ result, selectedIndex, onSelect, onClose }: Props) {
  const model = useMemo(() => buildRouteDetailsModel(result), [result]);
  const closeButton = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    closeButton.current?.focus();
  }, []);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [onClose]);

  const duration = model.durationMinutes === null ? "—" : formatElapsed(model.durationMinutes).slice(1);

  return (
    <aside
      id="route-details-panel"
      className="route-details-panel"
      role="dialog"
      aria-modal="true"
      aria-labelledby="route-details-title"
    >
      <div className="route-form-header">
        <h2 id="route-details-title">Détails de la route</h2>
        <button
          id="close-route-details"
          ref={closeButton}
          className="close-button"
          type="button"
          title="Fermer"
          aria-label="Fermer"
          onClick={onClose}
        >
          ×
        </button>
      </div>
      <div id="route-details-content">
        <p className={model.destinationReached ? "route-details-status reached" : "route-details-status partial"}>
          {model.statusText}
        </p>
        <p className="route-details-summary">
          {`${model.pointCount} point(s) · durée calculée : ${duration} · ${model.maneuverCount} manœuvre(s)`}
        </p>
        <div className="route-details-table-wrapper">
          <table className="route-details-table route-report-table">
            <thead>
              <tr>
                {REPORT_HEADERS.map((header) => (
                  <th key={header.label} title={header.title}>
                    {header.label}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {model.rows.map((row) => {
                const select = () => {
                  if (row.latitude !== null && row.longitude !== null) {
                    onSelect(row.index, { latitude: row.latitude, longitude: row.longitude });
                  }
                };
                const classes = [
                  "route-report-row",
                  row.maneuver ? "route-report-maneuver" : "",
                  row.index === selectedIndex ? "route-report-selected" : "",
                ].filter(Boolean);
                return (
                <tr
                  key={row.index}
                  className={classes.join(" ")}
                  tabIndex={0}
                  aria-selected={row.index === selectedIndex}
                  title="Afficher cette position sur la route"
                  onClick={select}
                  onKeyDown={(event) => {
                    if (event.key === "Enter" || event.key === " ") {
                      event.preventDefault();
                      select();
                    }
                  }}
                >
                  <td>{formatInstant(row.instant)}</td>
                  <td>{formatElapsed(row.elapsedMinutes)}</td>
                  <td className="route-report-heading">
                    {row.heading !== null && Number.isFinite(row.heading) ? `${Math.round(row.heading)}°` : "—"}
                  </td>
                  <td>{row.windAngle !== null ? `${formatAngle(row.windAngle)}°` : "—"}</td>
                  <td>{row.tack}</td>
                  <td>{formatPosition(row.latitude, row.longitude)}</td>
                  <td>{row.windDirection !== null ? `${Math.round(row.windDirection)}°` : "—"}</td>
                  <td>{formatNumber(row.windKnots, 2)}</td>
                  <td>{formatNumber(row.speedKnots, 2)}</td>
                  <td>{row.foilPercent !== null ? Math.round(row.foilPercent) : "—"}</td>
                  <td>{row.speedCoefficient !== null ? row.speedCoefficient.toFixed(2) : "—"}</td>
                  <td className="route-report-sail">{row.sail}</td>
                  <td>{row.stamina === null ? "—" : `${row.stamina.toFixed(1)} %`}</td>
                </tr>
                );
              })}
            </tbody>
          </table>
        </div>
        <p className="route-report-legend">
          Ligne surlignée en jaune : manœuvre (virement, empannage ou changement de voile) sur ce pas. Cliquez sur une
          ligne pour afficher sa position sur la route.
        </p>
      </div>
    </aside>
  );
}

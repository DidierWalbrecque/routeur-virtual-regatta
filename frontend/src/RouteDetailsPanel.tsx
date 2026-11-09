import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import {
  REPORT_HEADERS,
  buildRouteDetailsModel,
  constantColumns,
  formatAngle,
  formatElapsed,
  formatInstant,
  formatNumber,
  formatPosition,
  maneuverLabel,
  type ReportRow,
} from "./lib/route_details.ts";
import { exportFileName, routeRowsToTsv, routeToGpx } from "./lib/route_export.ts";
import type { Point, RouteResult } from "./lib/types.ts";

type Props = {
  result: RouteResult;
  /** Indice de la ligne choisie, ou null. */
  selectedIndex: number | null;
  onSelect: (index: number, point: Point) => void;
  onClose: () => void;
};

type ColumnKey = "date" | "elapsed" | "heading" | "twa" | "tack" | "position" | "twd" | "tws" | "speed" | "foils" | "coeff" | "sail" | "stamina";

type Column = {
  key: ColumnKey;
  className?: string;
  render: (row: ReportRow) => ReactNode;
};

const degrees = (value: number | null) => (value !== null && Number.isFinite(value) ? `${Math.round(value)}°` : "—");

const COLUMNS: Column[] = [
  { key: "date", className: "route-report-date", render: (row) => formatInstant(row.instant) },
  { key: "elapsed", render: (row) => formatElapsed(row.elapsedMinutes) },
  { key: "heading", className: "route-report-heading", render: (row) => degrees(row.heading) },
  { key: "twa", render: (row) => (row.windAngle !== null ? `${formatAngle(row.windAngle)}°` : "—") },
  { key: "tack", render: (row) => row.tack },
  { key: "position", render: (row) => formatPosition(row.latitude, row.longitude) },
  { key: "twd", render: (row) => degrees(row.windDirection) },
  { key: "tws", render: (row) => formatNumber(row.windKnots, 1) },
  { key: "speed", render: (row) => formatNumber(row.speedKnots, 1) },
  { key: "foils", render: (row) => (row.foilPercent !== null ? Math.round(row.foilPercent) : "—") },
  { key: "coeff", render: (row) => formatNumber(row.speedCoefficient, 2) },
  { key: "sail", className: "route-report-sail", render: (row) => row.sail },
  { key: "stamina", render: (row) => (row.stamina === null ? "—" : `${formatNumber(row.stamina, 1)} %`) },
];

function download(content: string, type: string, name: string) {
  const url = URL.createObjectURL(new Blob([content], { type }));
  const link = document.createElement("a");
  link.href = url;
  link.download = name;
  document.body.append(link);
  link.click();
  link.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

const NARROW_QUERY = "(max-width: 600px)";

/** Vrai sur un écran de téléphone ; suit les changements de taille. */
function useNarrowScreen(): boolean {
  const [narrow, setNarrow] = useState(() => {
    try {
      return window.matchMedia(NARROW_QUERY).matches;
    } catch {
      return false;
    }
  });
  useEffect(() => {
    let query: MediaQueryList;
    try {
      query = window.matchMedia(NARROW_QUERY);
    } catch {
      return;
    }
    const onChange = () => setNarrow(query.matches);
    query.addEventListener("change", onChange);
    return () => query.removeEventListener("change", onChange);
  }, []);
  return narrow;
}

/** Une étape de la route sous forme de carte, pour les écrans étroits. */
function StepCard({ row, selected, onSelect }: { row: ReportRow; selected: boolean; onSelect: () => void }) {
  const classes = ["route-card", row.maneuver ? "is-maneuver" : "", selected ? "is-selected" : ""].filter(Boolean).join(" ");
  const twa = row.windAngle !== null ? `${formatAngle(row.windAngle)}°${row.tack === "—" ? "" : ` ${row.tack}`}` : "—";
  return (
    <li>
      <button type="button" className={classes} data-index={row.index} aria-pressed={selected} onClick={onSelect}>
        <span className="route-card-head">
          <strong>{formatInstant(row.instant)}</strong>
          <span className="route-card-elapsed">{formatElapsed(row.elapsedMinutes)}</span>
          {row.maneuver && <span className="route-card-maneuver">{maneuverLabel(row.maneuver)}</span>}
        </span>
        <span className="route-card-grid">
          <span>
            <small>Cap</small>
            <b className="route-report-heading">{degrees(row.heading)}</b>
          </span>
          <span>
            <small>TWA</small>
            <b>{twa}</b>
          </span>
          <span>
            <small>Vent</small>
            <b>{row.windKnots === null ? "—" : `${formatNumber(row.windKnots, 1)} nd · ${degrees(row.windDirection)}`}</b>
          </span>
          <span>
            <small>Vitesse</small>
            <b>{row.speedKnots === null ? "—" : `${formatNumber(row.speedKnots, 1)} nd`}</b>
          </span>
          <span>
            <small>Voile</small>
            <b className="route-report-sail">{row.sail}</b>
          </span>
          <span>
            <small>Stamina</small>
            <b>{row.stamina === null ? "—" : `${formatNumber(row.stamina, 1)} %`}</b>
          </span>
        </span>
      </button>
    </li>
  );
}

function Icon({ name }: { name: string }) {
  return (
    <span className="material-symbols-outlined" aria-hidden="true">
      {name}
    </span>
  );
}

export function RouteDetailsPanel({ result, selectedIndex, onSelect, onClose }: Props) {
  const model = useMemo(() => buildRouteDetailsModel(result), [result]);
  const hidden = useMemo(() => constantColumns(model.rows), [model]);
  const columns = COLUMNS.filter((column) => !hidden.has(column.key as "foils" | "coeff"));
  const closeButton = useRef<HTMLButtonElement>(null);
  const [collapsed, setCollapsed] = useState(false);
  const narrow = useNarrowScreen();
  const [copyStatus, setCopyStatus] = useState("");

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

  // Une ligne choisie défile jusqu'à être visible (clic sur la carte ou au clavier).
  const body = useRef<HTMLElement>(null);
  useEffect(() => {
    if (selectedIndex === null) return;
    body.current?.querySelector<HTMLElement>(`[data-index="${selectedIndex}"]`)?.scrollIntoView({ block: "nearest" });
  }, [selectedIndex]);

  const duration = model.durationMinutes === null ? "—" : formatElapsed(model.durationMinutes).slice(1);
  const startedAt = result.route?.[0]?.instant;

  const copyTable = async () => {
    try {
      await navigator.clipboard.writeText(routeRowsToTsv(model.rows));
      setCopyStatus("Tableau copié : collez-le dans un tableur.");
    } catch {
      setCopyStatus("Copie impossible : le navigateur refuse l'accès au presse-papiers.");
    }
    window.setTimeout(() => setCopyStatus(""), 4000);
  };

  return (
    <aside
      id="route-details-panel"
      className={collapsed ? "route-details-panel is-collapsed" : "route-details-panel"}
      role="dialog"
      aria-modal="false"
      aria-labelledby="route-details-title"
    >
      <div className="route-form-header route-details-header">
        <div>
          <h2 id="route-details-title">Détails de la route</h2>
          <p className={model.destinationReached ? "route-details-status reached" : "route-details-status partial"}>
            {model.statusText}
          </p>
          <p className="route-details-summary">
            {`${model.pointCount} points · durée ${duration} · ${model.maneuverCount} manœuvre(s)`}
          </p>
        </div>
        <div className="route-details-actions">
          <button
            type="button"
            className="close-button"
            title="Exporter en GPX"
            aria-label="Exporter en GPX"
            onClick={() => download(routeToGpx(result), "application/gpx+xml", exportFileName(startedAt, "gpx"))}
          >
            <Icon name="download" />
          </button>
          <button type="button" className="close-button" title="Copier le tableau" aria-label="Copier le tableau" onClick={() => void copyTable()}>
            <Icon name="content_copy" />
          </button>
          <button
            type="button"
            className="close-button"
            title={collapsed ? "Afficher le tableau" : "Réduire au résumé"}
            aria-label={collapsed ? "Afficher le tableau" : "Réduire au résumé"}
            aria-expanded={!collapsed}
            aria-controls="route-details-content"
            onClick={() => setCollapsed((value) => !value)}
          >
            <Icon name={collapsed ? "expand_less" : "expand_more"} />
          </button>
          <button
            id="close-route-details"
            ref={closeButton}
            className="close-button"
            type="button"
            title="Fermer"
            aria-label="Fermer"
            onClick={onClose}
          >
            <Icon name="close" />
          </button>
        </div>
      </div>
      {copyStatus && (
        <p className="route-details-copy" role="status">
          {copyStatus}
        </p>
      )}
      <div id="route-details-content" hidden={collapsed}>
        {narrow ? (
          // Sur téléphone, une carte par étape : choisir une étape réduit le
          // panneau pour laisser voir sa position sur la carte.
          <ol className="route-cards" ref={body as React.RefObject<HTMLOListElement>}>
            {model.rows.map((row) => (
              <StepCard
                key={row.index}
                row={row}
                selected={row.index === selectedIndex}
                onSelect={() => {
                  if (row.latitude === null || row.longitude === null) return;
                  onSelect(row.index, { latitude: row.latitude, longitude: row.longitude });
                  setCollapsed(true);
                }}
              />
            ))}
          </ol>
        ) : (
        <div className="route-details-table-wrapper">
          <table className="route-details-table route-report-table">
            <thead>
              <tr>
                {columns.map((column) => {
                  const header = REPORT_HEADERS[COLUMNS.indexOf(column)];
                  return (
                    <th key={column.key} title={header.title} className={column.className}>
                      {header.label}
                    </th>
                  );
                })}
              </tr>
            </thead>
            <tbody ref={body as React.RefObject<HTMLTableSectionElement>}>
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
                    data-index={row.index}
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
                    {columns.map((column) => (
                      <td key={column.key} className={column.className}>
                        {column.render(row)}
                      </td>
                    ))}
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
        )}
        <p className="route-report-legend">
          {narrow
            ? "Étape en jaune : manœuvre (virement, empannage ou changement de voile). Touchez une étape pour la voir sur la carte."
            : "Ligne surlignée en jaune : manœuvre (virement, empannage ou changement de voile) sur ce pas. Cliquez sur une ligne pour afficher sa position sur la route."}
          {!narrow && hidden.size > 0 && ` Colonnes sans variation masquées : ${[...hidden].map((key) => (key === "foils" ? "Foils" : "Coeff")).join(", ")}.`}
        </p>
      </div>
    </aside>
  );
}

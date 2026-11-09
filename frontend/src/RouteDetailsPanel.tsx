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
  /** Masquée sur téléphone, où la place manque. */
  secondary?: boolean;
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
  { key: "position", secondary: true, render: (row) => formatPosition(row.latitude, row.longitude) },
  { key: "twd", secondary: true, render: (row) => degrees(row.windDirection) },
  { key: "tws", render: (row) => formatNumber(row.windKnots, 1) },
  { key: "speed", render: (row) => formatNumber(row.speedKnots, 1) },
  { key: "foils", secondary: true, render: (row) => (row.foilPercent !== null ? Math.round(row.foilPercent) : "—") },
  { key: "coeff", secondary: true, render: (row) => formatNumber(row.speedCoefficient, 2) },
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
  const body = useRef<HTMLTableSectionElement>(null);
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
        <div className="route-details-table-wrapper">
          <table className="route-details-table route-report-table">
            <thead>
              <tr>
                {columns.map((column) => {
                  const header = REPORT_HEADERS[COLUMNS.indexOf(column)];
                  return (
                    <th key={column.key} title={header.title} className={[column.className, column.secondary ? "is-secondary" : ""].filter(Boolean).join(" ") || undefined}>
                      {header.label}
                    </th>
                  );
                })}
              </tr>
            </thead>
            <tbody ref={body}>
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
                      <td key={column.key} className={[column.className, column.secondary ? "is-secondary" : ""].filter(Boolean).join(" ") || undefined}>
                        {column.render(row)}
                      </td>
                    ))}
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
        <p className="route-report-legend">
          Ligne surlignée en jaune : manœuvre (virement, empannage ou changement de voile) sur ce pas. Cliquez sur une
          ligne pour afficher sa position sur la route.
          {hidden.size > 0 && ` Colonnes sans variation masquées : ${[...hidden].map((key) => (key === "foils" ? "Foils" : "Coeff")).join(", ")}.`}
        </p>
      </div>
    </aside>
  );
}

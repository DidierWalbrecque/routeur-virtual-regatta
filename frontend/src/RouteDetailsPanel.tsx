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
import {
  buildGameOrders,
  gameOrdersToText,
  orderSetting,
  summarizeRoute,
  type GameOrder,
  type OrderMode,
} from "./lib/game_orders.ts";
import type { Point, RouteResult } from "./lib/types.ts";

type Props = {
  result: RouteResult;
  /** Indice de la ligne choisie, ou null. */
  selectedIndex: number | null;
  onSelect: (index: number, point: Point) => void;
  onClose: () => void;
  /** Ouvert réduit au résumé (juste après un calcul), sans prendre le focus. */
  initiallyCollapsed?: boolean;
};

type Tab = "orders" | "steps";
const PANEL_STORAGE_KEY = "routeur.routePanel";

function loadPanelPrefs(): { tab: Tab; mode: OrderMode } {
  try {
    const saved = JSON.parse(window.localStorage.getItem(PANEL_STORAGE_KEY) || "{}") as { tab?: string; mode?: string };
    return { tab: saved.tab === "steps" ? "steps" : "orders", mode: saved.mode === "heading" ? "heading" : "twa" };
  } catch {
    return { tab: "orders", mode: "twa" };
  }
}

/** « 1 h 20 », « 40 min », « 2 j 3 h ». */
function formatSpan(minutes: number | null): string {
  if (minutes === null || !Number.isFinite(minutes)) return "—";
  if (minutes < 60) return `${minutes} min`;
  const days = Math.floor(minutes / 1440);
  const hours = Math.floor((minutes % 1440) / 60);
  const rest = minutes % 60;
  if (days > 0) return hours ? `${days} j ${hours} h` : `${days} j`;
  return rest ? `${hours} h ${String(rest).padStart(2, "0")}` : `${hours} h`;
}

/** Un ordre à donner dans le jeu. */
function OrderItem(props: { order: GameOrder; mode: OrderMode; selected: boolean; onSelect: () => void }) {
  const { order, mode } = props;
  const classes = ["route-order", order.maneuver ? "is-maneuver" : "", props.selected ? "is-selected" : ""].filter(Boolean).join(" ");
  const secondary =
    mode === "twa"
      ? order.heading === null
        ? ""
        : `cap ${order.heading}°`
      : order.twa === null
        ? ""
        : `TWA ${order.twa}° ${order.tack}`;
  return (
    <li>
      <button type="button" className={classes} data-index={order.index} aria-pressed={props.selected} onClick={props.onSelect}>
        <span className="route-order-time">
          <strong>{formatInstant(order.instant).slice(-5)}</strong>
          <small>{formatElapsed(order.elapsedMinutes)}</small>
        </span>
        <span className="route-order-main">
          <span className="route-order-setting">{orderSetting(order, mode)}</span>
          <span className="route-order-meta">
            <span className="route-report-sail">{order.sail}</span>
            {secondary && <span>{secondary}</span>}
            <span>{`pendant ${formatSpan(order.durationMinutes)}`}</span>
          </span>
        </span>
        {order.maneuver && <span className="route-card-maneuver">{maneuverLabel(order.maneuver)}</span>}
      </button>
    </li>
  );
}

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

export function RouteDetailsPanel({ result, selectedIndex, onSelect, onClose, initiallyCollapsed = false }: Props) {
  const model = useMemo(() => buildRouteDetailsModel(result), [result]);
  const hidden = useMemo(() => constantColumns(model.rows), [model]);
  const columns = COLUMNS.filter((column) => !hidden.has(column.key as "foils" | "coeff"));
  const closeButton = useRef<HTMLButtonElement>(null);
  const [collapsed, setCollapsed] = useState(initiallyCollapsed);
  const narrow = useNarrowScreen();
  const [copyStatus, setCopyStatus] = useState("");
  const [prefs, setPrefs] = useState(loadPanelPrefs);
  const orders = useMemo(() => buildGameOrders(model.rows, prefs.mode), [model, prefs.mode]);
  const summary = useMemo(() => summarizeRoute(model.rows, model.maneuverCount, model.durationMinutes), [model]);

  useEffect(() => {
    try {
      window.localStorage.setItem(PANEL_STORAGE_KEY, JSON.stringify(prefs));
    } catch {
      // Stockage indisponible : le choix reste valable pour la session.
    }
  }, [prefs]);

  useEffect(() => {
    if (!initiallyCollapsed) closeButton.current?.focus();
    // eslint-disable-next-line react-hooks/exhaustive-deps
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
  }, [selectedIndex, prefs.tab]);

  const startedAt = result.route?.[0]?.instant;
  const select = (index: number, latitude: number | null, longitude: number | null) => {
    if (latitude === null || longitude === null) return;
    onSelect(index, { latitude, longitude });
    // Sur téléphone, le panneau se réduit pour laisser voir la position.
    if (narrow) setCollapsed(true);
  };

  const copyContent = async () => {
    const orderTab = prefs.tab === "orders";
    try {
      await navigator.clipboard.writeText(orderTab ? gameOrdersToText(orders, prefs.mode) : routeRowsToTsv(model.rows));
      setCopyStatus(orderTab ? "Ordres copiés." : "Tableau copié : collez-le dans un tableur.");
    } catch {
      setCopyStatus("Copie impossible : le navigateur refuse l'accès au presse-papiers.");
    }
    window.setTimeout(() => setCopyStatus(""), 4000);
  };

  return (
    <aside
      id="route-details-panel"
      className={["route-details-panel", collapsed ? "is-collapsed" : "", prefs.tab === "orders" ? "is-orders" : ""].filter(Boolean).join(" ")}
      role="dialog"
      aria-modal="false"
      aria-labelledby="route-details-title"
    >
      <div className="route-form-header route-details-header">
        <div>
          <h2 id="route-details-title">Route calculée</h2>
          <p className={model.destinationReached ? "route-details-status reached" : "route-details-status partial"}>
            {model.destinationReached ? "Destination atteinte" : "Destination non atteinte dans l'horizon : meilleure route affichée"}
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
          <button
            type="button"
            className="close-button"
            title={prefs.tab === "orders" ? "Copier les ordres" : "Copier le tableau"}
            aria-label={prefs.tab === "orders" ? "Copier les ordres" : "Copier le tableau"}
            onClick={() => void copyContent()}
          >
            <Icon name="content_copy" />
          </button>
          <button
            type="button"
            className="close-button"
            title={collapsed ? "Afficher les ordres et le détail" : "Réduire au résumé"}
            aria-label={collapsed ? "Afficher les ordres et le détail" : "Réduire au résumé"}
            aria-expanded={!collapsed}
            aria-controls="route-details-content"
            onClick={() => setCollapsed((value) => !value)}
          >
            {/* La flèche indique le sens où le panneau va s'étendre ou se replier :
                sur ordinateur il est en haut et s'ouvre vers le bas, sur téléphone
                il est réduit en bas de l'écran et s'ouvre vers le haut. */}
            <Icon name={collapsed !== narrow ? "expand_more" : "expand_less"} />
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
      <dl className="route-summary" aria-label="Résultat du calcul">
        <div>
          <dt>{model.destinationReached ? "Arrivée" : "Fin de l'horizon"}</dt>
          <dd>{summary.endInstant ? formatInstant(summary.endInstant) : "—"}</dd>
        </div>
        <div>
          <dt>Durée</dt>
          <dd>{formatSpan(summary.durationMinutes)}</dd>
        </div>
        <div>
          <dt>Distance</dt>
          <dd>{`${formatNumber(summary.distanceNM, summary.distanceNM < 100 ? 1 : 0)} NM`}</dd>
        </div>
        <div>
          <dt>Manœuvres</dt>
          <dd>{summary.maneuverCount}</dd>
        </div>
        <div>
          <dt>Stamina finale</dt>
          <dd>{summary.finalStamina === null ? "—" : `${formatNumber(summary.finalStamina, 0)} %`}</dd>
        </div>
      </dl>
      {copyStatus && (
        <p className="route-details-copy" role="status">
          {copyStatus}
        </p>
      )}
      <div id="route-details-content" hidden={collapsed}>
        <div className="route-tabs">
          <div role="tablist" aria-label="Contenu de la route" className="route-tablist">
            <button
              type="button"
              role="tab"
              id="route-tab-orders"
              aria-selected={prefs.tab === "orders"}
              aria-controls="route-tab-panel"
              className={prefs.tab === "orders" ? "route-tab is-active" : "route-tab"}
              onClick={() => setPrefs((value) => ({ ...value, tab: "orders" }))}
            >
              {`Ordres (${orders.length})`}
            </button>
            <button
              type="button"
              role="tab"
              id="route-tab-steps"
              aria-selected={prefs.tab === "steps"}
              aria-controls="route-tab-panel"
              className={prefs.tab === "steps" ? "route-tab is-active" : "route-tab"}
              onClick={() => setPrefs((value) => ({ ...value, tab: "steps" }))}
            >
              {`Pas à pas (${model.pointCount})`}
            </button>
          </div>
          {prefs.tab === "orders" && (
            <div className="segmented compact route-order-mode" role="radiogroup" aria-label="Réglage à donner">
              {(["twa", "heading"] as OrderMode[]).map((mode) => (
                <label key={mode} className={prefs.mode === mode ? "segment on" : "segment"}>
                  <input
                    type="radio"
                    name="order-mode"
                    value={mode}
                    checked={prefs.mode === mode}
                    onChange={() => setPrefs((value) => ({ ...value, mode }))}
                  />
                  <span className="segment-label">{mode === "twa" ? "TWA" : "Cap"}</span>
                </label>
              ))}
            </div>
          )}
        </div>
        <div
          id="route-tab-panel"
          role="tabpanel"
          aria-labelledby={prefs.tab === "orders" ? "route-tab-orders" : "route-tab-steps"}
          className="route-tab-panel"
        >
        {prefs.tab === "orders" ? (
          <ol className="route-orders" ref={body as React.RefObject<HTMLOListElement>}>
            {orders.map((order) => (
              <OrderItem
                key={order.index}
                order={order}
                mode={prefs.mode}
                selected={order.index === selectedIndex}
                onSelect={() => select(order.index, order.latitude, order.longitude)}
              />
            ))}
          </ol>
        ) : narrow ? (
          // Sur téléphone, une carte par étape : choisir une étape réduit le
          // panneau pour laisser voir sa position sur la carte.
          <ol className="route-cards" ref={body as React.RefObject<HTMLOListElement>}>
            {model.rows.map((row) => (
              <StepCard
                key={row.index}
                row={row}
                selected={row.index === selectedIndex}
                onSelect={() => select(row.index, row.latitude, row.longitude)}
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
        </div>
        <p className="route-report-legend">
          {prefs.tab === "orders"
            ? `Un ordre par manœuvre, changement de voile ou écart de ${prefs.mode === "twa" ? "TWA" : "cap"} d'au moins 5° tenu au moins 30 min. ${narrow ? "Touchez" : "Cliquez sur"} un ordre pour voir où le donner.`
            : narrow
              ? "Étape en jaune : manœuvre (virement, empannage ou changement de voile). Touchez une étape pour la voir sur la carte."
              : "Ligne surlignée en jaune : manœuvre (virement, empannage ou changement de voile) sur ce pas. Cliquez sur une ligne pour afficher sa position sur la route."}
          {prefs.tab === "steps" && !narrow && hidden.size > 0 && ` Colonnes sans variation masquées : ${[...hidden].map((key) => (key === "foils" ? "Foils" : "Coeff")).join(", ")}.`}
        </p>
      </div>
    </aside>
  );
}

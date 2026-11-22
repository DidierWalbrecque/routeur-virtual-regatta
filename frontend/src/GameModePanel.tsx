import { useEffect, useMemo, useRef, useState } from "react";
import { buildGameOrders, orderSetting, type GameOrder, type OrderMode } from "./lib/game_orders.ts";
import { formatCountdown, formatTimeUntil, maneuverCost, timeOrders, type TimedOrder } from "./lib/game_mode.ts";
import { buildRouteDetailsModel, formatInstant, formatNumber, maneuverLabel } from "./lib/route_details.ts";
import type { RouteResult } from "./lib/types.ts";

const MODE_STORAGE_KEY = "routeur.gameMode.orderMode";
const ALERT_STORAGE_KEY = "routeur.gameMode.alert";
/** Le rappel tombe cette durée avant chaque ordre. */
const ALERT_BEFORE_MS = 5 * 60_000;

function readStorage(key: string): string | null {
  try {
    return window.localStorage.getItem(key);
  } catch {
    return null;
  }
}

function writeStorage(key: string, value: string) {
  try {
    window.localStorage.setItem(key, value);
  } catch {
    // Stockage indisponible : le choix reste valable pour la session.
  }
}

function Icon({ name }: { name: string }) {
  return (
    <span className="material-symbols-outlined" aria-hidden="true">
      {name}
    </span>
  );
}

/** Ordre en une ligne, tel qu'on le recopie dans le jeu. */
function orderText(order: GameOrder, mode: OrderMode): string {
  return [
    formatInstant(order.instant).slice(-5),
    order.maneuver ? maneuverLabel(order.maneuver) : "",
    orderSetting(order, mode),
    order.sail === "—" ? "" : order.sail,
  ]
    .filter(Boolean)
    .join(" · ");
}

type Props = {
  result: RouteResult;
  /** « GFS 06Z du 22/11 » : run utilisé par la route. */
  runLabel: string;
  /** Le curseur de la frise suit l'heure actuelle. */
  followNow: boolean;
  onFollowNow: () => void;
  onSelect: (order: GameOrder) => void;
  onExit: () => void;
};

/**
 * Mode jeu : le prochain ordre à donner avec son compte à rebours, puis la
 * feuille de route complète, cochée au fil du temps.
 */
export function GameModePanel({ result, runLabel, followNow, onFollowNow, onSelect, onExit }: Props) {
  const [mode, setMode] = useState<OrderMode>(() => (readStorage(MODE_STORAGE_KEY) === "heading" ? "heading" : "twa"));
  const [alertOn, setAlertOn] = useState(() => readStorage(ALERT_STORAGE_KEY) === "true");
  const [now, setNow] = useState(() => Date.now());
  const [copyStatus, setCopyStatus] = useState("");
  const [reminder, setReminder] = useState("");
  const alerted = useRef(new Set<number>());

  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, []);
  useEffect(() => writeStorage(MODE_STORAGE_KEY, mode), [mode]);
  useEffect(() => writeStorage(ALERT_STORAGE_KEY, String(alertOn)), [alertOn]);

  const rows = useMemo(() => buildRouteDetailsModel(result).rows, [result]);
  const orders = useMemo(() => buildGameOrders(rows, mode), [rows, mode]);
  const timed = useMemo(() => timeOrders(orders, now), [orders, now]);
  const next = timed.find((item) => item.status === "next") ?? null;
  const current = timed.find((item) => item.status === "current") ?? null;

  // Rappel 5 min avant chaque ordre : notification du navigateur si elle est
  // permise, et bandeau dans le panneau dans tous les cas.
  useEffect(() => {
    if (!alertOn || !next) return;
    const remaining = next.start - now;
    if (remaining > ALERT_BEFORE_MS || remaining <= 0 || alerted.current.has(next.start)) return;
    alerted.current.add(next.start);
    const text = `Dans ${Math.ceil(remaining / 60_000)} min : ${orderText(next.order, mode)}`;
    setReminder(text);
    try {
      if ("Notification" in window && Notification.permission === "granted") new Notification("Routeur VR · ordre à donner", { body: text });
    } catch {
      // Notifications indisponibles : le bandeau suffit.
    }
    console.info(`[JEU] Rappel : ${text}`);
  }, [alertOn, next, now, mode]);

  const toggleAlert = async () => {
    const enable = !alertOn;
    if (enable) {
      try {
        if ("Notification" in window && Notification.permission === "default") await Notification.requestPermission();
      } catch {
        // Permission refusée ou indisponible : le bandeau du panneau reste.
      }
    }
    setAlertOn(enable);
  };

  const copy = async (order: GameOrder) => {
    try {
      await navigator.clipboard.writeText(orderText(order, mode));
      setCopyStatus("Ordre copié.");
    } catch {
      setCopyStatus("Copie impossible : le navigateur refuse l'accès au presse-papiers.");
    }
    window.setTimeout(() => setCopyStatus(""), 3000);
  };

  const focus = next ?? current;
  const cost = focus ? maneuverCost(rows, focus.order.index) : null;

  return (
    <section id="game-mode-panel" className="game-panel" aria-labelledby="game-mode-title">
      <header className="game-panel-head">
        <div>
          <h2 id="game-mode-title">Mode jeu</h2>
          <p>{runLabel ? `Route calculée sur GFS ${runLabel}` : "Route calculée"}</p>
        </div>
        <button type="button" className="game-exit" onClick={onExit}>
          <Icon name="arrow_back" />
          Retour au routage
        </button>
      </header>

      <div className="game-next">
        {next ? (
          <>
            <p className="game-label">Prochain ordre dans</p>
            <p className="game-countdown" role="timer" aria-live="off">
              {formatCountdown(next.start - now)}
            </p>
          </>
        ) : (
          <p className="game-label">{current ? "Dernier ordre : tenir jusqu'à la fin de la route" : "La route est terminée"}</p>
        )}
        {focus && (
          <div className="game-order-card">
            <div className="game-order-title">
              <strong>{formatInstant(focus.order.instant).slice(-5)}</strong>
              <span>{focus.order.maneuver ? maneuverLabel(focus.order.maneuver) : "Nouveau réglage"}</span>
            </div>
            <dl className="game-order-grid">
              <div>
                <dt>Régler</dt>
                <dd>{orderSetting(focus.order, mode)}</dd>
              </div>
              <div>
                <dt>Voile</dt>
                <dd>{focus.order.sail}</dd>
              </div>
              <div>
                <dt>Coût</dt>
                <dd>{cost === null ? "—" : `−${formatNumber(cost, 0)} stam.`}</dd>
              </div>
            </dl>
            <div className="game-order-actions">
              <button type="button" className="game-button-primary" onClick={() => void copy(focus.order)}>
                <Icon name="content_copy" />
                Copier l'ordre
              </button>
              <button type="button" className="game-button-outline" aria-pressed={alertOn} onClick={() => void toggleAlert()}>
                <Icon name={alertOn ? "notifications_active" : "notifications"} />
                {alertOn ? "Rappel 5 min avant : activé" : "Me prévenir 5 min avant"}
              </button>
            </div>
          </div>
        )}
        {copyStatus && (
          <p className="game-status" role="status">
            {copyStatus}
          </p>
        )}
        {reminder && (
          <p className="game-reminder" role="alert">
            <Icon name="notifications_active" />
            <span>{reminder}</span>
            <button type="button" aria-label="Fermer le rappel" onClick={() => setReminder("")}>
              <Icon name="close" />
            </button>
          </p>
        )}
      </div>

      <div className="game-sheet">
        <div className="game-sheet-head">
          <h3>Feuille de route</h3>
          {!followNow && (
            <button type="button" className="link-button" onClick={onFollowNow}>
              Revenir à l'heure actuelle
            </button>
          )}
          <div className="segmented compact" role="radiogroup" aria-label="Réglage à donner">
            {(["twa", "heading"] as OrderMode[]).map((value) => (
              <label key={value} className={mode === value ? "segment on" : "segment"}>
                <input type="radio" name="game-order-mode" value={value} checked={mode === value} onChange={() => setMode(value)} />
                <span className="segment-label">{value === "twa" ? "TWA" : "Cap"}</span>
              </label>
            ))}
          </div>
        </div>
        <ol className="game-orders">
          {timed.map((item) => (
            <GameOrderRow key={item.order.index} item={item} mode={mode} now={now} onSelect={() => onSelect(item.order)} />
          ))}
        </ol>
        <p className="game-note">
          Passé, en cours et à venir selon l'heure actuelle et la route calculée : le routeur ne sait pas si l'ordre a été donné dans le jeu.
        </p>
      </div>
    </section>
  );
}

function GameOrderRow({ item, mode, now, onSelect }: { item: TimedOrder; mode: OrderMode; now: number; onSelect: () => void }) {
  const { order, status } = item;
  const when =
    status === "past" ? "passé" : status === "current" ? "en cours" : formatTimeUntil(item.start - now);
  return (
    <li>
      <button type="button" className={`game-order is-${status}`} onClick={onSelect} title="Voir où donner cet ordre">
        <span className="game-order-mark" aria-hidden="true">
          {status === "past" ? <Icon name="check" /> : null}
        </span>
        <span className="game-order-time">{formatInstant(order.instant).slice(-5)}</span>
        <span className="game-order-text">
          {[order.maneuver ? maneuverLabel(order.maneuver) : "", orderSetting(order, mode), order.sail === "—" ? "" : order.sail]
            .filter(Boolean)
            .join(" · ")}
        </span>
        <span className="game-order-when">{when}</span>
      </button>
    </li>
  );
}

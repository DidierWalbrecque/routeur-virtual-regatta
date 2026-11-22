import { useEffect, useState } from "react";
import { formatGap, headingGap, routeGap } from "./lib/game_mode.ts";
import { formatNumber } from "./lib/route_details.ts";
import { legAt, stepIndexAt } from "./lib/timeline.ts";
import type { RoutePoint, VrState } from "./lib/types.ts";

/** Au-delà de cet écart, le bateau est noté hors de la route prévue. */
const OFF_ROUTE_NM = 2;
/** Même règle que la barre du haut : au-delà, la lecture n'est plus « en direct ». */
const LIVE_MAX_AGE_MS = 3 * 60_000;
/** Au-delà de cet écart de cap, le cap tenu est signalé. */
const HEADING_WARNING_DEGREES = 5;

type Props = {
  vrState: VrState | undefined;
  route: RoutePoint[];
  calculating: boolean;
  onRecalculate: () => void;
};

function age(updatedAt: number | undefined, now: number): string {
  if (!updatedAt) return "";
  const seconds = Math.max(0, Math.round((now - updatedAt) / 1000));
  if (seconds < 60) return `il y a ${seconds} s`;
  const minutes = Math.round(seconds / 60);
  return minutes < 60 ? `il y a ${minutes} min` : `il y a ${Math.round(minutes / 60)} h`;
}

/**
 * Bateau du jeu comparé à la route prévue : écart, avance ou retard, cap et
 * voile tenus face à ceux du plan à l'heure de la dernière lecture du jeu.
 */
export function LiveBoatCard({ vrState, route, calculating, onRecalculate }: Props) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 5000);
    return () => window.clearInterval(timer);
  }, []);

  const position = vrState?.preStart ? undefined : vrState?.position;
  const readAt = vrState?.updatedAt ?? now;
  const live = Boolean(position && vrState?.updatedAt && now - vrState.updatedAt <= LIVE_MAX_AGE_MS);
  const gap = position ? routeGap(route, position) : null;
  const planned = legAt(route, stepIndexAt(route, readAt));
  const delayMinutes = gap ? (readAt - gap.plannedTime) / 60_000 : null;
  const heading = vrState?.heading ?? null;
  const plannedHeading = planned?.heading ?? null;
  const headingOff =
    heading !== null && plannedHeading !== null && plannedHeading !== undefined && headingGap(heading, plannedHeading) >= HEADING_WARNING_DEGREES;
  const sail = vrState?.currentSail?.name ?? null;
  const plannedSail = planned?.sail ?? null;
  const sailOff = sail !== null && plannedSail !== null && sail !== plannedSail;

  return (
    <aside id="live-boat-card" className="live-card floating-card" aria-labelledby="live-card-title">
      <h2 id="live-card-title" className="live-card-title">
        <span className={live ? "cockpit-dot is-on" : "cockpit-dot"} aria-hidden="true" />
        {position ? "Bateau dans le jeu" : "Position du jeu inconnue"}
        {position && <span className="cockpit-muted">{` · lu ${age(vrState?.updatedAt, now)}`}</span>}
      </h2>
      {position && gap ? (
        <dl className="live-card-grid">
          <div>
            <dt>Écart à la route</dt>
            <dd className={gap.crossTrackNM >= OFF_ROUTE_NM ? "is-warning" : undefined}>{`${formatNumber(gap.crossTrackNM, 1)} NM`}</dd>
          </div>
          <div>
            <dt>{delayMinutes !== null && delayMinutes < 0 ? "Avance sur le plan" : "Retard sur le plan"}</dt>
            <dd>{delayMinutes === null ? "—" : Math.abs(delayMinutes) < 1 ? "à l'heure" : formatGap(delayMinutes)}</dd>
          </div>
          <div>
            <dt>Cap tenu / prévu</dt>
            <dd className={headingOff ? "is-warning" : undefined}>
              {`${heading === null ? "—" : `${Math.round(heading)}°`} / ${plannedHeading === null || plannedHeading === undefined ? "—" : `${Math.round(plannedHeading)}°`}`}
            </dd>
          </div>
          <div>
            <dt>Voile tenue / prévue</dt>
            <dd className={sailOff ? "is-warning" : undefined}>{`${sail ?? "—"} / ${plannedSail ?? "—"}`}</dd>
          </div>
        </dl>
      ) : (
        <p className="cockpit-muted">Ouvrez la course dans le jeu avec l'extension active pour comparer le bateau à la route.</p>
      )}
      <button type="button" className="game-button-outline" disabled={calculating || !position} onClick={onRecalculate}>
        <span className="material-symbols-outlined" aria-hidden="true">
          refresh
        </span>
        {calculating ? (
          "Calcul en cours…"
        ) : (
          <>
            <span className="live-label-long">Recalculer depuis la position actuelle</span>
            <span className="live-label-short">Recalculer d'ici</span>
          </>
        )}
      </button>
      <p className="live-card-legend">
        <span>
          <i className="live-legend-route" aria-hidden="true" /> Route prévue
        </span>
        <span>
          <i className="live-legend-track" aria-hidden="true" /> Trajet réel
        </span>
      </p>
    </aside>
  );
}

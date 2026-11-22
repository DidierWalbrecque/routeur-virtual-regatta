import { useEffect, useState } from "react";
import type { VrStatus } from "./RouteForm.tsx";
import type { NextRun } from "./WeatherRunCard.tsx";
import type { VrState } from "./lib/types.ts";
import { formatHourMinute, formatUtcHourMinute, nextRunText, type NextRunEta, type WeatherRunModel } from "./lib/weather_run.ts";

type Props = {
  boatName: string;
  vrState: VrState | undefined;
  vrStatus: VrStatus;
  weatherModel: WeatherRunModel | null;
  /** « 06Z du 22/11 ». */
  runLabel: string;
  nextRun: NextRun | null;
  /** Run suivant attendu et ses heures de publication. */
  nextRunEta: NextRunEta | null;
  /** Les prévisions ne couvrent pas l'horizon demandé. */
  weatherWarning: boolean;
  weatherOpen: boolean;
  onToggleWeather: () => void;
  gribDownloading: boolean;
  gribTooltip: string;
  refreshing: boolean;
  onRefreshWeather: () => void;
  calculating: boolean;
  /** Formulaire de calcul ouvert : le bouton l'ouvre et le referme. */
  formOpen: boolean;
  onCalculate: () => void;
};

/** Au-delà de cet âge, l'état du jeu n'est plus présenté comme « en direct ». */
const LIVE_MAX_AGE_MS = 3 * 60_000;

/** « il y a 12 s », « il y a 3 min » : fraîcheur de l'état reçu du jeu. */
function freshness(updatedAt: number | undefined, now: number): string {
  if (!updatedAt) return "";
  const seconds = Math.max(0, Math.round((now - updatedAt) / 1000));
  if (seconds < 60) return `il y a ${seconds} s`;
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `il y a ${minutes} min`;
  return `il y a ${Math.round(minutes / 60)} h`;
}

function Icon({ name }: { name: string }) {
  return (
    <span className="material-symbols-outlined" aria-hidden="true">
      {name}
    </span>
  );
}

/** Barre du haut du cockpit : bateau en direct, stamina, météo et calcul. */
export function CockpitBar(props: Props) {
  const { vrState, vrStatus, weatherModel, nextRun } = props;
  const eta = props.nextRunEta;
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 5_000);
    return () => window.clearInterval(timer);
  }, []);

  const stamina = vrState?.stamina;
  const staminaKnown = stamina !== null && stamina !== undefined && Number.isFinite(Number(stamina));
  const staminaValue = staminaKnown ? Math.max(0, Math.min(100, Number(stamina))) : 0;
  const age = freshness(vrState?.updatedAt, now);
  const live = vrStatus.ok && Boolean(vrState?.updatedAt) && now - Number(vrState?.updatedAt) <= LIVE_MAX_AGE_MS;
  const boatText = !vrStatus.ok
    ? "jeu non lu"
    : vrState?.preStart
      ? "départ non pris"
      : live
        ? `en direct ${age}`
        : age
          ? `lu ${age}`
          : "lu";
  const ahead = weatherModel ? Math.max(0, weatherModel.horizonHours - Math.max(0, weatherModel.ageHours)) : 0;

  return (
    <header className="cockpit-bar">
      <div className="cockpit-brand">
        <Icon name="sailing" />
        <span>Routeur VR</span>
      </div>

      <div className={live ? "cockpit-boat is-live" : "cockpit-boat"} title={vrStatus.title ? `${vrStatus.text}\n${vrStatus.title}` : vrStatus.text}>
        <span className="cockpit-dot" aria-hidden="true" />
        <strong>{props.boatName}</strong>
        <span className="cockpit-muted">{boatText}</span>
      </div>

      {staminaKnown && (
        <div className="cockpit-stamina" title={`Stamina ${staminaValue.toFixed(1)} %`}>
          <span className="cockpit-muted">Stamina</span>
          <span className="cockpit-gauge" role="meter" aria-label="Stamina" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(staminaValue)}>
            <span style={{ width: `${staminaValue}%` }} />
          </span>
          <span className="cockpit-number">{`${Math.round(staminaValue)} %`}</span>
        </div>
      )}

      <div className="cockpit-weather">
        <button
          id="weather-run-chip"
          type="button"
          className={props.weatherWarning ? "cockpit-chip is-warning" : "cockpit-chip"}
          aria-expanded={props.weatherOpen}
          aria-controls="weather-run-card"
          disabled={!weatherModel}
          title={[
            weatherModel ? `Afficher le détail des prévisions (${ahead} h devant vous)` : "Aucun run GRIB : actualisez les GRIB",
            nextRun ? `Run ${nextRun.label} en cours de téléchargement : ${nextRun.downloaded}/${nextRun.expected}` : "",
            !nextRun && eta
              ? `Prochain run ${eta.label} : premières échéances vers ${formatUtcHourMinute(eta.firstAt)} (${formatHourMinute(eta.firstAt)} heure locale), complet vers ${formatUtcHourMinute(eta.completeAt)}`
              : "",
          ]
            .filter(Boolean)
            .join("\n")}
          onClick={props.onToggleWeather}
        >
          <Icon name="cloud" />
          <span className="cockpit-muted">Météo</span>
          <span className="cockpit-number">{weatherModel ? `GFS ${props.runLabel.replace(" du ", " · ")}` : "aucun run"}</span>
          {nextRun ? (
            <span className="cockpit-next-run">{`${nextRun.label.slice(0, 3)} ${nextRun.downloaded}/${nextRun.expected}`}</span>
          ) : (
            eta && <span className="cockpit-next-run">{nextRunText(eta, now)}</span>
          )}
        </button>
        <button
          id="refresh-weather"
          type="button"
          className={props.gribDownloading ? "cockpit-icon-button is-busy" : "cockpit-icon-button"}
          aria-label="Actualiser les GRIB"
          title={props.gribTooltip}
          disabled={props.refreshing}
          onClick={props.onRefreshWeather}
        >
          <Icon name="cloud_download" />
        </button>
      </div>

      <div className="cockpit-spacer" />

      <button
        id="calculate-route"
        type="button"
        className={props.formOpen ? "cockpit-primary is-active" : "cockpit-primary"}
        aria-pressed={props.formOpen}
        aria-controls="route-form-panel"
        onClick={props.onCalculate}
      >
        <Icon name="alt_route" />
        <span>{props.calculating ? "Calcul en cours…" : "Calculer une route"}</span>
      </button>
    </header>
  );
}

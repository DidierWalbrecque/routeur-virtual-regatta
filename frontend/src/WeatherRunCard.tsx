import type { WeatherRunModel } from "./lib/weather_run.ts";

const HOURS_PER_DAY = 24;
// Au-delà de cet horizon la prévision GFS devient indicative (re-routage requis).
const LOW_CONFIDENCE_HOURS = 240;

/** Run suivant en cours de téléchargement. */
export type NextRun = { label: string; downloaded: number; expected: number };

function formatCoveredUntil(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "";
  const day = String(date.getUTCDate()).padStart(2, "0");
  const month = String(date.getUTCMonth() + 1).padStart(2, "0");
  const hour = String(date.getUTCHours()).padStart(2, "0");
  return `${day}/${month} ${hour}Z`;
}

// Timeline « prévision devant vous » : une règle 0 → horizon avec un curseur au
// présent. À gauche du curseur le passé (hachuré) depuis l'émission du run, à
// droite la prévision exploitable. La zone au-delà de LOW_CONFIDENCE_HOURS est
// hachurée en diagonale (prévision indicative), et tout passe en alerte quand la
// couverture ne couvre pas l'horizon de calcul demandé. Repliée, la carte se
// réduit à une ligne qui s'ouvre au clic.
export function WeatherRunCard({
  model,
  warning = false,
  nextRun = null,
  collapsed,
  onToggle,
}: {
  model: WeatherRunModel | null;
  warning?: boolean;
  nextRun?: NextRun | null;
  collapsed: boolean;
  onToggle: () => void;
}) {
  if (!model) return null;
  const horizon = model.horizonHours;
  const age = Math.min(Math.max(0, model.ageHours), horizon);
  const ahead = Math.max(0, horizon - age);
  const pastPercent = horizon > 0 ? (age / horizon) * 100 : 0;
  const coveredText = formatCoveredUntil(model.coveredUntil);
  const nextText = nextRun ? `${nextRun.label} en cours ${nextRun.downloaded}/${nextRun.expected}` : "";

  if (collapsed) {
    return (
      <button
        id="weather-run-card"
        type="button"
        className="weather-run-card weather-run-chip"
        aria-expanded="false"
        title="Afficher le détail des prévisions"
        onClick={onToggle}
      >
        <span className="material-symbols-outlined" aria-hidden="true">
          cloud
        </span>
        <span>
          <strong>{model.runText.slice(-3)}</strong>
          {" · "}
          <span className={warning ? "weather-run-chip-hours is-warning" : "weather-run-chip-hours"}>{`${ahead} h`}</span>
          {nextRun && <span className="weather-run-chip-next">{` · ${nextRun.label.slice(0, 3)} ${nextRun.downloaded}/${nextRun.expected}`}</span>}
        </span>
      </button>
    );
  }

  const ticks: number[] = [];
  for (let hour = HOURS_PER_DAY; hour < horizon; hour += HOURS_PER_DAY) {
    ticks.push((hour / horizon) * 100);
  }
  const showLowConfidence = horizon > LOW_CONFIDENCE_HOURS;
  const lowConfidencePercent = showLowConfidence ? (LOW_CONFIDENCE_HOURS / horizon) * 100 : 100;
  const labelOnLeft = pastPercent < 50;

  return (
    <aside id="weather-run-card" className="weather-run-card" aria-live="polite">
      <div className="weather-run-head">
        <p className="weather-run-title">
          <span className="weather-run-name">{model.title} · </span>
          <strong>{model.runText}</strong>
        </p>
        <span className="weather-run-age">{`émis il y a ${model.ageHours} h`}</span>
        <button type="button" className="weather-run-collapse" aria-label="Réduire l'encart météo" title="Réduire" onClick={onToggle}>
          <span className="material-symbols-outlined" aria-hidden="true">
            expand_more
          </span>
        </button>
      </div>
      <div className="weather-run-track-wrap">
        <div className="weather-run-ends">
          <span>+0h</span>
          <span>{`+${horizon}h`}</span>
        </div>
        <div
          className="weather-run-track"
          role="img"
          aria-label={`${ahead} h de prévision devant vous, run émis il y a ${model.ageHours} h${
            warning ? ", couverture insuffisante pour l’horizon demandé" : ""
          }`}
        >
          <span
            className={`weather-run-ahead-fill${warning ? " is-warning" : ""}`}
            style={{ left: `${pastPercent}%` }}
          />
          <span className="weather-run-past-fill" style={{ width: `${pastPercent}%` }} />
          {showLowConfidence && (
            <span className="weather-run-lowconf" style={{ left: `${lowConfidencePercent}%` }} />
          )}
          <span className="weather-run-ticks">
            {ticks.map((tick, index) => (
              <i key={index} style={{ left: `${tick}%` }} />
            ))}
          </span>
          <span className="weather-run-cursor" style={{ left: `${pastPercent}%` }}>
            <span className={`weather-run-cursor-label${labelOnLeft ? " is-left" : " is-right"}`}>
              maintenant
            </span>
          </span>
        </div>
      </div>
      <div className="weather-run-summary">
        <strong className={`weather-run-remaining${warning ? " is-warning" : ""}`}>{`${ahead} h`}</strong>
        <span className="weather-run-remaining-label">de prévision devant vous</span>
      </div>
      <p className="weather-run-footline">
        {`${model.fileCount} fichiers${coveredText ? ` · jusqu'au ${coveredText}` : ""}`}
      </p>
      {nextText && (
        <p className="weather-run-next" title="Le calcul complète ce run avec le précédent tant qu'il n'est pas entier">
          <span className="material-symbols-outlined" aria-hidden="true">
            downloading
          </span>
          {`Run ${nextText}`}
        </p>
      )}
      {showLowConfidence && (
        <p className="weather-run-legend">{`au-delà de J+${LOW_CONFIDENCE_HOURS / HOURS_PER_DAY} : indicatif`}</p>
      )}
    </aside>
  );
}

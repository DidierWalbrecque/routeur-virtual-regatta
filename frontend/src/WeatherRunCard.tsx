import type { WeatherRunModel } from "./lib/weather_run.ts";

export function WeatherRunCard({ model }: { model: WeatherRunModel | null }) {
  if (!model) return null;
  return (
    <aside id="weather-run-card" className="weather-run-card" aria-live="polite">
      <p className="weather-run-title">
        <span className="weather-run-name">{model.title} : </span>
        <strong>{`${model.runText} — +${model.horizonHours} h`}</strong>
      </p>
      <div
        className="weather-run-bar"
        role="img"
        aria-label={`Prévision de ${model.horizonHours} h, run émis il y a ${model.ageHours} h (${model.elapsedPercent} % de l’horizon écoulé)`}
      >
        <span className="weather-run-elapsed" style={{ width: `${model.elapsedPercent}%` }} />
        <span className="weather-run-ahead" />
      </div>
      <p className="weather-run-detail">{`Run émis il y a ${model.ageHours} h · ${model.fileCount} fichier(s)`}</p>
    </aside>
  );
}

import { BEAUFORT } from "./lib/beaufort.ts";

// Curseur d'heure du vent : jusqu'à 10 jours, au-delà la prévision GFS n'est qu'indicative.
const WIND_MAX_HOURS = 240;

const WIND_TIME_FORMAT = new Intl.DateTimeFormat("fr-FR", {
  weekday: "short",
  day: "2-digit",
  month: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
});

/** Bornes en nœuds d'une force, pour l'infobulle de l'échelle. */
function beaufortRange(force: number): string {
  const from = force === 0 ? 0 : BEAUFORT[force - 1].below;
  const to = BEAUFORT[force].below;
  return Number.isFinite(to) ? `${from} à ${to - 1} nœuds` : `${from} nœuds et plus`;
}

type Props = {
  /** Heure d'origine du curseur (ms) et décalage choisi, en heures. */
  base: number;
  hours: number;
  onHours: (hours: number) => void;
  /** Masque le vent. */
  onClose: () => void;
};

/** Réglages du fond de vent : heure affichée et échelle de Beaufort. */
export function WindCard({ base, hours, onHours, onClose }: Props) {
  return (
    <aside id="wind-card" className="wind-card floating-card" aria-labelledby="wind-card-title">
      <div className="map-legend-head">
        <h2 id="wind-card-title">Vent · échelle de Beaufort</h2>
        <button type="button" className="close-button" aria-label="Masquer le vent" title="Masquer le vent" onClick={onClose}>
          <span className="material-symbols-outlined" aria-hidden="true">
            close
          </span>
        </button>
      </div>
      <label className="wind-time">
        <span>
          {WIND_TIME_FORMAT.format(new Date(base + hours * 3_600_000))}
          <span className="wind-time-offset">{hours === 0 ? "maintenant" : `+${hours} h`}</span>
        </span>
        <input
          type="range"
          min={0}
          max={WIND_MAX_HOURS}
          step={1}
          value={hours}
          aria-label="Heure du vent affiché"
          onChange={(event) => onHours(Number(event.target.value))}
        />
      </label>
      <ol className="beaufort-scale" aria-label="Échelle de Beaufort">
        {BEAUFORT.map((level) => (
          <li key={level.force} style={{ background: level.color }} title={`Force ${level.force} · ${level.label} · ${beaufortRange(level.force)}`}>
            {level.force}
          </li>
        ))}
      </ol>
    </aside>
  );
}

import { PINS } from "./MapView.tsx";
import { gateRingSvg, type GateRounding } from "./lib/gate_marker.ts";
import { BEAUFORT } from "./lib/beaufort.ts";

// Curseur d'heure du vent : jusqu'à 10 jours, au-delà la prévision GFS n'est qu'indicative.
const WIND_MAX_HOURS = 240;

type Props = {
  showSeamarks: boolean;
  onSeamarks: (visible: boolean) => void;
  /** Vent coloré : affiché ou non, heure d'origine (ms) et décalage en heures. */
  wind: { visible: boolean; base: number; hours: number };
  onWindVisible: (visible: boolean) => void;
  onWindHours: (hours: number) => void;
  onClose: () => void;
};

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

/** Épingle de la carte en miniature, avec sa lettre ou son icône. */
function Pin({ pin }: { pin: { color: string; letter?: string; icon?: string } }) {
  return (
    <span className="legend-pin" aria-hidden="true">
      <svg viewBox="0 0 28 40" width="14" height="20">
        <path d="M14 0C6.3 0 0 6.2 0 13.9 0 24.3 14 40 14 40s14-15.7 14-26.1C28 6.2 21.7 0 14 0z" fill={pin.color} />
        <circle cx="14" cy="14" r="9" fill="#ffffff" />
      </svg>
      <span className={pin.icon ? "legend-pin-content material-symbols-outlined" : "legend-pin-content"} style={{ color: pin.color }}>
        {pin.icon ?? pin.letter}
      </span>
    </span>
  );
}

function Line({ color, dashed = false, width = 3 }: { color: string; dashed?: boolean; width?: number }) {
  return (
    <svg className="legend-symbol" viewBox="0 0 22 10" aria-hidden="true">
      <line x1="1" y1="5" x2="21" y2="5" stroke={color} strokeWidth={width} strokeDasharray={dashed ? "4 3" : undefined} strokeLinecap="round" />
    </svg>
  );
}

/**
 * Repère de porte en miniature, identique à celui de la carte (et animé de la
 * même façon), avec le numéro d'ordre au centre.
 */
function GateSymbol({ rounding }: { rounding: GateRounding }) {
  return (
    <span className={`legend-gate gate-marker is-${rounding}`} aria-hidden="true">
      {/* SVG produit par notre propre code (gate_marker.ts), sans donnée extérieure. */}
      <span className="gate-ring" dangerouslySetInnerHTML={{ __html: gateRingSvg(rounding) }} />
      <span className="gate-order">1</span>
    </span>
  );
}

/** Cercle pointillé du rayon d'arrivée. */
function RadiusCircle() {
  return (
    <svg className="legend-symbol" viewBox="0 0 16 16" aria-hidden="true">
      <circle cx="8" cy="8" r="6.5" fill="#c2255c" fillOpacity="0.15" stroke="#f06595" strokeWidth="1.5" strokeDasharray="2.5 2" />
    </svg>
  );
}

function Swatch({ color }: { color: string }) {
  return (
    <svg className="legend-symbol" viewBox="0 0 16 12" aria-hidden="true">
      <rect x="1" y="1" width="14" height="10" rx="2" fill={color} fillOpacity="0.35" stroke={color} strokeWidth="1.5" />
    </svg>
  );
}

/** Légende des repères et des couleurs de la carte. */
export function MapLegend({ showSeamarks, onSeamarks, wind, onWindVisible, onWindHours, onClose }: Props) {
  return (
    <aside id="map-legend" className="map-legend floating-card" aria-labelledby="map-legend-title">
      <div className="map-legend-head">
        <h2 id="map-legend-title">Légende</h2>
        <button type="button" className="close-button" aria-label="Fermer la légende" title="Fermer" onClick={onClose}>
          <span className="material-symbols-outlined" aria-hidden="true">
            close
          </span>
        </button>
      </div>
      <ul>
        <li>
          <Pin pin={PINS.departure} /> Départ du routage (bateau)
        </li>
        <li>
          <Pin pin={PINS.destination} /> Destination du routage
        </li>
        <li>
          <Pin pin={PINS.gameArrival} /> Arrivée du jeu
        </li>
        <li>
          <RadiusCircle /> Rayon d'arrivée du jeu
        </li>
        <li>
          <Pin pin={PINS.raceStart} /> Départ de la course
        </li>
        <li>
          <GateSymbol rounding="clockwise" /> Porte : marque à laisser à tribord, à contourner dans le sens de la flèche
        </li>
        <li>
          <GateSymbol rounding="counterclockwise" /> Porte : marque à laisser à bâbord
        </li>
        <li>
          <GateSymbol rounding="free" /> Porte à franchir dans les deux sens
        </li>
        <li className="map-legend-note">Le numéro au centre donne l'ordre de passage (1 = prochaine porte).</li>
        <li>
          <Line color="#f59f00" width={4} /> Route calculée
        </li>
        <li>
          <svg className="legend-symbol legend-gradient" viewBox="0 0 22 10" aria-hidden="true">
            <defs>
              {/* En coordonnées absolues : un dégradé relatif ne s'applique pas à une ligne sans hauteur. */}
              <linearGradient id="legend-isochrones" gradientUnits="userSpaceOnUse" x1="1" y1="0" x2="21" y2="0">
                <stop offset="0" stopColor="#4dabf7" />
                <stop offset="0.5" stopColor="#38d9a9" />
                <stop offset="1" stopColor="#ffd43b" />
              </linearGradient>
            </defs>
            <line x1="1" y1="5" x2="21" y2="5" stroke="url(#legend-isochrones)" strokeWidth="3" strokeLinecap="round" />
          </svg>
          Isochrones, des premières aux dernières
        </li>
        <li>
          <Swatch color="#ff6b6b" /> Zone interdite active
        </li>
        <li>
          <Line color="#f783ac" /> Trace GPX importée
        </li>
      </ul>
      <label className="switch map-legend-switch">
        <input type="checkbox" checked={showSeamarks} onChange={(event) => onSeamarks(event.target.checked)} />
        <span className="switch-track" aria-hidden="true" />
        Balisage OpenSeaMap (à partir du zoom 9)
      </label>
      <label className="switch map-legend-switch">
        <input type="checkbox" checked={wind.visible} onChange={(event) => onWindVisible(event.target.checked)} />
        <span className="switch-track" aria-hidden="true" />
        Vent (échelle de Beaufort)
      </label>
      {wind.visible && (
        <div className="wind-legend">
          <label className="wind-time">
            <span>
              {WIND_TIME_FORMAT.format(new Date(wind.base + wind.hours * 3_600_000))}
              <span className="wind-time-offset">{wind.hours === 0 ? "maintenant" : `+${wind.hours} h`}</span>
            </span>
            <input
              type="range"
              min={0}
              max={WIND_MAX_HOURS}
              step={1}
              value={wind.hours}
              aria-label="Heure du vent affiché"
              onChange={(event) => onWindHours(Number(event.target.value))}
            />
          </label>
          <ol className="beaufort-scale" aria-label="Échelle de Beaufort">
            {BEAUFORT.map((level) => (
              <li key={level.force} style={{ background: level.color }} title={`Force ${level.force} · ${level.label} · ${beaufortRange(level.force)}`}>
                {level.force}
              </li>
            ))}
          </ol>
        </div>
      )}
    </aside>
  );
}

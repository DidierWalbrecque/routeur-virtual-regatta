import { PINS } from "./MapView.tsx";

type Props = {
  showSeamarks: boolean;
  onSeamarks: (visible: boolean) => void;
  onClose: () => void;
};

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

/** Numéro d'ordre d'une porte, tel qu'il apparaît au milieu de la porte. */
function OrderBadge() {
  return (
    <svg className="legend-symbol" viewBox="0 0 16 16" aria-hidden="true">
      <circle cx="8" cy="8" r="6.5" fill="#1d2433" stroke="#51cf66" strokeWidth="2" />
      <text x="8" y="11" textAnchor="middle" fontSize="8.5" fontWeight="700" fill="#ffffff" fontFamily="system-ui, sans-serif">
        1
      </text>
    </svg>
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
export function MapLegend({ showSeamarks, onSeamarks, onClose }: Props) {
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
          <Line color="#51cf66" /> Porte à laisser à tribord
        </li>
        <li>
          <Line color="#ff922b" /> Porte à laisser à bâbord
        </li>
        <li>
          <Line color="#9775fa" dashed /> Porte à passer dans les deux sens
        </li>
        <li>
          <OrderBadge /> Ordre de passage (1 = prochaine porte)
        </li>
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
    </aside>
  );
}

type Props = {
  showSeamarks: boolean;
  onSeamarks: (visible: boolean) => void;
  onClose: () => void;
};

function Pin({ color }: { color: string }) {
  return (
    <svg className="legend-symbol" viewBox="0 0 16 22" aria-hidden="true">
      <path d="M8 0C3.6 0 0 3.6 0 8c0 6 8 14 8 14s8-8 8-14c0-4.4-3.6-8-8-8z" fill={color} />
      <circle cx="8" cy="8" r="3" fill="#ffffff" />
    </svg>
  );
}

function Line({ color, dashed = false, width = 3 }: { color: string; dashed?: boolean; width?: number }) {
  return (
    <svg className="legend-symbol" viewBox="0 0 22 10" aria-hidden="true">
      <line x1="1" y1="5" x2="21" y2="5" stroke={color} strokeWidth={width} strokeDasharray={dashed ? "4 3" : undefined} strokeLinecap="round" />
    </svg>
  );
}

function Mark({ color, letter }: { color: string; letter: string }) {
  return (
    <svg className="legend-symbol" viewBox="0 0 16 16" aria-hidden="true">
      <circle cx="8" cy="8" r="6.5" fill={color} stroke="#ffffff" strokeWidth="1.5" />
      <text x="8" y="11" textAnchor="middle" fontSize="8" fill="#ffffff" fontFamily="system-ui, sans-serif">
        {letter}
      </text>
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
          <Pin color="#1971c2" /> Départ du routage (bateau)
        </li>
        <li>
          <Pin color="#6741d9" /> Destination du routage
        </li>
        <li>
          <Pin color="#c2255c" /> Arrivée du jeu
        </li>
        <li>
          <Pin color="#f59f00" /> Départ de la course
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
          <Mark color="#51cf66" letter="D" /> <Mark color="#ff6b6b" letter="A" /> Extrémités d'une porte
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

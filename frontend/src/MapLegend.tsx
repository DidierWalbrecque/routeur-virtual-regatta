import { useEffect, useRef } from "react";
import { PINS } from "./MapView.tsx";
import { gateRingSvg, type GateRounding } from "./lib/gate_marker.ts";
import { BEAUFORT } from "./lib/beaufort.ts";
import { barbShape } from "./lib/wind_barbs.ts";

/** Bornes en nœuds d'une force, pour l'infobulle de l'échelle. */
function beaufortRange(force: number): string {
  const from = force === 0 ? 0 : BEAUFORT[force - 1].below;
  const to = BEAUFORT[force].below;
  return Number.isFinite(to) ? `${from} à ${to - 1} nœuds` : `${from} nœuds et plus`;
}

type Props = {
  onClose: () => void;
  /** Bateau dans l'hémisphère sud : les traits des barbules changent de côté. */
  south: boolean;
  /** Augmente quand les barbules s'affichent : la légende défile jusqu'à leur lecture. */
  focusBarbs: number;
};

/** Exemples de barbules, du calme à la tempête. */
const BARB_EXAMPLES = [
  { knots: 0, label: "calme" },
  { knots: 5, label: "5 kn" },
  { knots: 10, label: "10 kn" },
  { knots: 25, label: "25 kn" },
  { knots: 50, label: "50 kn" },
  { knots: 65, label: "65 kn" },
];

/** Barbule de la carte en miniature : vent d'ouest, hampe vers la gauche. */
function Barb({ knots, south }: { knots: number; south: boolean }) {
  const shape = barbShape(knots, 26, 14, 22, south);
  return (
    <svg className="legend-barb" viewBox="0 0 32 28" aria-hidden="true">
      <g transform="rotate(270 26 14)" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" fill="currentColor">
        {shape.calm && <circle cx="26" cy="14" r="5" fill="none" />}
        {shape.lines.map(([x1, y1, x2, y2], index) => (
          <line key={index} x1={x1} y1={y1} x2={x2} y2={y2} />
        ))}
        {shape.pennants.map((points, index) => (
          <polygon key={index} points={points.join(" ")} strokeLinejoin="round" />
        ))}
      </g>
    </svg>
  );
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
export function MapLegend({ onClose, south, focusBarbs }: Props) {
  const legend = useRef<HTMLElement>(null);
  const barbsTitle = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    if (!focusBarbs || !legend.current || !barbsTitle.current) return;
    legend.current.scrollTop = barbsTitle.current.offsetTop - legend.current.offsetTop - 8;
  }, [focusBarbs]);
  return (
    <aside ref={legend} id="map-legend" className="map-legend floating-card" aria-labelledby="map-legend-title">
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
        <li>
          <span className="legend-boat" aria-hidden="true" /> Bateau à l'heure de la frise, et son isochrone en blanc
        </li>
      </ul>
      <h2 id="map-legend-wind">Vent · échelle de Beaufort</h2>
      <ol className="beaufort-scale" aria-labelledby="map-legend-wind">
        {BEAUFORT.map((level) => (
          <li key={level.force} style={{ background: level.color }} title={`Force ${level.force} · ${level.label} · ${beaufortRange(level.force)}`}>
            {level.force}
          </li>
        ))}
      </ol>
      <h2 ref={barbsTitle} id="map-legend-barbs">Barbules</h2>
      <ol className="barb-key" aria-labelledby="map-legend-barbs">
        {BARB_EXAMPLES.map((example) => (
          <li key={example.knots}>
            <Barb knots={example.knots} south={south} />
            <span>{example.label}</span>
          </li>
        ))}
      </ol>
      <p className="map-legend-note barb-note">
        La hampe pointe d'où vient le vent : ici, vent d'ouest. Demi-trait 5 kn, trait 10 kn, fanion 50 kn ; on les additionne.
        {south ? " Dans l'hémisphère sud, les traits sont sous la hampe." : " Dans l'hémisphère nord, les traits sont au-dessus de la hampe."}
      </p>
    </aside>
  );
}

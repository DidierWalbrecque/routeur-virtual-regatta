import { useMemo } from "react";
import { approximateDistanceNM } from "./lib/geo.ts";
import type { BoatHint, Point } from "./lib/types.ts";
import type { Zone } from "./lib/zones.ts";

// Au-delà de cette distance du bateau, une zone de la course est grisée : elle
// ne gêne pas le calcul en cours.
const FAR_ZONE_NM = 300;

type Props = {
  zones: Zone[];
  /** Nombre de zones reçues du jeu, et structure de la manche pour le diagnostic. */
  game: { stateKnown: boolean; count: number; legShape: BoatHint[] };
  /** Position du bateau, pour trier les zones de la plus proche à la plus lointaine. */
  boat: Point | null;
  /** Zone mise en évidence sur la carte. */
  focusedId: string | null;
  onDraw: () => void;
  onToggle: (zone: Zone, enabled: boolean) => void;
  onToggleAll: (zones: Zone[], enabled: boolean) => void;
  onFocus: (zone: Zone) => void;
  onRename: (id: string, name: string) => void;
  onDelete: (id: string) => void;
  onClose: () => void;
};

/** Distance du bateau au sommet le plus proche de la zone, en milles. */
function zoneDistance(zone: Zone, boat: Point | null): number | null {
  if (!boat || zone.points.length === 0) return null;
  return Math.min(...zone.points.map((point) => approximateDistanceNM(boat, point)));
}

function formatDistance(distance: number | null): string {
  if (distance === null) return "";
  if (distance < 1) return "< 1 NM";
  return `${Math.round(distance).toLocaleString("fr-FR")} NM`;
}

function FocusButton({ zone, focused, onFocus }: { zone: Zone; focused: boolean; onFocus: (zone: Zone) => void }) {
  return (
    <button
      type="button"
      className={focused ? "close-button zone-focus is-active" : "close-button zone-focus"}
      title="Afficher sur la carte"
      aria-label={`Afficher ${zone.name} sur la carte`}
      aria-pressed={focused}
      onClick={() => onFocus(zone)}
    >
      <span className="material-symbols-outlined" aria-hidden="true">
        center_focus_strong
      </span>
    </button>
  );
}

export function ZonesPanel(props: Props) {
  const { zones, game, boat, focusedId, onDraw, onToggle, onToggleAll, onFocus, onRename, onDelete, onClose } = props;
  const fromGame = useMemo(
    () =>
      zones
        .filter((zone) => zone.source === "game")
        .map((zone) => ({ zone, distance: zoneDistance(zone, boat) }))
        .sort((a, b) => (a.distance ?? Infinity) - (b.distance ?? Infinity)),
    [zones, boat],
  );
  const drawn = zones.filter((zone) => zone.source !== "game");
  const allGameEnabled = fromGame.every(({ zone }) => zone.enabled);

  return (
    <aside id="zones-panel" className="zones-panel" aria-labelledby="zones-title">
      <div className="route-form-header">
        <div>
          <h2 id="zones-title">Zones interdites</h2>
          <p className="zones-help">La route ne traversera aucune zone cochée.</p>
        </div>
        <button className="close-button" type="button" title="Fermer" aria-label="Fermer" onClick={onClose}>
          <span className="material-symbols-outlined" aria-hidden="true">
            close
          </span>
        </button>
      </div>

      <div className="zones-body">
        <div className="zones-group-head">
          <h3 className="zones-group">Zones de la course</h3>
          {fromGame.length > 1 && (
            <button
              type="button"
              className="zones-link"
              onClick={() => onToggleAll(fromGame.map(({ zone }) => zone), !allGameEnabled)}
            >
              {allGameEnabled ? "Tout décocher" : "Tout cocher"}
            </button>
          )}
        </div>
        {fromGame.length > 0 ? (
          <ul className="zones-list">
            {fromGame.map(({ zone, distance }) => {
              const far = distance !== null && distance > FAR_ZONE_NM;
              const classes = ["zone-row", zone.enabled ? "" : "off", far ? "far" : ""].filter(Boolean).join(" ");
              return (
                <li key={zone.id} className={classes}>
                  <input
                    type="checkbox"
                    aria-label={`Activer ${zone.name}`}
                    checked={zone.enabled}
                    onChange={(event) => onToggle(zone, event.target.checked)}
                  />
                  <button type="button" className="zone-label" title={`${zone.name} : afficher sur la carte`} onClick={() => onFocus(zone)}>
                    {zone.name}
                  </button>
                  <span className="zone-distance" title={far ? "Loin du bateau : sans effet sur un calcul court" : "Distance au bateau"}>
                    {formatDistance(distance)}
                  </span>
                  <FocusButton zone={zone} focused={zone.id === focusedId} onFocus={onFocus} />
                </li>
              );
            })}
          </ul>
        ) : (
          <>
            <p className="zones-empty">
              {game.stateKnown
                ? "Aucune zone reçue de Virtual Regatta pour cette course."
                : "En attente de l'état Virtual Regatta…"}
            </p>
            {game.stateKnown && game.legShape.length > 0 && (
              <details className="zones-diagnostic">
                <summary>Champs reçus de la manche</summary>
                <ul>
                  {game.legShape.map((entry) => (
                    <li key={entry.path}>
                      {entry.path} : {entry.value}
                    </li>
                  ))}
                </ul>
              </details>
            )}
          </>
        )}

        <h3 className="zones-group">Zones dessinées</h3>
        {drawn.length === 0 ? (
          <p className="zones-empty">Aucune zone. Tracez un polygone que la route ne doit pas traverser.</p>
        ) : (
          <ul className="zones-list">
            {drawn.map((zone) => (
              <li key={zone.id} className={zone.enabled ? "zone-row" : "zone-row off"}>
                <input
                  type="checkbox"
                  aria-label={`Activer ${zone.name}`}
                  checked={zone.enabled}
                  onChange={(event) => onToggle(zone, event.target.checked)}
                />
                <input
                  className="zone-name"
                  aria-label="Nom de la zone"
                  value={zone.name}
                  maxLength={40}
                  onChange={(event) => onRename(zone.id, event.target.value)}
                />
                <FocusButton zone={zone} focused={zone.id === focusedId} onFocus={onFocus} />
                <button className="close-button" type="button" title="Supprimer la zone" aria-label={`Supprimer ${zone.name}`} onClick={() => onDelete(zone.id)}>
                  <span className="material-symbols-outlined" aria-hidden="true">
                    delete
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
      <footer className="route-form-footer">
        <button id="draw-zone" className="form-submit" type="button" onClick={onDraw}>
          <span className="material-symbols-outlined" aria-hidden="true">
            draw
          </span>
          Dessiner une zone
        </button>
      </footer>
    </aside>
  );
}

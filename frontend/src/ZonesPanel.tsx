import type { BoatHint } from "./lib/types.ts";
import type { Zone } from "./lib/zones.ts";

type Props = {
  zones: Zone[];
  /** Nombre de zones reçues du jeu, et structure de la manche pour le diagnostic. */
  game: { stateKnown: boolean; count: number; legShape: BoatHint[] };
  onDraw: () => void;
  onToggle: (zone: Zone, enabled: boolean) => void;
  onRename: (id: string, name: string) => void;
  onDelete: (id: string) => void;
  onClose: () => void;
};

export function ZonesPanel({ zones, game, onDraw, onToggle, onRename, onDelete, onClose }: Props) {
  const fromGame = zones.filter((zone) => zone.source === "game");
  const drawn = zones.filter((zone) => zone.source !== "game");

  return (
    <aside id="zones-panel" className="zones-panel" aria-labelledby="zones-title">
      <div className="route-form-header">
        <h2 id="zones-title">Zones interdites</h2>
        <button className="close-button" type="button" title="Fermer" aria-label="Fermer" onClick={onClose}>
          ×
        </button>
      </div>
      <p className="zones-help">La route ne traversera aucune zone cochée.</p>

      <h3 className="zones-group">Zones de la course</h3>
      {fromGame.length > 0 ? (
        <ul className="zones-list">
          {fromGame.map((zone) => (
            <li key={zone.id} className={zone.enabled ? "zone-row" : "zone-row off"}>
              <input
                type="checkbox"
                aria-label={`Activer ${zone.name}`}
                checked={zone.enabled}
                onChange={(event) => onToggle(zone, event.target.checked)}
              />
              <span className="zone-label" title={zone.name}>
                {zone.name}
              </span>
              <span className="zone-points">{zone.points.length} pts</span>
            </li>
          ))}
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
              <span className="zone-points">{zone.points.length} pts</span>
              <button className="close-button" type="button" title="Supprimer la zone" aria-label={`Supprimer ${zone.name}`} onClick={() => onDelete(zone.id)}>
                ×
              </button>
            </li>
          ))}
        </ul>
      )}
      <button id="draw-zone" className="form-submit" type="button" onClick={onDraw}>
        Dessiner une zone
      </button>
    </aside>
  );
}

import type { FormEvent } from "react";
import { horizonOptionsFor, type HorizonChoice } from "./lib/horizon.ts";
import type { Checkpoint, PolarOption } from "./lib/types.ts";
import {
  EQUIPMENT,
  EQUIPMENT_LABELS,
  type Equipment,
  checkpointKey,
  checkpointLabel,
  checkpointTitle,
} from "./lib/vr.ts";

export type FormState = {
  start: string;
  positionSource: "vr" | "manual";
  staminaSource: "vr" | "manual";
  staminaManual: string;
  useCourseGates: boolean;
  coastalMarginNM: string;
  destination: string;
  instant: string;
  polar: string;
  horizon: HorizonChoice;
  step: string;
  mode: "quick" | "precise";
  equipment: Set<Equipment>;
  checkpoint: string;
};

export type VrStatus = { ok: boolean; text: string; title?: string };

type Props = {
  form: FormState;
  polars: PolarOption[];
  polarNote: { detected: boolean; text: string; title?: string };
  horizonNote: { warning: boolean; text: string };
  checkpoints: Checkpoint[];
  /** Nom de l'arrivée officielle du jeu, si connue : proposée dans la liste d'arrivée. */
  gameArrivalName?: string | null;
  checkpointCache: Map<string, { start: Checkpoint["start"]; end: Checkpoint["end"] }>;
  vrStatus: VrStatus;
  weatherStatus: string;
  error: string;
  submitting: boolean;
  onField: <K extends keyof FormState>(field: K, value: FormState[K]) => void;
  onStart: (value: string) => void;
  onDestination: (value: string) => void;
  onCheckpoint: (key: string) => void;
  onToggleEquipment: (item: Equipment, checked: boolean) => void;
  onSubmit: () => void;
  onClose: () => void;
  startInput: React.RefObject<HTMLInputElement | null>;
};

export function RouteForm(props: Props) {
  const { form, polars, checkpoints, checkpointCache } = props;
  const submit = (event: FormEvent) => {
    event.preventDefault();
    props.onSubmit();
  };

  return (
    <aside id="route-form-panel" className="route-form-panel">
      <div className="route-form-header">
        <h2>Calculer une route</h2>
        <button id="close-route-form" className="close-button" type="button" title="Fermer" aria-label="Fermer" onClick={props.onClose}>
          ×
        </button>
      </div>
      <form id="route-form" onSubmit={submit}>
        <div id="route-vr-state-status" className="vr-status" title={props.vrStatus.title} role="status">
          <span className={props.vrStatus.ok ? "vr-dot ok" : "vr-dot"} aria-hidden="true" />
          <span>{props.vrStatus.text}</span>
        </div>

        <fieldset className="form-section">
          <legend>Trajet</legend>
          <label>
            Départ
            <input id="route-start" ref={props.startInput} required value={form.start} onChange={(event) => props.onStart(event.target.value)} />
          </label>
          <label>
            Source de la position
            <select
              id="route-position-source"
              value={form.positionSource}
              onChange={(event) => props.onField("positionSource", event.target.value as FormState["positionSource"])}
            >
              <option value="vr">Virtual Regatta (live)</option>
              <option value="manual">Saisie manuelle</option>
            </select>
          </label>
          <label>
            Arrivée
            <input id="route-destination" required value={form.destination} onChange={(event) => props.onDestination(event.target.value)} />
          </label>
          {(checkpoints.length > 0 || props.gameArrivalName) && (
            <select id="route-checkpoints-list" aria-label="Portes restantes" value={form.checkpoint} onChange={(event) => props.onCheckpoint(event.target.value)}>
              <option value="" disabled>
                Ou choisir une porte restante…
              </option>
              {props.gameArrivalName && (
                <option value="game-arrival">Arrivée du jeu — {props.gameArrivalName}</option>
              )}
              {checkpoints.map((checkpoint, index) => {
                const key = checkpointKey(checkpoint, index);
                const cached = checkpointCache.get(key);
                return (
                  <option key={key} value={key} title={checkpointTitle({ ...checkpoint, ...(cached || {}) })}>
                    {checkpointLabel(checkpoint, index)}
                  </option>
                );
              })}
            </select>
          )}
        </fieldset>

        <fieldset className="form-section">
          <legend>Simulation</legend>
          <label>
            Stamina
            <select
              id="route-stamina-source"
              value={form.staminaSource}
              onChange={(event) => props.onField("staminaSource", event.target.value as FormState["staminaSource"])}
            >
              <option value="vr">Virtual Regatta (live)</option>
              <option value="manual">Valeur manuelle</option>
            </select>
          </label>
          {form.staminaSource === "manual" && (
            <label>
              Stamina manuelle (%)
              <input
                id="route-stamina-manual"
                type="number"
                min="0"
                max="100"
                step="0.5"
                value={form.staminaManual}
                onChange={(event) => props.onField("staminaManual", event.target.value)}
              />
            </label>
          )}
          <label className="form-check">
            <input
              id="route-use-course-gates"
              type="checkbox"
              checked={form.useCourseGates}
              onChange={(event) => props.onField("useCourseGates", event.target.checked)}
            />
            Portes de course (Virtual Regatta)
          </label>
        </fieldset>

        <fieldset className="form-section">
          <legend>Calcul</legend>
          <label>
            Départ (heure locale, JJ/MM/AAAA HH:MM)
            <input
              id="route-instant"
              required
              value={form.instant}
              pattern="\d{2}/\d{2}/\d{4} \d{2}:\d{2}"
              onChange={(event) => props.onField("instant", event.target.value)}
            />
          </label>
          <label>
            Horizon du routage
            <select id="route-horizon" value={form.horizon} onChange={(event) => props.onField("horizon", event.target.value as HorizonChoice)}>
              {horizonOptionsFor(form.mode).map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
            <span className={props.horizonNote.warning ? "polar-note warning" : "polar-note"}>{props.horizonNote.text}</span>
          </label>
          <div className="form-grid">
            <label>
              Pas (min)
              <input id="route-step" type="number" min="1" required value={form.step} onChange={(event) => props.onField("step", event.target.value)} />
            </label>
            <label>
              Mode
              <select id="route-mode" value={form.mode} onChange={(event) => props.onField("mode", event.target.value as FormState["mode"])}>
                <option value="quick">Rapide</option>
                <option value="precise">Précis</option>
              </select>
            </label>
          </div>
          <label>
            Marge à la côte (NM)
            <input id="route-coastal-margin" type="number" min="0" max="10" step="0.5" required value={form.coastalMarginNM} onChange={(event) => props.onField("coastalMarginNM", event.target.value)} />
          </label>
          <label>
            Polaire
            <select id="route-polar" required value={form.polar} onChange={(event) => props.onField("polar", event.target.value)}>
              {polars.map((polar) => (
                <option key={polar.value} value={polar.value}>
                  {polar.label}
                </option>
              ))}
            </select>
            <span className={props.polarNote.detected ? "polar-note detected" : "polar-note"} title={props.polarNote.title}>
              {props.polarNote.text}
            </span>
          </label>
        </fieldset>

        <fieldset className="form-section">
          <legend>Équipements possédés</legend>
          <div className="chips">
            {EQUIPMENT.map((item) => (
              <label className={form.equipment.has(item) ? "chip on" : "chip"} key={item}>
                <input type="checkbox" value={item} checked={form.equipment.has(item)} onChange={(event) => props.onToggleEquipment(item, event.target.checked)} />
                {EQUIPMENT_LABELS[item]}
              </label>
            ))}
          </div>
        </fieldset>

        <div className="route-form-footer">
          <p id="route-form-error" className="form-error" role="alert">
            {props.error}
          </p>
          <button id="submit-route" className="form-submit" type="submit" disabled={props.submitting}>
            Calculer la route
          </button>
          <p id="route-weather-status" className="form-hint">
            {props.weatherStatus}
          </p>
        </div>
      </form>
    </aside>
  );
}

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
  useVrState: boolean;
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
          <label className="form-check vr-use">
            <input id="route-use-vr-state" type="checkbox" checked={form.useVrState} onChange={(event) => props.onField("useVrState", event.target.checked)} />
            Utiliser la position, la stamina et les options reçues de Virtual Regatta
          </label>
          <label>
            Arrivée
            <input id="route-destination" required value={form.destination} onChange={(event) => props.onDestination(event.target.value)} />
          </label>
          {checkpoints.length > 0 && (
            <select id="route-checkpoints-list" aria-label="Portes restantes" value={form.checkpoint} onChange={(event) => props.onCheckpoint(event.target.value)}>
              <option value="" disabled>
                Ou choisir une porte restante…
              </option>
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

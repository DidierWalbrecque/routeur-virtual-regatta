import type { FormEvent, ReactNode } from "react";
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

/** « ultim_bp_11.json » → « Ultim BP 11 ». */
export function polarDisplayName(label: string): string {
  return label
    .replace(/\.json$/i, "")
    .split(/[_\s-]+/)
    .filter(Boolean)
    .map((word) => (/^[a-z]{1,2}$/i.test(word) ? word.toUpperCase() : word.charAt(0).toUpperCase() + word.slice(1)))
    .join(" ");
}

type SegmentOption<T extends string> = { value: T; label: string; hint?: string };

/** Choix exclusif présenté comme un bouton à plusieurs positions. */
function Segmented<T extends string>(props: {
  name: string;
  label: string;
  value: T;
  options: SegmentOption<T>[];
  onChange: (value: T) => void;
  id?: string;
  compact?: boolean;
}) {
  return (
    <div
      id={props.id}
      className={props.compact ? "segmented compact" : "segmented"}
      role="radiogroup"
      aria-label={props.label}
    >
      {props.options.map((option) => (
        <label key={option.value} className={option.value === props.value ? "segment on" : "segment"}>
          <input
            type="radio"
            name={props.name}
            value={option.value}
            checked={option.value === props.value}
            onChange={() => props.onChange(option.value)}
          />
          <span className="segment-label">{option.label}</span>
          {option.hint && <span className="segment-hint">{option.hint}</span>}
        </label>
      ))}
    </div>
  );
}

/** Section repliable, avec un résumé lisible quand elle est fermée. */
function Collapsible(props: { title: string; summary: string; open?: boolean; children: ReactNode }) {
  return (
    <details className="form-collapsible" open={props.open}>
      <summary>
        <span className="form-collapsible-title">{props.title}</span>
        <span className="form-collapsible-summary">{props.summary}</span>
        <span className="material-symbols-outlined form-collapsible-chevron" aria-hidden="true">
          expand_more
        </span>
      </summary>
      <div className="form-collapsible-body">{props.children}</div>
    </details>
  );
}

export function RouteForm(props: Props) {
  const { form, polars, checkpoints, checkpointCache } = props;
  const submit = (event: FormEvent) => {
    event.preventDefault();
    props.onSubmit();
  };

  const polarLabel = polars.find((polar) => polar.value === form.polar)?.label ?? form.polar.split(/[\\/]/).pop() ?? "";
  const ownedCount = EQUIPMENT.filter((item) => form.equipment.has(item)).length;
  const boatSummary = `${polarDisplayName(polarLabel)} · ${ownedCount}/${EQUIPMENT.length} options`;
  const advancedSummary = [
    `pas ${form.step} min`,
    `côte ${form.coastalMarginNM} NM`,
    form.staminaSource === "vr" ? "stamina du jeu" : `stamina ${form.staminaManual} %`,
    form.useCourseGates ? "portes" : "sans portes",
  ].join(" · ");

  return (
    <aside id="route-form-panel" className="route-form-panel" aria-labelledby="route-form-title">
      <header className="route-form-header">
        <div>
          <h2 id="route-form-title">Calculer une route</h2>
          <div id="route-vr-state-status" className="vr-status" title={props.vrStatus.title} role="status">
            <span className={props.vrStatus.ok ? "vr-dot ok" : "vr-dot"} aria-hidden="true" />
            <span>{props.vrStatus.text}</span>
          </div>
        </div>
        <button id="close-route-form" className="close-button" type="button" title="Fermer" aria-label="Fermer" onClick={props.onClose}>
          <span className="material-symbols-outlined" aria-hidden="true">
            close
          </span>
        </button>
      </header>

      <form id="route-form" onSubmit={submit}>
        <div className="route-form-body">
          <section className="form-section" aria-labelledby="route-section-trip">
            <h3 id="route-section-trip">Trajet</h3>

            <div className="field">
              <div className="field-head">
                <label htmlFor="route-start">
                  <span className="field-dot departure" aria-hidden="true" />
                  Départ
                </label>
                <Segmented
                  id="route-position-source"
                  name="position-source"
                  label="Source de la position"
                  compact
                  value={form.positionSource}
                  options={[
                    { value: "vr", label: "Jeu" },
                    { value: "manual", label: "Manuel" },
                  ]}
                  onChange={(value) => props.onField("positionSource", value)}
                />
              </div>
              <input
                id="route-start"
                ref={props.startInput}
                required
                spellCheck={false}
                readOnly={form.positionSource === "vr"}
                value={form.start}
                onChange={(event) => props.onStart(event.target.value)}
              />
            </div>

            <div className="field">
              <div className="field-head">
                <label htmlFor="route-destination">
                  <span className="field-dot destination" aria-hidden="true" />
                  Destination
                </label>
              </div>
              <input
                id="route-destination"
                required
                spellCheck={false}
                value={form.destination}
                onChange={(event) => props.onDestination(event.target.value)}
              />
              {(checkpoints.length > 0 || props.gameArrivalName) && (
                <select
                  id="route-checkpoints-list"
                  className="field-secondary"
                  aria-label="Remplir la destination depuis une porte ou l'arrivée du jeu"
                  value={form.checkpoint}
                  onChange={(event) => props.onCheckpoint(event.target.value)}
                >
                  <option value="" disabled>
                    Choisir une porte ou l'arrivée du jeu…
                  </option>
                  {props.gameArrivalName && <option value="game-arrival">Arrivée officielle — {props.gameArrivalName}</option>}
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
            </div>
          </section>

          <section className="form-section" aria-labelledby="route-section-calculation">
            <h3 id="route-section-calculation">Calcul</h3>
            <Segmented
              id="route-mode"
              name="mode"
              label="Mode de calcul"
              value={form.mode}
              options={[
                { value: "quick", label: "Rapide", hint: "jusqu'à 10 jours" },
                { value: "precise", label: "Précis", hint: "jusqu'à 4 jours" },
              ]}
              onChange={(value) => props.onField("mode", value)}
            />
            <div className="form-grid">
              <div className="field">
                <label htmlFor="route-horizon">Horizon</label>
                <select id="route-horizon" value={form.horizon} onChange={(event) => props.onField("horizon", event.target.value as HorizonChoice)}>
                  {horizonOptionsFor(form.mode).map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </div>
              <div className="field">
                <label htmlFor="route-instant">Départ (heure locale)</label>
                <input
                  id="route-instant"
                  required
                  placeholder="JJ/MM/AAAA HH:MM"
                  value={form.instant}
                  pattern="\d{2}/\d{2}/\d{4} \d{2}:\d{2}"
                  onChange={(event) => props.onField("instant", event.target.value)}
                />
              </div>
            </div>
            <p className={props.horizonNote.warning ? "field-note warning" : "field-note"}>{props.horizonNote.text}</p>
          </section>

          <Collapsible title="Bateau" summary={boatSummary}>
            <div className="field">
              <label htmlFor="route-polar">Polaire</label>
              <select id="route-polar" required value={form.polar} onChange={(event) => props.onField("polar", event.target.value)}>
                {polars.map((polar) => (
                  <option key={polar.value} value={polar.value}>
                    {polarDisplayName(polar.label)}
                  </option>
                ))}
              </select>
              <p className={props.polarNote.detected ? "field-note detected" : "field-note"} title={props.polarNote.title}>
                {props.polarNote.text}
              </p>
            </div>
            <div className="field">
              <span className="field-label">Options possédées</span>
              <div className="chips">
                {EQUIPMENT.map((item) => (
                  <label className={form.equipment.has(item) ? "chip on" : "chip"} key={item}>
                    <input type="checkbox" value={item} checked={form.equipment.has(item)} onChange={(event) => props.onToggleEquipment(item, event.target.checked)} />
                    {EQUIPMENT_LABELS[item]}
                  </label>
                ))}
              </div>
            </div>
          </Collapsible>

          <Collapsible title="Réglages avancés" summary={advancedSummary}>
            <div className="form-grid">
              <div className="field">
                <label htmlFor="route-step">Pas (min)</label>
                <input id="route-step" type="number" min="1" required value={form.step} onChange={(event) => props.onField("step", event.target.value)} />
              </div>
              <div className="field">
                <label htmlFor="route-coastal-margin">Marge à la côte (NM)</label>
                <input
                  id="route-coastal-margin"
                  type="number"
                  min="0"
                  max="10"
                  step="0.5"
                  required
                  value={form.coastalMarginNM}
                  onChange={(event) => props.onField("coastalMarginNM", event.target.value)}
                />
              </div>
            </div>
            <div className="field">
              <div className="field-head">
                <span className="field-label">Stamina</span>
                <Segmented
                  id="route-stamina-source"
                  name="stamina-source"
                  label="Source de la stamina"
                  compact
                  value={form.staminaSource}
                  options={[
                    { value: "vr", label: "Jeu" },
                    { value: "manual", label: "Manuel" },
                  ]}
                  onChange={(value) => props.onField("staminaSource", value)}
                />
              </div>
              {form.staminaSource === "manual" && (
                <input
                  id="route-stamina-manual"
                  type="number"
                  min="0"
                  max="100"
                  step="0.5"
                  aria-label="Stamina manuelle (%)"
                  value={form.staminaManual}
                  onChange={(event) => props.onField("staminaManual", event.target.value)}
                />
              )}
            </div>
            <label className="switch">
              <input
                id="route-use-course-gates"
                type="checkbox"
                checked={form.useCourseGates}
                onChange={(event) => props.onField("useCourseGates", event.target.checked)}
              />
              <span className="switch-track" aria-hidden="true" />
              Passer par les portes de la course
            </label>
          </Collapsible>
        </div>

        <footer className="route-form-footer">
          <p id="route-form-error" className="form-error" role="alert">
            {props.error}
          </p>
          <button id="submit-route" className="form-submit" type="submit" disabled={props.submitting}>
            <span className="material-symbols-outlined" aria-hidden="true">
              alt_route
            </span>
            Calculer la route
          </button>
          <p id="route-weather-status" className="form-hint">
            {props.weatherStatus}
          </p>
        </footer>
      </form>
    </aside>
  );
}

import { useEffect, useMemo, useRef, useState } from "react";
import { BEAUFORT, beaufortForce } from "./lib/beaufort.ts";
import { maneuverLabel } from "./lib/route_details.ts";
import { dayTicks, formatOffset, fractionOf, windGradient, type TimelineMark, type TimelineRange, type WindBand } from "./lib/timeline.ts";

/** Durée d'une lecture complète de la frise, quelle que soit sa longueur. */
const PLAYBACK_MS = 30_000;
const DAY_MS = 86_400_000;
/** Écart minimal entre deux noms de jour (« mer. 25 » fait environ 50 px). */
const LABEL_MIN_GAP_PX = 62;
/** Centré sur son trait, un nom de jour plus près du bord déborderait sur le bouton ou la lecture. */
const LABEL_EDGE_PX = 12;

const CURSOR_FORMAT = new Intl.DateTimeFormat("fr-FR", {
  weekday: "short",
  day: "2-digit",
  month: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
});
const DAY_FORMAT = new Intl.DateTimeFormat("fr-FR", { weekday: "short", day: "numeric" });

/** État du bateau au curseur, tiré du pas de route en cours. */
export type TimelineReadout = {
  twa: string;
  /** Vent en nœuds, déjà formaté (« 29,5 »). */
  tws: string;
  /** Vent en nœuds, pour sa force Beaufort, ou null s'il est inconnu. */
  twsKnots: number | null;
  sail: string;
};

type Props = {
  range: TimelineRange;
  cursor: number;
  onCursor: (time: number) => void;
  /** Pas du curseur, en minutes. */
  stepMinutes: number;
  /** Frise liée à une route (sinon elle ne pilote que le vent). */
  hasRoute: boolean;
  disabled?: boolean;
  marks: TimelineMark[];
  bands: WindBand[];
  readout: TimelineReadout | null;
  windShown: boolean;
  onShowWind: () => void;
};

/**
 * Frise de temps du cockpit, en ligne fine : un rail aux couleurs du vent le
 * long de la route, un curseur rond et les manœuvres en points jaunes. Le
 * curseur fait avancer le bateau sur la route, l'isochrone mise en avant et le
 * fond de vent ; la lecture le déroule seul.
 */
export function Timeline(props: Props) {
  const { range, cursor, onCursor, stepMinutes, hasRoute, disabled = false, marks, bands, readout } = props;
  const [playing, setPlaying] = useState(false);
  const cursorRef = useRef(cursor);
  cursorRef.current = cursor;
  const onCursorRef = useRef(onCursor);
  onCursorRef.current = onCursor;

  useEffect(() => {
    if (disabled) setPlaying(false);
  }, [disabled]);

  // Lecture : le curseur parcourt toute la frise en PLAYBACK_MS, puis s'arrête.
  useEffect(() => {
    if (!playing) return;
    const span = range.end - range.start;
    let previous = performance.now();
    let frame = 0;
    const tick = (now: number) => {
      const next = Math.min(range.end, cursorRef.current + ((now - previous) / PLAYBACK_MS) * span);
      previous = now;
      onCursorRef.current(next);
      if (next >= range.end) {
        setPlaying(false);
        return;
      }
      frame = requestAnimationFrame(tick);
    };
    frame = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(frame);
  }, [playing, range.start, range.end]);

  const togglePlay = () => {
    if (!playing && cursorRef.current >= range.end) onCursor(range.start);
    setPlaying((value) => !value);
  };

  const ticks = useMemo(() => dayTicks(range), [range]);
  // Un nom de jour sur `labelEvery` : assez d'écart pour qu'ils ne se chevauchent pas.
  const track = useRef<HTMLDivElement>(null);
  const [trackWidth, setTrackWidth] = useState(0);
  useEffect(() => {
    const element = track.current;
    if (!element) return;
    const observer = new ResizeObserver(() => setTrackWidth(element.clientWidth));
    observer.observe(element);
    return () => observer.disconnect();
  }, []);
  const dayWidth = (trackWidth * DAY_MS) / Math.max(1, range.end - range.start);
  const labelEvery = dayWidth > 0 ? Math.max(1, Math.ceil(LABEL_MIN_GAP_PX / dayWidth)) : 1;
  const percent = `${(fractionOf(range, cursor) * 100).toFixed(3)}%`;
  const offset = formatOffset((cursor - range.start) / 60_000);
  const stepMs = Math.max(1, stepMinutes) * 60_000;
  const gradient = useMemo(() => windGradient(bands), [bands]);
  const force = readout && readout.twsKnots !== null ? BEAUFORT[Math.max(0, beaufortForce(readout.twsKnots))] : null;

  return (
    <section id="timeline" className={disabled ? "timeline is-disabled" : "timeline"} aria-label="Frise de temps">
      <button
        type="button"
        className="timeline-play"
        aria-label={playing ? "Mettre en pause" : "Lire la route dans le temps"}
        title={playing ? "Pause" : "Lire"}
        disabled={disabled}
        onClick={togglePlay}
      >
        <span className="material-symbols-outlined" aria-hidden="true">
          {playing ? "pause" : "play_arrow"}
        </span>
      </button>
      <div ref={track} className="timeline-track">
        <span
          className="timeline-rail"
          style={hasRoute ? { backgroundImage: gradient } : undefined}
          role={hasRoute ? "img" : undefined}
          aria-label={hasRoute ? "Force du vent le long de la route, échelle de Beaufort" : undefined}
        />
        {hasRoute && <span className="timeline-future" style={{ left: percent }} />}
        {ticks.map((tick, index) => (
          <i key={tick.time} className="timeline-midnight" style={{ left: `${tick.fraction * 100}%` }}>
            {index % labelEvery === 0 && Math.min(tick.fraction, 1 - tick.fraction) * trackWidth >= LABEL_EDGE_PX && (
              <b>{DAY_FORMAT.format(new Date(tick.time))}</b>
            )}
          </i>
        ))}
        {!hasRoute && !props.windShown && (
          <span className="timeline-hint">
            Calculez une route pour la rejouer, ou{" "}
            <button type="button" className="link-button" onClick={props.onShowWind}>
              affichez le vent
            </button>
          </span>
        )}
        {marks.map((mark) => (
          <button
            key={mark.index}
            type="button"
            className="timeline-mark"
            style={{ left: `${mark.fraction * 100}%` }}
            aria-label={`${maneuverLabel(mark.maneuver)} à ${CURSOR_FORMAT.format(new Date(mark.time))}`}
            title={`${maneuverLabel(mark.maneuver)} · ${CURSOR_FORMAT.format(new Date(mark.time))}`}
            disabled={disabled}
            onClick={() => onCursor(mark.time)}
          />
        ))}
        <input
          className="timeline-range"
          type="range"
          min={range.start}
          max={range.end}
          step={stepMs}
          value={cursor}
          disabled={disabled}
          aria-label="Heure affichée"
          aria-valuetext={`${CURSOR_FORMAT.format(new Date(cursor))}, ${offset}`}
          onChange={(event) => {
            setPlaying(false);
            onCursor(Number(event.target.value));
          }}
        />
        <span className="timeline-cursor" style={{ left: percent }} aria-hidden="true" />
      </div>
      <div className="timeline-readout" aria-live="off">
        <span>
          <strong>{CURSOR_FORMAT.format(new Date(cursor))}</strong>
          <em>{offset}</em>
        </span>
        {readout && (
          <span className="timeline-state">
            {force && <i className="timeline-force" style={{ background: force.color }} title={`Force ${force.force} : ${force.label}`} />}
            {`${readout.tws} kn · TWA ${readout.twa} · ${readout.sail}`}
          </span>
        )}
      </div>
    </section>
  );
}

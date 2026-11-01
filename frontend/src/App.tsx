import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { MapView, DEFAULT_DEPARTURE, DEFAULT_DESTINATION, type Endpoints, type MapViewHandle, type RouteFrame } from "./MapView.tsx";
import { RouteDetailsPanel } from "./RouteDetailsPanel.tsx";
import { RouteForm, type FormState, type VrStatus } from "./RouteForm.tsx";
import { WeatherRunCard } from "./WeatherRunCard.tsx";
import { ZonesPanel } from "./ZonesPanel.tsx";
import { formatLocalDateTime, formatVirtualRegattaPosition, parseLocalDate, parseVirtualRegattaPosition } from "./lib/geo.ts";
import { forecastHoursFor, formatDuration, horizonAllowedIn, horizonMinutes, HORIZON_OPTIONS, MAX_MINUTES_BY_MODE } from "./lib/horizon.ts";
import { readSSEStream } from "./lib/route_stream.ts";
import type { Checkpoint, Isochrone, Point, PolarOption, RouteResult, RouteStart, VrState } from "./lib/types.ts";
import { matchPolar } from "./lib/polar_match.ts";
import { buildWeatherRunModel } from "./lib/weather_run.ts";
import {
  DISABLED_GAME_ZONES_KEY,
  MIN_ZONE_POINTS,
  ZONES_STORAGE_KEY,
  createZone,
  gameZonesFrom,
  parseDisabledIds,
  parseStoredZones,
  zonesForRequest,
  type Zone,
} from "./lib/zones.ts";
import { EQUIPMENT, equipmentFromVrOptions, checkpointKey, usableCheckpoints, type Equipment } from "./lib/vr.ts";

const VR_POLL_INTERVAL_MS = 10_000;
const DEFAULT_POLAR = "../data/polars/ultim_bp_11.json";
// Le tracé affiché vient des isochrones du mode choisi ; ces paramètres ne servent
// qu'à l'évaluation des caps pratiques, que l'interface n'affiche pas.
const PRACTICAL_HEADINGS = Array.from({ length: 24 }, (_, index) => index * 15);

function initialForm(): FormState {
  return {
    start: `13°N 07'16" - 121°E 35'12"`,
    useVrState: true,
    destination: `13°N 10'16" - 121°E 20'12"`,
    instant: formatLocalDateTime(new Date()),
    polar: DEFAULT_POLAR,
    horizon: "arrival",
    step: "10",
    mode: "precise",
    equipment: new Set<Equipment>(),
    checkpoint: "",
  };
}

// randomUUID n'existe que dans un contexte sécurisé (localhost, HTTPS).
function newId(): string {
  return globalThis.crypto?.randomUUID?.() ?? `zone-${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

function MaterialIcon({ name }: { name: string }) {
  return (
    <span className="material-symbols-outlined" aria-hidden="true">
      {name}
    </span>
  );
}

export function App() {
  const [form, setForm] = useState<FormState>(initialForm);
  const [formOpen, setFormOpen] = useState(false);
  const [formError, setFormError] = useState("");
  const [polars, setPolars] = useState<PolarOption[]>([{ label: "ultim_bp_11.json", value: DEFAULT_POLAR }]);
  const [endpoints, setEndpoints] = useState<Endpoints>({ position: DEFAULT_DEPARTURE, destination: DEFAULT_DESTINATION });
  const [frame, setFrame] = useState<RouteFrame | null>(null);
  const [result, setResult] = useState<RouteResult | undefined>();
  const [detailsOpen, setDetailsOpen] = useState(false);
  const [calculating, setCalculating] = useState(false);
  const [progress, setProgress] = useState("");
  const [vrState, setVrState] = useState<VrState | undefined>();
  const [vrStatus, setVrStatus] = useState<VrStatus>({ ok: false, text: "Virtual Regatta : lecture en cours…" });
  const [weather, setWeather] = useState<{ files: string[]; run: string }>({ files: [], run: "" });
  const [weatherStatus, setWeatherStatus] = useState("");
  const [refreshing, setRefreshing] = useState(false);
  const [mapError, setMapError] = useState(false);
  const [zones, setZones] = useState<Zone[]>(() => {
    try {
      return parseStoredZones(window.localStorage.getItem(ZONES_STORAGE_KEY));
    } catch {
      return [];
    }
  });
  const [disabledGameZones, setDisabledGameZones] = useState<Set<string>>(() => {
    try {
      return parseDisabledIds(window.localStorage.getItem(DISABLED_GAME_ZONES_KEY));
    } catch {
      return new Set();
    }
  });
  const [zonesOpen, setZonesOpen] = useState(false);
  const [draft, setDraft] = useState<Point[] | null>(null);

  const mapHandle = useRef<MapViewHandle>(null);
  const startInput = useRef<HTMLInputElement>(null);
  const activeController = useRef<AbortController | null>(null);
  const checkpointCache = useRef(new Map<string, { start: Checkpoint["start"]; end: Checkpoint["end"] }>());
  const streamIsochrones = useRef<Isochrone[]>([]);
  const streamBase = useRef<RouteResult | null>(null);
  const frameCounter = useRef(0);

  const weatherModel = useMemo(
    () => (weather.run ? buildWeatherRunModel(weather.run, weather.files) : null),
    [weather],
  );
  const checkpoints = useMemo(() => usableCheckpoints(vrState), [vrState]);

  // Ce que les prévisions téléchargées permettent pour l'horizon choisi.
  const horizonNote = useMemo(() => {
    if (!weatherModel) return { warning: false, text: "Actualisez les GRIB pour connaître l'horizon disponible." };
    let start = Number.NaN;
    try {
      start = Date.parse(parseLocalDate(form.instant));
    } catch {
      return { warning: true, text: "Heure de départ invalide." };
    }
    const result = horizonMinutes(form.horizon, start, Date.parse(weatherModel.coveredUntil), MAX_MINUTES_BY_MODE[form.mode]);
    if (!result.ok) return { warning: true, text: result.error };
    if (result.limited) {
      const label = HORIZON_OPTIONS.find((option) => option.value === form.horizon)?.label ?? "";
      return {
        warning: true,
        text: `Limité à ${formatDuration(result.minutes)} par les prévisions : actualisez les GRIB pour couvrir ${label.toLowerCase()}.`,
      };
    }
    return {
      warning: false,
      text:
        form.horizon === "arrival"
          ? `Jusqu'à l'arrivée, au plus ${formatDuration(result.minutes)} (prévisions disponibles).`
          : `Calcul sur ${formatDuration(result.minutes)}.`,
    };
  }, [weatherModel, form.horizon, form.instant, form.mode]);

  // Polaire du bateau reçu de Virtual Regatta : choisie automatiquement quand le
  // bateau détecté change, sans écraser un choix manuel entre deux détections.
  const polarMatch = useMemo(() => matchPolar(polars, vrState?.boatHints), [polars, vrState]);
  const autoPolar = useRef("");
  useEffect(() => {
    if (!polarMatch || polarMatch.polar.value === autoPolar.current) return;
    autoPolar.current = polarMatch.polar.value;
    setForm((previous) => ({ ...previous, polar: polarMatch.polar.value }));
  }, [polarMatch]);
  const polarNote = polarMatch
    ? {
        detected: true,
        text: `Bateau détecté : ${polarMatch.polar.boat || polarMatch.polar.label}`,
        title: `${polarMatch.hint.path} = ${polarMatch.hint.value}`,
      }
    : {
        detected: false,
        text: vrState
          ? "Bateau non fourni par Virtual Regatta : choisissez la polaire."
          : "Choisissez la polaire de votre bateau.",
        title: undefined,
      };

  // En mode précis, l'horizon de 5 jours n'existe pas : on revient à 3 jours.
  useEffect(() => {
    const allowed = horizonAllowedIn(form.horizon, form.mode);
    if (allowed !== form.horizon) setForm((previous) => ({ ...previous, horizon: allowed }));
  }, [form.horizon, form.mode]);

  // Les zones restent dans le navigateur d'une session à l'autre.
  useEffect(() => {
    try {
      window.localStorage.setItem(ZONES_STORAGE_KEY, JSON.stringify(zones));
    } catch {
      // Stockage indisponible : les zones restent valables pour la session.
    }
  }, [zones]);

  useEffect(() => {
    try {
      window.localStorage.setItem(DISABLED_GAME_ZONES_KEY, JSON.stringify([...disabledGameZones]));
    } catch {
      // Stockage indisponible : le choix reste valable pour la session.
    }
  }, [disabledGameZones]);

  // Zones reçues du jeu (relevées par l'extension) puis zones dessinées.
  const gameZones = useMemo(() => gameZonesFrom(vrState, disabledGameZones), [vrState, disabledGameZones]);
  const allZones = useMemo(() => [...gameZones, ...zones], [gameZones, zones]);
  const enabledZoneCount = allZones.filter((zone) => zone.enabled).length;

  const setField = useCallback(<K extends keyof FormState>(field: K, value: FormState[K]) => {
    setForm((previous) => ({ ...previous, [field]: value }));
  }, []);

  // --- Réinitialisation de la route (uniquement sur action de l'utilisateur) ---
  const resetRoute = useCallback(() => {
    setFrame(null);
    setResult(undefined);
    setDetailsOpen(false);
    streamIsochrones.current = [];
    streamBase.current = null;
  }, []);

  const updateEndpoints = useCallback(
    (start: string, destination: string) => {
      const position = parseVirtualRegattaPosition(start);
      const target = parseVirtualRegattaPosition(destination);
      if (!position || !target) return;
      resetRoute();
      setEndpoints({ position, destination: target });
    },
    [resetRoute],
  );

  // --- Zones interdites (actions de l'utilisateur : la route affichée devient périmée) ---
  const startDrawing = () => {
    setFormOpen(false);
    setZonesOpen(false);
    setDetailsOpen(false);
    setDraft([]);
  };
  const draftRef = useRef<Point[] | null>(null);
  draftRef.current = draft;
  const finishDrawing = useCallback(() => {
    const current = draftRef.current;
    if (!current || current.length < MIN_ZONE_POINTS) return;
    setZones((previous) => {
      const zone = createZone(previous, current, newId());
      return zone ? [...previous, zone] : previous;
    });
    resetRoute();
    setZonesOpen(true);
    setDraft(null);
  }, [resetRoute]);
  const cancelDrawing = useCallback(() => {
    setDraft(null);
    setZonesOpen(true);
  }, []);
  const onDraftClick = useCallback(
    (point: Point, closing: boolean) => {
      if (closing) {
        finishDrawing();
        return;
      }
      setDraft((current) => (current ? [...current, point] : current));
    },
    [finishDrawing],
  );
  const updateZones = (change: (previous: Zone[]) => Zone[]) => {
    setZones(change);
    resetRoute();
  };

  // Raccourcis pendant le tracé : Entrée termine, Retour arrière annule le dernier
  // point, Échap abandonne.
  const drawing = draft !== null;
  useEffect(() => {
    if (!drawing) return;
    const onKeyDown = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      if (target && /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName)) return;
      if (event.key === "Enter") finishDrawing();
      else if (event.key === "Escape") cancelDrawing();
      else if (event.key === "Backspace") {
        event.preventDefault();
        setDraft((current) => (current ? current.slice(0, -1) : current));
      }
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [drawing, finishDrawing, cancelDrawing]);

  const onStart = (value: string) => {
    setField("start", value);
    updateEndpoints(value, form.destination);
  };
  const onDestination = (value: string) => {
    setField("destination", value);
    updateEndpoints(form.start, value);
  };
  const onCheckpoint = (key: string) => {
    setField("checkpoint", key);
    const checkpoint = checkpointCache.current.get(key);
    if (!checkpoint?.start) return;
    const destination = formatVirtualRegattaPosition(checkpoint.start);
    setForm((previous) => ({ ...previous, checkpoint: key, destination }));
    updateEndpoints(form.start, destination);
  };
  const onToggleEquipment = (item: Equipment, checked: boolean) => {
    setForm((previous) => {
      const equipment = new Set(previous.equipment);
      if (checked) equipment.add(item);
      else equipment.delete(item);
      return { ...previous, equipment };
    });
  };

  // --- Polaires disponibles ---
  useEffect(() => {
    let cancelled = false;
    fetch("/api/polars")
      .then((response) => (response.ok ? response.json() : Promise.reject(new Error(`HTTP ${response.status}`))))
      .then((body: { polars: PolarOption[]; default: string }) => {
        if (cancelled || !body.polars?.length) return;
        setPolars(body.polars);
        setForm((previous) => (body.polars.some((polar) => polar.value === previous.polar) ? previous : { ...previous, polar: body.default || body.polars[0].value }));
      })
      .catch((error) => console.warn("[POLARS] Liste indisponible", error));
    return () => {
      cancelled = true;
    };
  }, []);

  // --- État Virtual Regatta ---
  const refreshVrState = useCallback(async () => {
    try {
      const response = await fetch("/api/vr/state", { cache: "no-store" });
      const state = (await response.json()) as VrState & { error?: string };
      if (!response.ok) throw new Error(state.error || `HTTP ${response.status}`);
      (state.checkpoints || []).forEach((checkpoint, index) => {
        checkpointCache.current.set(checkpointKey(checkpoint, index), { start: checkpoint.start, end: checkpoint.end });
      });
      setVrState(state);
      const position = formatVirtualRegattaPosition(state.position);
      // Mise à jour silencieuse : la route affichée n'est pas effacée.
      setForm((previous) => ({
        ...previous,
        start: position || previous.start,
        equipment: equipmentFromVrOptions(state.options),
      }));
      const stamina = state.stamina == null ? "inconnue" : `${Number(state.stamina).toFixed(1)} %`;
      const remaining = usableCheckpoints(state).length;
      setVrStatus({
        ok: true,
        text: `Virtual Regatta connecté · stamina ${stamina} · ${remaining} porte(s) restante(s)`,
        title: state.updatedAt ? `Mise à jour ${new Date(Number(state.updatedAt)).toLocaleTimeString()}` : undefined,
      });
    } catch (error) {
      setVrStatus({ ok: false, text: `Virtual Regatta indisponible : ${(error as Error).message}` });
      console.warn("[VR STATE] Lecture impossible", error);
    }
  }, []);

  useEffect(() => {
    void refreshVrState();
  }, [refreshVrState]);

  useEffect(() => {
    if (!formOpen && !zonesOpen) return;
    void refreshVrState();
    const timer = setInterval(() => void refreshVrState(), VR_POLL_INTERVAL_MS);
    return () => clearInterval(timer);
  }, [formOpen, zonesOpen, refreshVrState]);

  useEffect(() => {
    if (formOpen) startInput.current?.focus();
  }, [formOpen]);

  // --- Actualisation des GRIB (ne touche pas à l'heure de départ) ---
  const refreshWeather = async () => {
    setRefreshing(true);
    console.info("[GRIB] Demande d’actualisation envoyée");
    try {
      const response = await fetch("/api/weather/refresh", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ forecastHours: forecastHoursFor(form.horizon), lookbackHours: 48 }),
      });
      const body = await response.json();
      if (!response.ok) throw new Error(body.error || `HTTP ${response.status}`);
      const files: string[] = body.files || [];
      const run: string = body.run || "";
      setWeather({ files, run });
      setWeatherStatus("");
      console.info(`[GRIB] Actualisation réussie : run ${run}, ${files.length} fichier(s), ${body.removedFiles?.length || 0} ancien(s) supprimé(s)`);
    } catch (error) {
      setWeatherStatus(`Actualisation des GRIB impossible : ${(error as Error).message}`);
      console.error("[GRIB] Échec de l’actualisation", error);
    } finally {
      setRefreshing(false);
    }
  };

  // --- Calcul de la route en flux ---
  const submit = async () => {
    setFormError("");
    if (weather.files.length === 0 || !weather.run) {
      setFormError("Actualisez les GRIB avant de lancer le calcul.");
      console.warn("[ROUTE] Calcul impossible : actualisez d’abord les GRIB");
      return;
    }
    activeController.current?.abort();
    resetRoute();
    const controller = new AbortController();
    activeController.current = controller;
    console.info(`[ROUTE] Calcul demandé avec le run ${weather.run}`);
    try {
      const instant = parseLocalDate(form.instant);
      if (!weatherModel) throw new Error("Run GRIB invalide : actualisez les GRIB.");
      const horizon = horizonMinutes(
        form.horizon,
        Date.parse(instant),
        Date.parse(weatherModel.coveredUntil),
        MAX_MINUTES_BY_MODE[form.mode],
      );
      if (!horizon.ok) throw new Error(horizon.error);
      const duration = horizon.minutes;
      const step = Number(form.step);
      if (!Number.isFinite(step) || step <= 0 || step > duration) {
        throw new Error("Le pas est invalide pour cet horizon.");
      }
      setCalculating(true);
      setProgress("");
      const response = await fetch("/api/practical-headings/stream", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          weatherFiles: weather.files,
          polarPath: form.polar,
          position: { virtualRegatta: form.start },
          destination: { virtualRegatta: form.destination },
          instant,
          durationMinutes: duration,
          stepMinutes: step,
          headings: PRACTICAL_HEADINGS,
          owned: EQUIPMENT.filter((item) => form.equipment.has(item)),
          fixedSailID: 0,
          mode: form.mode,
          requireRobustScenarios: false,
          useVirtualRegattaState: form.useVrState,
          exclusionZones: zonesForRequest(allZones),
        }),
        signal: controller.signal,
      });
      await readSSEStream(response, (eventName, data) => {
        if (activeController.current !== controller) return;
        if (eventName === "start") {
          const start = data as RouteStart;
          const position = start.position || DEFAULT_DEPARTURE;
          const destination = start.destination || DEFAULT_DESTINATION;
          frameCounter.current += 1;
          streamIsochrones.current = [];
          streamBase.current = {
            position,
            destination,
            route: start.startedAt ? [{ position, instant: start.startedAt }] : [],
            isochrones: [],
          };
          setFrame({ result: streamBase.current, showRoute: false, fit: false, key: String(frameCounter.current) });
          return;
        }
        if (eventName === "frontier") {
          const base = streamBase.current;
          if (!base) return;
          streamIsochrones.current = [...streamIsochrones.current, data as Isochrone];
          const partial = { ...base, isochrones: streamIsochrones.current };
          setFrame({ result: partial, showRoute: false, fit: false, key: String(frameCounter.current) });
          const frontier = data as Isochrone;
          setProgress(` (${frontier.completedLayers}/${frontier.totalLayers})`);
          return;
        }
        if (eventName === "complete") {
          const complete = data as RouteResult;
          setResult(complete);
          setFrame({ result: complete, showRoute: true, fit: true, key: String(frameCounter.current) });
          return;
        }
        if (eventName === "cancelled") throw new Error((data as { error?: string }).error || "Calcul annulé.");
        if (eventName === "error") throw new Error((data as { error?: string }).error || "Le calcul a échoué.");
      });
      setFormOpen(false);
      console.info("[ROUTE] Flux de calcul terminé");
    } catch (error) {
      if (!(controller.signal.aborted && activeController.current !== controller)) {
        setFormError((error as Error).message || "Le calcul a échoué.");
        console.error("[ROUTE] Échec du calcul", error);
      }
    } finally {
      if (activeController.current === controller) {
        activeController.current = null;
        setCalculating(false);
      }
    }
  };

  const closeDetails = useCallback(() => setDetailsOpen(false), []);
  const [selectedRow, setSelectedRow] = useState<{ index: number; point: Point } | null>(null);
  useEffect(() => setSelectedRow(null), [result]);
  const hasRoute = Boolean(result && result.route.length > 0);

  return (
    <>
      <MapView
        ref={mapHandle}
        endpoints={endpoints}
        frame={frame}
        vrState={vrState}
        zones={allZones}
        highlight={selectedRow?.point ?? null}
        draft={draft}
        onDraftClick={onDraftClick}
        rightPadding={() =>
          ["route-form-panel", "route-details-panel"].reduce(
            (width, id) => Math.max(width, document.getElementById(id)?.getBoundingClientRect().width || 0),
            0,
          )
        }
        onError={() => setMapError(true)}
      />
      {calculating && (
        <div id="route-calculation-status" className="route-calculation-status" role="status" aria-live="polite">
          <MaterialIcon name="progress_activity" />
          <span>{`Calcul de la route en cours…${progress}`}</span>
        </div>
      )}
      <div id="map-error" role="alert" className={mapError ? "visible" : undefined}>
        MapLibre n’a pas pu être chargé.
      </div>
      <WeatherRunCard model={weatherModel} />
      <div className="map-dock" role="toolbar" aria-label="Actions de la carte" aria-orientation="vertical">
        <button
          id="calculate-route"
          className="map-dock-button map-dock-primary"
          type="button"
          aria-label="Calculer une route"
          data-tooltip="Calculer une route"
          onClick={() => {
            setZonesOpen(false);
            setFormOpen(true);
          }}
        >
          <MaterialIcon name="alt_route" />
        </button>
        <hr />
        <button
          id="toggle-zones"
          className={zonesOpen || drawing ? "map-dock-button is-active" : "map-dock-button"}
          type="button"
          aria-label="Zones interdites"
          aria-pressed={zonesOpen}
          data-tooltip={`Zones interdites${enabledZoneCount > 0 ? ` (${enabledZoneCount} active(s))` : ""}`}
          onClick={() => {
            setFormOpen(false);
            setZonesOpen((open) => !open);
          }}
        >
          <MaterialIcon name="block" />
          {enabledZoneCount > 0 && <span className="dock-badge">{enabledZoneCount}</span>}
        </button>
        <hr />
        <button
          id="refresh-weather"
          className={refreshing ? "map-dock-button is-busy" : "map-dock-button"}
          type="button"
          aria-label="Actualiser les GRIB"
          data-tooltip={
            refreshing
              ? "Téléchargement des GRIB…"
              : weatherModel
                ? `Actualiser les GRIB\nRun ${weatherModel.runText} · +${weatherModel.horizonHours} h`
                : "Actualiser les GRIB"
          }
          disabled={refreshing}
          onClick={() => void refreshWeather()}
        >
          <MaterialIcon name="cloud_download" />
        </button>
        <hr />
        <button
          id="recenter-map"
          className="map-dock-button"
          type="button"
          aria-label="Recentrer la carte"
          data-tooltip="Recentrer la carte"
          onClick={() => mapHandle.current?.recenter()}
        >
          <MaterialIcon name="my_location" />
        </button>
      </div>
      {hasRoute && (
        <button id="show-route-details" className="route-details-button" type="button" aria-controls="route-details-panel" aria-expanded={detailsOpen} onClick={() => setDetailsOpen(true)}>
          <MaterialIcon name="route" />
          Détails de la route
        </button>
      )}
      {detailsOpen && result && <RouteDetailsPanel
          result={result}
          selectedIndex={selectedRow?.index ?? null}
          onSelect={(index, point) => setSelectedRow({ index, point })}
          onClose={closeDetails}
        />}
      {draft !== null && (
        <div className="draw-banner" role="status" aria-live="polite">
          <span>
            {draft.length < MIN_ZONE_POINTS
              ? `Tracé d'une zone interdite : ${draft.length} point(s), cliquez sur la carte (3 minimum)`
              : `Zone de ${draft.length} points : cliquez sur le premier point ou appuyez sur Entrée pour terminer`}
          </span>
          <button type="button" onClick={() => setDraft((current) => (current ? current.slice(0, -1) : current))} disabled={draft.length === 0}>
            Annuler le dernier point
          </button>
          <button type="button" className="primary" onClick={finishDrawing} disabled={draft.length < MIN_ZONE_POINTS}>
            Terminer
          </button>
          <button type="button" onClick={cancelDrawing}>
            Abandonner
          </button>
        </div>
      )}
      {zonesOpen && (
        <ZonesPanel
          zones={allZones}
          game={{ stateKnown: vrState !== undefined, count: gameZones.length, legShape: vrState?.legShape || [] }}
          onDraw={startDrawing}
          onToggle={(zone, enabled) => {
            if (zone.source === "game") {
              setDisabledGameZones((previous) => {
                const next = new Set(previous);
                if (enabled) next.delete(zone.id);
                else next.add(zone.id);
                return next;
              });
              resetRoute();
            } else {
              updateZones((previous) => previous.map((item) => (item.id === zone.id ? { ...item, enabled } : item)));
            }
          }}
          onRename={(id, name) => setZones((previous) => previous.map((zone) => (zone.id === id ? { ...zone, name } : zone)))}
          onDelete={(id) => updateZones((previous) => previous.filter((zone) => zone.id !== id))}
          onClose={() => setZonesOpen(false)}
        />
      )}
      {formOpen && (
        <RouteForm
          form={form}
          polars={polars}
          checkpoints={checkpoints}
          checkpointCache={checkpointCache.current}
          polarNote={polarNote}
          horizonNote={horizonNote}
          vrStatus={vrStatus}
          weatherStatus={
            weatherStatus ||
            (weatherModel
              ? `${weatherModel.title} · run ${weatherModel.runText} · +${weatherModel.horizonHours} h`
              : "Actualisez les GRIB avant de lancer le calcul.")
          }
          error={formError}
          submitting={calculating}
          onField={setField}
          onStart={onStart}
          onDestination={onDestination}
          onCheckpoint={onCheckpoint}
          onToggleEquipment={onToggleEquipment}
          onSubmit={() => void submit()}
          onClose={() => setFormOpen(false)}
          startInput={startInput}
        />
      )}
    </>
  );
}

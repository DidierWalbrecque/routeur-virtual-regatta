import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { MapView, DEFAULT_DEPARTURE, DEFAULT_DESTINATION, type Endpoints, type GpxOverlay, type MapViewHandle, type RouteFrame } from "./MapView.tsx";
import { RouteDetailsPanel } from "./RouteDetailsPanel.tsx";
import { RouteForm, type FormState, type VrStatus } from "./RouteForm.tsx";
import { WeatherRunCard } from "./WeatherRunCard.tsx";
import { ZonesPanel } from "./ZonesPanel.tsx";
import { formatLocalDateTime, formatVirtualRegattaPosition, parseLocalDate, parseVirtualRegattaPosition } from "./lib/geo.ts";
import { formatDuration, GRIB_FORECAST_HOURS, horizonAllowedIn, horizonMinutes, HORIZON_OPTIONS, MAX_MINUTES_BY_MODE } from "./lib/horizon.ts";
import { readSSEStream } from "./lib/route_stream.ts";
import type { Checkpoint, Isochrone, Point, PolarOption, RouteResult, RouteStart, VrState } from "./lib/types.ts";
import { matchPolar } from "./lib/polar_match.ts";
import { parseGpx, gpxFeatureCollection, gpxPointCount } from "./lib/gpx.ts";
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
import { EQUIPMENT, equipmentFromVrOptions, checkpointKey, meaningfulName, usableCheckpoints, type Equipment } from "./lib/vr.ts";

const VR_POLL_INTERVAL_MS = 10_000;
const DEFAULT_POLAR = "../data/polars/ultim_bp_11.json";
// Le tracé affiché vient des isochrones du mode choisi ; ces paramètres ne servent
// qu'à l'évaluation des caps pratiques, que l'interface n'affiche pas.
const PRACTICAL_HEADINGS = Array.from({ length: 24 }, (_, index) => index * 15);

function initialForm(): FormState {
  return {
    start: `13°07'16.00"N - 121°35'12.00"E`,
    positionSource: "vr",
    staminaSource: "vr",
    staminaManual: "100",
    useCourseGates: true,
    coastalMarginNM: "2",
    destination: `13°10'16.00"N - 121°20'12.00"E`,
    instant: formatLocalDateTime(new Date()),
    polar: DEFAULT_POLAR,
    horizon: "arrival",
    step: "10",
    mode: "precise",
    equipment: new Set<Equipment>(),
    checkpoint: "",
  };
}

// Réglages du formulaire mémorisés d'une session à l'autre (sauf l'heure de
// départ, toujours réinitialisée à « maintenant »).
const FORM_STORAGE_KEY = "routeur-form";

function loadForm(): FormState {
  const base = initialForm();
  try {
    const raw = window.localStorage.getItem(FORM_STORAGE_KEY);
    if (!raw) return base;
    const saved = JSON.parse(raw) as Partial<Omit<FormState, "equipment">> & {
      equipment?: Equipment[];
      useVrState?: boolean;
    };
    const merged: FormState = {
      ...base,
      ...saved,
      instant: base.instant,
      equipment: new Set<Equipment>(saved.equipment ?? []),
    };
    // Migration de l'ancien réglage unique vers les trois réglages découplés.
    if (saved.positionSource === undefined && typeof saved.useVrState === "boolean") {
      const vr = saved.useVrState;
      merged.positionSource = vr ? "vr" : "manual";
      merged.staminaSource = vr ? "vr" : "manual";
      merged.useCourseGates = vr;
    }
    return merged;
  } catch {
    return base;
  }
}

// randomUUID n'existe que dans un contexte sécurisé (localhost, HTTPS).
function newId(): string {
  return globalThis.crypto?.randomUUID?.() ?? `zone-${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

// Lecture tolérante d'une réponse JSON : un corps vide (backend injoignable,
// proxy Vite en erreur) ne doit pas faire planter l'appelant avec
// « Unexpected end of JSON input ». On lit le texte puis on parse si possible.
type GribTarget = { run: string; downloaded: number; expected: number; complete: boolean };
type GribStatusBody = { files?: string[]; run?: string; removedFiles?: string[]; target?: GribTarget | null; busy?: boolean };
const GRIB_STATUS_POLL_MS = 10_000;

async function readJsonResponse<T = unknown>(response: Response): Promise<T> {
  const text = await response.text();
  let body: unknown;
  if (text) {
    try {
      body = JSON.parse(text);
    } catch {
      body = undefined;
    }
  }
  if (!response.ok) {
    const message =
      body && typeof body === "object" && "error" in body && typeof (body as { error?: unknown }).error === "string"
        ? (body as { error: string }).error
        : `HTTP ${response.status}`;
    throw new Error(message);
  }
  if (body === undefined) {
    throw new Error("réponse vide du serveur (backend injoignable ?)");
  }
  return body as T;
}

function MaterialIcon({ name }: { name: string }) {
  return (
    <span className="material-symbols-outlined" aria-hidden="true">
      {name}
    </span>
  );
}

export function App() {
  const [form, setForm] = useState<FormState>(loadForm);
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
  const [gribTarget, setGribTarget] = useState<GribTarget | null>(null);
  const [gribBusy, setGribBusy] = useState(false);
  const lastGribRunRef = useRef("");
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
  const [showRoutePoints, setShowRoutePoints] = useState(true);
  const [draft, setDraft] = useState<Point[] | null>(null);
  const [gpx, setGpx] = useState<{ name: string; count: number; overlay: GpxOverlay } | null>(null);
  const [gpxError, setGpxError] = useState("");

  const mapHandle = useRef<MapViewHandle>(null);
  const startInput = useRef<HTMLInputElement>(null);
  const activeController = useRef<AbortController | null>(null);
  const checkpointCache = useRef(new Map<string, { start: Checkpoint["start"]; end: Checkpoint["end"] }>());
  const streamIsochrones = useRef<Isochrone[]>([]);
  const streamBase = useRef<RouteResult | null>(null);
  const frameCounter = useRef(0);
  const gpxInput = useRef<HTMLInputElement>(null);
  const gpxCounter = useRef(0);

  const weatherModel = useMemo(
    () => (weather.run ? buildWeatherRunModel(weather.run, weather.files) : null),
    [weather],
  );

  const gribProgressText = useMemo(() => {
    if (!gribTarget || gribTarget.complete || gribTarget.downloaded === 0) return "";
    const label = `${gribTarget.run.slice(0, 8)} ${gribTarget.run.slice(8)}Z`;
    return `Run ${label} en cours de publication : ${gribTarget.downloaded}/${gribTarget.expected} échéances`;
  }, [gribTarget]);

  // Le bouton tourne pendant un clic « Actualiser » et pendant le téléchargement du run suivant en arrière-plan.
  const gribDownloading =
    refreshing || gribBusy || (gribTarget !== null && !gribTarget.complete && gribTarget.downloaded > 0);
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
    setForm((previous) => (previous.positionSource === "vr" ? { ...previous, polar: polarMatch.polar.value } : previous));
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

  // Un horizon trop long pour le mode choisi (ex. 10/16 j en précis) est ramené au maximum permis.
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

  // Mémorise les réglages (la Set d'équipements est sérialisée en tableau).
  useEffect(() => {
    try {
      window.localStorage.setItem(FORM_STORAGE_KEY, JSON.stringify({ ...form, equipment: [...form.equipment] }));
    } catch {
      // Stockage indisponible : les réglages restent valables pour la session.
    }
  }, [form]);

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

  // Les marqueurs suivent toujours le formulaire, y compris au chargement (valeurs mémorisées)
  // et après une modification indirecte (point de porte, saisie).
  useEffect(() => {
    updateEndpoints(form.start, form.destination);
  }, [form.start, form.destination, updateEndpoints]);

  // L'arrivée officielle du jeu remplace la destination dès qu'elle change (nouveau leg, première lecture).
  const gameArrivalKey = vrState?.arrival ? `${vrState.arrival.latitude},${vrState.arrival.longitude}` : "";
  useEffect(() => {
    if (!vrState?.arrival) return;
    const text = formatVirtualRegattaPosition({ latitude: vrState.arrival.latitude, longitude: vrState.arrival.longitude });
    setForm((previous) => (previous.destination === text ? previous : { ...previous, destination: text, checkpoint: "" }));
    console.info(`[VR] Arrivée du jeu : ${vrState.arrival.name || "sans nom"}`);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [gameArrivalKey]);

  // En mode Virtual Regatta, le marqueur de calcul suit la position live du bateau.
  const boatLatitude = vrState?.position?.latitude;
  const boatLongitude = vrState?.position?.longitude;
  useEffect(() => {
    if (form.positionSource !== "vr" || boatLatitude === undefined || boatLongitude === undefined) return;
    setEndpoints((previous) => ({ ...previous, position: { latitude: boatLatitude, longitude: boatLongitude } }));
  }, [form.positionSource, boatLatitude, boatLongitude]);

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
    if (key === "game-arrival") {
      const arrival = vrState?.arrival;
      if (!arrival) return;
      const destination = formatVirtualRegattaPosition({ latitude: arrival.latitude, longitude: arrival.longitude });
      setForm((previous) => ({ ...previous, checkpoint: key, destination }));
      updateEndpoints(form.start, destination);
      return;
    }
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
      const state = await readJsonResponse<VrState>(response);
      (state.checkpoints || []).forEach((checkpoint, index) => {
        checkpointCache.current.set(checkpointKey(checkpoint, index), { start: checkpoint.start, end: checkpoint.end });
      });
      setVrState(state);
      const position = formatVirtualRegattaPosition(state.position);
      // Mise à jour silencieuse : la route affichée n'est pas effacée.
      setForm((previous) =>
        previous.positionSource === "vr"
          ? { ...previous, start: position || previous.start, equipment: equipmentFromVrOptions(state.options) }
          : previous,
      );
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
        body: JSON.stringify({ forecastHours: GRIB_FORECAST_HOURS, lookbackHours: 48 }),
      });
      const body = await readJsonResponse<GribStatusBody>(response);
      applyGribStatus(body);
      setWeatherStatus("");
      console.info(`[GRIB] Actualisation réussie : run ${body.run || "aucun"}, ${body.files?.length || 0} fichier(s), ${body.removedFiles?.length || 0} ancien(s) supprimé(s)`);
    } catch (error) {
      setWeatherStatus(`Actualisation des GRIB impossible : ${(error as Error).message}`);
      console.error("[GRIB] Échec de l’actualisation", error);
    } finally {
      setRefreshing(false);
    }
  };

  // Adopte le run actif renvoyé par le serveur. Il ne change qu'une fois le run suivant entièrement téléchargé.
  const applyGribStatus = (body: GribStatusBody) => {
    const files: string[] = body.files || [];
    const run: string = body.run || "";
    setGribTarget(body.target ?? null);
    setGribBusy(body.busy ?? false);
    if (!run || files.length === 0) return;
    if (lastGribRunRef.current !== run) {
      console.info(`[GRIB] Run actif : ${run}, ${files.length} fichier(s)`);
      lastGribRunRef.current = run;
    }
    setWeather(previous =>
      previous.run === run && previous.files.length === files.length ? previous : { files, run },
    );
  };

  // Suivi de la publication du run suivant : le serveur le télécharge en arrière-plan.
  useEffect(() => {
    let cancelled = false;
    const poll = async () => {
      try {
        const response = await fetch("/api/weather/status", { cache: "no-store" });
        const body = await readJsonResponse<GribStatusBody>(response);
        if (!cancelled) applyGribStatus(body);
      } catch (error) {
        console.warn("[GRIB] Suivi de publication indisponible", error);
      }
    };
    const timer = window.setInterval(() => void poll(), GRIB_STATUS_POLL_MS);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, []);

  // Au démarrage, adopter les GRIB déjà présents sur disque sans re-télécharger.
  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const response = await fetch("/api/weather/current", { cache: "no-store" });
        const body = await readJsonResponse<GribStatusBody>(response);
        if (cancelled) return;
        applyGribStatus(body);
        if (body.run) console.info(`[GRIB] Run existant adopté : ${body.run}, ${body.files?.length || 0} fichier(s)`);
      } catch (error) {
        console.warn("[GRIB] État initial des GRIB indisponible", error);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  // --- Calcul de la route en flux ---
  const submit = async () => {
    setFormError("");
    // L'état local peut être en retard sur le watcher : le serveur fait foi pour le run actif.
    let gribFiles = weather.files;
    let gribRun = weather.run;
    if (gribFiles.length === 0 || !gribRun) {
      try {
        const response = await fetch("/api/weather/current", { cache: "no-store" });
        const body = await readJsonResponse<GribStatusBody>(response);
        applyGribStatus(body);
        gribFiles = body.files || [];
        gribRun = body.run || "";
      } catch (error) {
        console.warn("[ROUTE] Run GRIB actif indisponible", error);
      }
    }
    if (gribFiles.length === 0 || !gribRun) {
      setFormError(
        gribProgressText
          ? `${gribProgressText}. Le calcul sera possible dès que le run est complet.`
          : "Aucun run GRIB disponible : actualisez les GRIB.",
      );
      console.warn("[ROUTE] Calcul impossible : aucun run GRIB complet");
      return;
    }
    const gribModel = buildWeatherRunModel(gribRun, gribFiles);
    activeController.current?.abort();
    resetRoute();
    const controller = new AbortController();
    activeController.current = controller;
    console.info(`[ROUTE] Calcul demandé avec le run ${gribRun}`);
    try {
      const instant = parseLocalDate(form.instant);
      if (!gribModel) throw new Error("Run GRIB invalide : actualisez les GRIB.");
      const horizon = horizonMinutes(
        form.horizon,
        Date.parse(instant),
        Date.parse(gribModel.coveredUntil),
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
          weatherFiles: gribFiles,
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
          positionSource: form.positionSource,
          staminaSource: form.staminaSource,
          manualStamina: form.staminaSource === "manual" ? Number(form.staminaManual) : null,
          useCourseGates: form.useCourseGates,
          coastalMarginNM: Number(form.coastalMarginNM),
          arrivalRadiusNM: vrState?.arrival?.radius ?? null,
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

  // --- Trace GPX importée (affichée en référence sur la carte) ---
  const onGpxFile = useCallback(async (file: File | null) => {
    if (!file) return;
    try {
      const geometry = parseGpx(await file.text());
      const count = gpxPointCount(geometry);
      if (count === 0) throw new Error("aucune trace ni point de passage exploitable.");
      gpxCounter.current += 1;
      setGpx({
        name: file.name,
        count,
        overlay: { collection: gpxFeatureCollection(geometry), key: String(gpxCounter.current) },
      });
      setGpxError("");
      console.info(`[GPX] ${file.name} importé : ${count} point(s)`);
    } catch (error) {
      setGpx(null);
      setGpxError(`GPX illisible (${file.name}) : ${(error as Error).message}`);
      console.error("[GPX] Import impossible", error);
    }
  }, []);
  const clearGpx = useCallback(() => {
    setGpx(null);
    setGpxError("");
  }, []);

  const closeDetails = useCallback(() => setDetailsOpen(false), []);
  const [selectedRow, setSelectedRow] = useState<{ index: number; point: Point } | null>(null);
  useEffect(() => setSelectedRow(null), [result]);
  const hasRoute = Boolean(result && result.route.length > 0);

  return (
    <>
      <MapView
        ref={mapHandle}
        endpoints={endpoints}
        raceStart={vrState?.legStart ? { latitude: vrState.legStart.latitude, longitude: vrState.legStart.longitude } : null}
        gameArrival={vrState?.arrival ? { latitude: vrState.arrival.latitude, longitude: vrState.arrival.longitude } : null}
        frame={frame}
        vrState={vrState}
        zones={allZones}
        highlight={selectedRow?.point ?? null}
        draft={draft}
        gpx={gpx?.overlay ?? null}
        onDraftClick={onDraftClick}
        showRoutePoints={showRoutePoints}
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
      <input
        ref={gpxInput}
        type="file"
        accept=".gpx,application/gpx+xml,application/xml,text/xml"
        style={{ display: "none" }}
        onChange={(event) => {
          void onGpxFile(event.target.files?.[0] ?? null);
          event.target.value = "";
        }}
      />
      {gpxError && (
        <div className="gpx-toast floating-card" role="alert">
          <MaterialIcon name="warning" />
          <span>{gpxError}</span>
          <button type="button" aria-label="Fermer" onClick={() => setGpxError("")}>
            <MaterialIcon name="close" />
          </button>
        </div>
      )}
      <WeatherRunCard model={weatherModel} warning={horizonNote.warning} />
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
          id="toggle-route-points"
          className={showRoutePoints ? "map-dock-button is-active" : "map-dock-button"}
          type="button"
          aria-label="Afficher les points de la route"
          aria-pressed={showRoutePoints}
          data-tooltip="Points de la route"
          onClick={() => setShowRoutePoints((value) => !value)}
        >
          <MaterialIcon name="scatter_plot" />
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
          id="import-gpx"
          className={gpx ? "map-dock-button is-active" : "map-dock-button"}
          type="button"
          aria-label="Importer une trace GPX"
          aria-pressed={Boolean(gpx)}
          data-tooltip={
            gpx
              ? `Trace GPX : ${gpx.name}\n${gpx.count} point(s) · cliquez pour remplacer`
              : "Importer une trace GPX"
          }
          onClick={() => gpxInput.current?.click()}
        >
          <MaterialIcon name="timeline" />
          {gpx && <span className="dock-badge dock-badge-gpx">GPX</span>}
        </button>
        {gpx && (
          <button
            id="clear-gpx"
            className="map-dock-button"
            type="button"
            aria-label="Retirer la trace GPX"
            data-tooltip="Retirer la trace GPX"
            onClick={clearGpx}
          >
            <MaterialIcon name="close" />
          </button>
        )}
        <hr />
        <button
          id="refresh-weather"
          className={gribDownloading ? "map-dock-button is-busy" : "map-dock-button"}
          type="button"
          aria-label="Actualiser les GRIB"
          data-tooltip={
            gribDownloading
              ? gribProgressText || "Téléchargement des GRIB…"
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
          gameArrivalName={vrState?.arrival ? (meaningfulName(vrState.arrival.name) ?? "arrivée du jeu") : null}
          polarNote={polarNote}
          horizonNote={horizonNote}
          vrStatus={vrStatus}
          weatherStatus={
            weatherStatus ||
            gribProgressText ||
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

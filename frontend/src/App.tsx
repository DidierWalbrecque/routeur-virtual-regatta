import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { MapView, DEFAULT_DEPARTURE, DEFAULT_DESTINATION, type Endpoints, type GpxOverlay, type MapInsets, type MapViewHandle, type RouteFrame } from "./MapView.tsx";
import { CalculationStatus } from "./CalculationStatus.tsx";
import { CockpitBar } from "./CockpitBar.tsx";
import { GameModePanel } from "./GameModePanel.tsx";
import { LiveBoatCard } from "./LiveBoatCard.tsx";
import { MapLegend } from "./MapLegend.tsx";
import { RouteDetailsPanel } from "./RouteDetailsPanel.tsx";
import { polarDisplayName, RouteForm, type FormState, type VrStatus } from "./RouteForm.tsx";
import { Timeline, type TimelineReadout } from "./Timeline.tsx";
import { WeatherRunCard } from "./WeatherRunCard.tsx";
import { ZonesPanel } from "./ZonesPanel.tsx";
import { formatLocalDateTime, formatVirtualRegattaPosition, parseLocalDate, parseVirtualRegattaPosition } from "./lib/geo.ts";
import { formatDuration, GRIB_FORECAST_HOURS, horizonAllowedIn, horizonMinutes, HORIZON_OPTIONS, MAX_MINUTES_BY_MODE } from "./lib/horizon.ts";
import { readSSEStream } from "./lib/route_stream.ts";
import { DeparturePanel, useDepartureWindow, type DepartureWindowHandlers } from "./DeparturePanel.tsx";
import type { DepartureOutcome, DepartureWindowStart } from "./lib/departure_window.ts";
import type { CalculationProgress } from "./lib/calculation_progress.ts";
import { weatherSwitchMessage, type WeatherSwitch } from "./lib/weather_switch.ts";
import type { Checkpoint, Isochrone, Point, PolarOption, RouteResult, RouteStart, VrState } from "./lib/types.ts";
import { matchPolar } from "./lib/polar_match.ts";
import { parseGpx, gpxFeatureCollection, gpxPointCount } from "./lib/gpx.ts";
import { buildWeatherRunModel, nextRunEta, shortRunLabel } from "./lib/weather_run.ts";
import { friendlyRouteError } from "./lib/route_errors.ts";
import { formatAngle, formatNumber } from "./lib/route_details.ts";
import {
  clampTime,
  floorHour,
  freeRange,
  isochroneIndexAt,
  legAt,
  maneuverMarks,
  positionAt,
  routeRange,
  stepIndexAt,
  windBands,
} from "./lib/timeline.ts";
import { useLongPressTips } from "./lib/long_press_tips.ts";
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
import { EQUIPMENT, equipmentFromVrOptions, checkpointKey, checkpointName, meaningfulName, usableCheckpoints, vrStatusText, type Equipment } from "./lib/vr.ts";

const VR_POLL_INTERVAL_MS = 10_000;
const WIND_STORAGE_KEY = "routeur.showWind";
const BARBS_STORAGE_KEY = "routeur.showBarbs";
const LEGEND_STORAGE_KEY = "routeur.legendOpen";
// Panneaux du cockpit : un seul ouvert à la fois, choisi dans la colonne d'onglets.
type PanelTab = "form" | "route" | "departure" | "zones";
const RIGHT_PANEL_IDS = ["route-form-panel", "zones-panel", "route-details-panel", "departure-panel"];
// Au-delà de 3 jours de frise, le vent suit le curseur de 3 h en 3 h : la
// lecture ne demande alors qu'un champ de vent toutes les 3 h de route.
const WIND_COARSE_SPAN_MS = 72 * 3_600_000;

/** Écran étroit (téléphone) : la légende démarre fermée. */
function isNarrowScreen(): boolean {
  try {
    return window.matchMedia("(max-width: 600px)").matches;
  } catch {
    return false;
  }
}

/**
 * Place prise au-dessus de la carte par les panneaux et le coin légende, en
 * pixels depuis les bords de la carte. Sur ordinateur le panneau du cockpit est
 * à côté de la carte et ne la couvre pas ; sur téléphone il la recouvre.
 */
function mapInsets(): MapInsets {
  const map = document.getElementById("map")?.getBoundingClientRect();
  if (!map) return { right: 0, bottom: 0, left: 0 };
  let right = 0;
  // Sur téléphone, un panneau occupe toute la largeur (en bas quand il est
  // réduit) : il gêne en bas, pas à droite.
  let panelBottom = 0;
  for (const id of RIGHT_PANEL_IDS) {
    const rect = document.getElementById(id)?.getBoundingClientRect();
    if (!rect || rect.width === 0) continue;
    if (rect.left >= map.right - 1 || rect.top >= map.bottom - 1) continue;
    if (rect.left < map.left + map.width * 0.3) panelBottom = Math.max(panelBottom, map.bottom - rect.top);
    else right = Math.max(right, map.right - rect.left);
  }
  // La frise flotte en bas de la carte.
  const timeline = document.getElementById("timeline")?.getBoundingClientRect();
  const timelineBottom = timeline && timeline.height > 0 ? Math.max(0, map.bottom - timeline.top) : 0;
  const corner = document.getElementById("map-corner")?.getBoundingClientRect();
  const visible = Boolean(corner && corner.height > 0);
  const cornerBottom = visible && corner ? map.bottom - corner.top : 0;
  // Avec un panneau en bas, tout se réserve en bas (left à 0).
  if (panelBottom > 0) return { right, bottom: Math.max(panelBottom, cornerBottom, timelineBottom), left: 0 };
  return { right, bottom: Math.max(cornerBottom, timelineBottom), left: visible && corner ? corner.right - map.left : 0 };
}
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
// Dernier départ de course appliqué au formulaire : un rechargement de la page
// n'écrase pas un départ saisi à la main pendant la même course.
const RACE_START_STORAGE_KEY = "routeur.lastRaceStart";

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
type GribStatusBody = {
  files?: string[];
  run?: string;
  removedFiles?: string[];
  target?: GribTarget | null;
  busy?: boolean;
  waiting?: string;
  lastError?: string;
};
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
  // Panneau ouvert à droite de la carte, ou null : un seul à la fois.
  const [panel, setPanel] = useState<PanelTab | null>(null);
  const formOpen = panel === "form";
  const detailsOpen = panel === "route";
  const zonesOpen = panel === "zones";
  const departureOpen = panel === "departure";
  const [formError, setFormError] = useState("");
  const [polars, setPolars] = useState<PolarOption[]>([{ label: "ultim_bp_11.json", value: DEFAULT_POLAR }]);
  const [endpoints, setEndpoints] = useState<Endpoints>({ position: DEFAULT_DEPARTURE, destination: DEFAULT_DESTINATION });
  const [frame, setFrame] = useState<RouteFrame | null>(null);
  const [result, setResult] = useState<RouteResult | undefined>();
  // Juste après un calcul, le panneau s'ouvre réduit au résumé chiffré.
  const [detailsSummaryOnly, setDetailsSummaryOnly] = useState(false);
  const [calculating, setCalculating] = useState(false);
  const [progress, setProgress] = useState<CalculationProgress | null>(null);
  const [calculationStartedAt, setCalculationStartedAt] = useState(0);
  // Run récent encore partiel utilisé par le dernier calcul, et avertissement à afficher.
  const [routeWeather, setRouteWeather] = useState<WeatherSwitch | null>(null);
  const [weatherNotice, setWeatherNotice] = useState("");
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
  const [showWind, setShowWind] = useState(() => {
    try {
      return window.localStorage.getItem(WIND_STORAGE_KEY) === "true";
    } catch {
      return false;
    }
  });
  const [showBarbs, setShowBarbs] = useState(() => {
    try {
      return window.localStorage.getItem(BARBS_STORAGE_KEY) === "true";
    } catch {
      return false;
    }
  });
  const windShown = showWind || showBarbs;
  // Chaque affichage des barbules ouvre la légende sur leur lecture.
  const [legendFocusBarbs, setLegendFocusBarbs] = useState(0);
  // Frise de temps : heure du curseur (ms). Sans route, la frise part de
  // l'heure où elle a été ouverte (freeNow) et ne règle que le vent.
  const [cursor, setCursor] = useState(() => Date.now());
  const [freeNow, setFreeNow] = useState(() => Date.now());
  const [showRoutePoints, setShowRoutePoints] = useState(true);
  // Légende ouverte par défaut sur ordinateur, fermée sur téléphone ; le
  // dernier choix de l'utilisateur est ensuite conservé.
  const [legendOpen, setLegendOpen] = useState(() => {
    try {
      const saved = window.localStorage.getItem(LEGEND_STORAGE_KEY);
      if (saved !== null) return saved === "true";
    } catch {
      // Stockage indisponible : valeur par défaut selon l'écran.
    }
    return !isNarrowScreen();
  });
  // Détail du run météo, ouvert depuis la puce de la barre du cockpit.
  const [weatherOpen, setWeatherOpen] = useState(false);
  // Mode jeu : ordres d'abord, curseur de la frise sur l'heure actuelle.
  const [gameMode, setGameMode] = useState(false);
  const [followNow, setFollowNow] = useState(true);
  // Run GRIB utilisé par la route affichée (« 06Z du 22/11 »).
  const [routeRun, setRouteRun] = useState("");
  // Positions lues dans le jeu pendant la session : le trajet réel du bateau.
  const [liveTrack, setLiveTrack] = useState<{ point: Point; time: number }[]>([]);
  const [focusedZoneId, setFocusedZoneId] = useState<string | null>(null);
  const [draft, setDraft] = useState<Point[] | null>(null);
  const [gpx, setGpx] = useState<{ name: string; count: number; overlay: GpxOverlay } | null>(null);
  const [gpxError, setGpxError] = useState("");

  const mapHandle = useRef<MapViewHandle>(null);
  const startInput = useRef<HTMLInputElement>(null);
  const activeController = useRef<AbortController | null>(null);
  // Contrôleur du calcul annulé à la demande : son interruption n'est pas une erreur.
  const cancelledController = useRef<AbortController | null>(null);
  const checkpointCache = useRef(new Map<string, { start: Checkpoint["start"]; end: Checkpoint["end"] }>());
  const streamIsochrones = useRef<Isochrone[]>([]);
  const streamBase = useRef<RouteResult | null>(null);
  const frameCounter = useRef(0);
  const gpxInput = useRef<HTMLInputElement>(null);
  const gpxCounter = useRef(0);
  const tools = useRef<HTMLDivElement>(null);
  useLongPressTips(tools, ".map-tool-button");
  // Menu de la trace GPX, sous l'onglet Zones : se ferme par Échap ou par un clic ailleurs.
  const [gpxMenuOpen, setGpxMenuOpen] = useState(false);
  const gpxMenuAnchor = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!gpxMenuOpen) return;
    const onPointerDown = (event: PointerEvent) => {
      if (!gpxMenuAnchor.current?.contains(event.target as Node)) setGpxMenuOpen(false);
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      setGpxMenuOpen(false);
      document.getElementById("import-gpx")?.focus();
    };
    document.addEventListener("pointerdown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("pointerdown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [gpxMenuOpen]);
  // Menu « Calques » : se ferme par Échap ou par un clic ailleurs.
  const [layersOpen, setLayersOpen] = useState(false);
  useEffect(() => {
    if (!layersOpen) return;
    const onPointerDown = (event: PointerEvent) => {
      if (!tools.current?.contains(event.target as Node)) setLayersOpen(false);
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      setLayersOpen(false);
      document.getElementById("map-layers")?.focus();
    };
    document.addEventListener("pointerdown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("pointerdown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [layersOpen]);

  const weatherModel = useMemo(
    () => (weather.run ? buildWeatherRunModel(weather.run, weather.files) : null),
    [weather],
  );

  const nextRun = useMemo(
    () =>
      gribTarget && !gribTarget.complete && gribTarget.downloaded > 0
        ? { label: shortRunLabel(gribTarget.run), downloaded: gribTarget.downloaded, expected: gribTarget.expected }
        : null,
    [gribTarget],
  );
  const upcomingRun = useMemo(() => (weather.run ? nextRunEta(weather.run) : null), [weather.run]);
  const gribProgressText = nextRun
    ? `Run ${nextRun.label} en cours de publication : ${nextRun.downloaded}/${nextRun.expected} échéances`
    : "";

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
      window.localStorage.setItem(WIND_STORAGE_KEY, String(showWind));
    } catch {
      // Stockage indisponible : le choix reste valable pour la session.
    }
  }, [showWind]);

  useEffect(() => {
    try {
      window.localStorage.setItem(BARBS_STORAGE_KEY, String(showBarbs));
    } catch {
      // Stockage indisponible : le choix reste valable pour la session.
    }
  }, [showBarbs]);

  // Sans route, le curseur repart de l'heure en cours à l'affichage du vent.
  useEffect(() => {
    if (windShown) setFreeNow(Date.now());
  }, [windShown]);

  useEffect(() => {
    try {
      window.localStorage.setItem(LEGEND_STORAGE_KEY, String(legendOpen));
    } catch {
      // Stockage indisponible : le choix reste valable pour la session.
    }
  }, [legendOpen]);

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
    setPanel((current) => (current === "route" ? null : current));
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
  // et après une modification indirecte (point de porte, saisie). Un départ
  // recopié depuis la position du bateau dans le jeu déplace seulement le
  // marqueur : la route affichée n'est effacée que sur action de l'utilisateur.
  const vrStartText = useRef("");
  useEffect(() => {
    if (form.start && form.start === vrStartText.current) {
      const position = parseVirtualRegattaPosition(form.start);
      if (position) setEndpoints((previous) => ({ ...previous, position }));
      return;
    }
    updateEndpoints(form.start, form.destination);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [form.start, updateEndpoints]);
  useEffect(() => {
    updateEndpoints(form.start, form.destination);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [form.destination, updateEndpoints]);

  // L'arrivée officielle du jeu remplace la destination dès qu'elle change (nouveau leg, première lecture).
  const gameArrivalKey = vrState?.arrival ? `${vrState.arrival.latitude},${vrState.arrival.longitude}` : "";
  useEffect(() => {
    if (!vrState?.arrival) return;
    const text = formatVirtualRegattaPosition({ latitude: vrState.arrival.latitude, longitude: vrState.arrival.longitude });
    setForm((previous) => (previous.destination === text ? previous : { ...previous, destination: text, checkpoint: "" }));
    console.info(`[VR] Arrivée du jeu : ${vrState.arrival.name || "sans nom"}`);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [gameArrivalKey]);

  // Le départ officiel du jeu remplace le départ saisi à la main quand la course
  // change (nouveau départ). En mode Virtual Regatta, la position du bateau
  // fait déjà foi.
  const gameStartKey = vrState?.legStart ? `${vrState.legStart.latitude},${vrState.legStart.longitude}` : "";
  useEffect(() => {
    const legStart = vrState?.legStart;
    if (!legStart || !gameStartKey) return;
    let applied = "";
    try {
      applied = window.localStorage.getItem(RACE_START_STORAGE_KEY) ?? "";
      window.localStorage.setItem(RACE_START_STORAGE_KEY, gameStartKey);
    } catch {
      // Stockage indisponible : le départ de la course s'applique à chaque lecture.
    }
    if (applied === gameStartKey) return;
    const text = formatVirtualRegattaPosition({ latitude: legStart.latitude, longitude: legStart.longitude });
    setForm((previous) => (previous.positionSource === "vr" || previous.start === text ? previous : { ...previous, start: text }));
    console.info(`[VR] Départ de la course : ${legStart.name || "sans nom"}`);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [gameStartKey]);

  // En mode Virtual Regatta, le marqueur de calcul suit la position live du bateau.
  const boatLatitude = vrState?.position?.latitude;
  const boatLongitude = vrState?.position?.longitude;
  useEffect(() => {
    if (form.positionSource !== "vr" || boatLatitude === undefined || boatLongitude === undefined) return;
    setEndpoints((previous) => ({ ...previous, position: { latitude: boatLatitude, longitude: boatLongitude } }));
  }, [form.positionSource, boatLatitude, boatLongitude]);

  // --- Zones interdites (actions de l'utilisateur : la route affichée devient périmée) ---
  const startDrawing = () => {
    setPanel(null);
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
    setPanel("zones");
    setDraft(null);
  }, [resetRoute]);
  const cancelDrawing = useCallback(() => {
    setDraft(null);
    setPanel("zones");
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
      if (position) vrStartText.current = position;
      // Mise à jour silencieuse : la route affichée n'est pas effacée. Avant le
      // départ d'un record, les options du joueur sont inconnues : les
      // équipements choisis dans le formulaire sont conservés.
      setForm((previous) =>
        previous.positionSource === "vr"
          ? {
              ...previous,
              start: position || previous.start,
              equipment: state.preStart ? previous.equipment : equipmentFromVrOptions(state.options),
            }
          : previous,
      );
      setVrStatus({
        ok: true,
        text: vrStatusText(state),
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

  // La position du bateau est relue en continu tant que l'onglet est visible,
  // et tout de suite au retour sur l'onglet : un rafraîchissement fait dans le
  // jeu met ainsi à jour le départ sans ouvrir le formulaire.
  useEffect(() => {
    const timer = setInterval(() => {
      if (!document.hidden) void refreshVrState();
    }, VR_POLL_INTERVAL_MS);
    const onVisible = () => {
      if (!document.hidden) void refreshVrState();
    };
    document.addEventListener("visibilitychange", onVisible);
    window.addEventListener("focus", onVisible);
    return () => {
      clearInterval(timer);
      document.removeEventListener("visibilitychange", onVisible);
      window.removeEventListener("focus", onVisible);
    };
  }, [refreshVrState]);

  useEffect(() => {
    if (formOpen || zonesOpen) void refreshVrState();
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
      if (!body.run || !body.files?.length) {
        // Aucun run complet : ne pas laisser croire que l'actualisation a réussi.
        const reason = body.lastError || body.waiting || "aucun run GFS complet disponible";
        setWeatherStatus(`Aucun GRIB utilisable : ${reason}`);
        console.warn(`[GRIB] Actualisation sans run complet : ${reason}`);
        return;
      }
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

  // --- Meilleure heure de départ d'un record ---
  const compareDepartures = async (
    comparisonHours: number,
    stepHours: number,
    owned: Equipment[],
    handlers: DepartureWindowHandlers,
    signal: AbortSignal,
  ) => {
    let gribFiles = weather.files;
    if (gribFiles.length === 0) {
      const response = await fetch("/api/weather/current", { cache: "no-store" });
      const body = await readJsonResponse<GribStatusBody>(response);
      applyGribStatus(body);
      gribFiles = body.files || [];
    }
    if (gribFiles.length === 0) throw new Error("Aucun run GRIB disponible : actualisez les GRIB.");
    console.info(
      `[DÉPART] Comparaison demandée : ${comparisonHours} h de route, un départ toutes les ${stepHours} h, options : ${owned.join(", ") || "aucune"}`,
    );
    const response = await fetch("/api/departure-window/stream", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        weatherFiles: gribFiles,
        polarPath: form.polar,
        destination: { virtualRegatta: form.destination },
        owned,
        manualStamina: form.staminaSource === "manual" ? Number(form.staminaManual) : null,
        coastalMarginNM: Number(form.coastalMarginNM),
        arrivalRadiusNM: vrState?.arrival?.radius ?? null,
        exclusionZones: zonesForRequest(allZones),
        comparisonHours,
        stepHours,
      }),
      signal,
    });
    await readSSEStream(response, (eventName, data) => {
      if (eventName === "start") handlers.onStart(data as DepartureWindowStart);
      if (eventName === "departure") handlers.onOutcome(data as DepartureOutcome);
      if (eventName === "complete") console.info("[DÉPART] Comparaison terminée");
      if (eventName === "cancelled") throw new Error((data as { error?: string }).error || "Comparaison annulée.");
      if (eventName === "error") throw new Error((data as { error?: string }).error || "La comparaison a échoué.");
    });
  };

  // À la fin de la comparaison (ou à son arrêt), le tableau des résultats s'ouvre.
  const formPolarName = polarDisplayName(polars.find((polar) => polar.value === form.polar)?.label ?? form.polar.split(/[\\/]/).pop() ?? "");
  const departureWindow = useDepartureWindow(
    compareDepartures,
    () => setPanel("departure"),
    formPolarName,
  );

  // --- Calcul de la route en flux ---
  // `overrides` : recalcul depuis la position du jeu, à l'heure actuelle (mode jeu).
  const submit = async (overrides: { instant?: string; start?: string } = {}) => {
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
    setRouteRun(shortRunLabel(gribRun));
    console.info(
      `[GRIB] ${gribFiles.length} fichier(s) envoyé(s) au calcul : ${gribFiles[0]?.split(/[\\/]/).pop() ?? "aucun"} → ${gribFiles[gribFiles.length - 1]?.split(/[\\/]/).pop() ?? "aucun"}`,
    );
    try {
      const instant = parseLocalDate(overrides.instant ?? form.instant);
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
      setProgress(null);
      setCalculationStartedAt(Date.now());
      setRouteWeather(null);
      setWeatherNotice("");
      const response = await fetch("/api/practical-headings/stream", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          weatherFiles: gribFiles,
          polarPath: form.polar,
          position: { virtualRegatta: overrides.start ?? form.start },
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
          if (start.totalLayers) setProgress({ completed: 0, total: start.totalLayers });
          setRouteWeather(start.weather ?? null);
          if (start.weather) console.info(`[GRIB] ${weatherSwitchMessage(start.weather)}`);
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
          if (frontier.totalLayers) setProgress({ completed: frontier.completedLayers ?? 0, total: frontier.totalLayers });
          return;
        }
        if (eventName === "complete") {
          const complete = data as RouteResult;
          setResult(complete);
          if (complete.weather?.affectsRoute) setWeatherNotice(weatherSwitchMessage(complete.weather));
          setFrame({ result: complete, showRoute: true, fit: true, key: String(frameCounter.current) });
          if ((complete.route?.length ?? 0) > 0) {
            // Sur téléphone, le panneau s'ouvre réduit au résumé pour laisser voir la carte.
            setDetailsSummaryOnly(isNarrowScreen());
            setPanel("route");
          }
          return;
        }
        if (eventName === "cancelled") throw new Error((data as { error?: string }).error || "Calcul annulé.");
        if (eventName === "error") throw new Error((data as { error?: string }).error || "Le calcul a échoué.");
      });
      setPanel((current) => (current === "form" ? null : current));
      console.info("[ROUTE] Flux de calcul terminé");
    } catch (error) {
      if (cancelledController.current === controller) {
        // Annulation demandée : les isochrones déjà calculées restent affichées.
        cancelledController.current = null;
        setFormError("Routage annulé. Les isochrones déjà calculées restent affichées.");
        console.info("[ROUTE] Calcul annulé à la demande");
      } else if (!(controller.signal.aborted && activeController.current !== controller)) {
        setFormError(
          friendlyRouteError((error as Error).message || "", {
            coastalMarginNM: Number(form.coastalMarginNM) || 0,
            enabledZoneCount,
          }),
        );
        console.error("[ROUTE] Échec du calcul", error);
      }
    } finally {
      if (activeController.current === controller) {
        activeController.current = null;
        setCalculating(false);
      }
    }
  };

  // Annule le calcul en cours : le serveur arrête la recherche quand la connexion se ferme.
  const cancelCalculation = useCallback(() => {
    const controller = activeController.current;
    if (!controller) return;
    cancelledController.current = controller;
    controller.abort();
  }, []);

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

  const closePanel = useCallback(() => setPanel(null), []);
  const toggleGameZones = (targets: Zone[], enabled: boolean) => {
    setDisabledGameZones((previous) => {
      const next = new Set(previous);
      for (const zone of targets) {
        if (enabled) next.delete(zone.id);
        else next.add(zone.id);
      }
      return next;
    });
    resetRoute();
  };
  const focusZone = (zone: Zone) => {
    setFocusedZoneId((current) => (current === zone.id ? null : zone.id));
    if (focusedZoneId !== zone.id) mapHandle.current?.focusPoints(zone.points);
  };
  useEffect(() => {
    if (!zonesOpen) setFocusedZoneId(null);
  }, [zonesOpen]);
  const openForm = () => setPanel("form");
  const togglePanel = (tab: PanelTab) => setPanel((current) => (current === tab ? null : tab));
  // Le panneau des départs n'existe qu'avant le départ d'un record.
  useEffect(() => {
    if (!vrState?.preStart && !departureWindow.running) setPanel((current) => (current === "departure" ? null : current));
  }, [vrState?.preStart, departureWindow.running]);
  // Ligne choisie dans le tableau : la carte se recentre dessus si besoin.
  const [selectedRow, setSelectedRow] = useState<{ index: number; point: Point } | null>(null);
  useEffect(() => setSelectedRow(null), [result]);
  const hasRoute = Boolean(result && result.route.length > 0);

  // --- Frise de temps (bateau, isochrone et vent à l'heure du curseur) ---
  const route = useMemo(() => (hasRoute && result ? result.route : []), [hasRoute, result]);
  const timelineRange = useMemo(() => routeRange(route) ?? freeRange(freeNow), [route, freeNow]);
  // Nouvelle route (même avec la même heure de départ) : le curseur revient au départ.
  useEffect(() => {
    setCursor(timelineRange.start);
  }, [result, timelineRange.start]);
  const cursorTime = clampTime(timelineRange, cursor);
  const timelineRoute = route.length > 0 && !calculating;
  const stepIndex = timelineRoute ? stepIndexAt(route, cursorTime) : -1;
  const timelineBoat = useMemo(() => (timelineRoute ? positionAt(route, cursorTime) : null), [timelineRoute, route, cursorTime]);
  const timelineIsochrone = timelineRoute && result ? isochroneIndexAt(result.isochrones ?? [], cursorTime) : -1;
  const timelineMarks = useMemo(() => (route.length ? maneuverMarks(route, timelineRange) : []), [route, timelineRange]);
  const timelineBands = useMemo(() => (route.length ? windBands(route, timelineRange) : []), [route, timelineRange]);
  const timelineStepMinutes = useMemo(() => {
    const step = route.length > 1 ? (Date.parse(route[1].instant) - Date.parse(route[0].instant)) / 60_000 : 60;
    return Number.isFinite(step) && step > 0 ? step : 60;
  }, [route]);
  const readout = useMemo<TimelineReadout | null>(() => {
    if (stepIndex < 0) return null;
    // Réglage du pas en cours, porté par le point où il se termine.
    const point = legAt(route, stepIndex);
    if (!point) return null;
    const twa = point.twa === undefined ? null : point.twa;
    return {
      twa: twa === null ? "—" : `${formatAngle(Math.abs(twa))}°`,
      tws: formatNumber(point.tws ?? null, 1),
      twsKnots: point.tws !== undefined && Number.isFinite(point.tws) ? point.tws : null,
      sail: point.sail || "—",
    };
  }, [route, stepIndex]);
  const windHourMs = timelineRange.end - timelineRange.start > WIND_COARSE_SPAN_MS ? 3 * 3_600_000 : 3_600_000;
  const windTime = windShown ? new Date(Math.floor(floorHour(cursorTime) / windHourMs) * windHourMs).toISOString() : null;
  const moveCursor = useCallback((time: number) => {
    setSelectedRow(null);
    setFollowNow(false);
    setCursor(time);
  }, []);

  // --- Mode jeu ---
  // Sans route (hors calcul en cours), il n'y a rien à suivre : retour au routage.
  useEffect(() => {
    if (gameMode && !hasRoute && !calculating) setGameMode(false);
  }, [gameMode, hasRoute, calculating]);
  // En mode jeu, le curseur suit l'heure actuelle tant qu'on ne le déplace pas.
  useEffect(() => {
    if (!gameMode || !followNow) return;
    const follow = () => setCursor(Date.now());
    follow();
    const timer = window.setInterval(follow, 10_000);
    return () => window.clearInterval(timer);
  }, [gameMode, followNow, timelineRange.start]);
  // Trajet réel : chaque nouvelle position lue dans le jeu (hors avant-départ).
  const liveLatitude = vrState?.preStart ? undefined : vrState?.position?.latitude;
  const liveLongitude = vrState?.preStart ? undefined : vrState?.position?.longitude;
  const liveTime = vrState?.updatedAt;
  useEffect(() => {
    if (liveLatitude === undefined || liveLongitude === undefined) return;
    const time = Number(liveTime) || Date.now();
    setLiveTrack((track) => {
      const last = track.at(-1);
      if (last && last.point.latitude === liveLatitude && last.point.longitude === liveLongitude) return track;
      return [...track.slice(-1999), { point: { latitude: liveLatitude, longitude: liveLongitude }, time }];
    });
  }, [liveLatitude, liveLongitude, liveTime]);
  const visibleTrack = useMemo(
    () => (gameMode ? liveTrack.filter((item) => item.time >= timelineRange.start).map((item) => item.point) : []),
    [gameMode, liveTrack, timelineRange.start],
  );
  const recalculateFromBoat = () => {
    const position = formatVirtualRegattaPosition(vrState?.position);
    if (!position) return;
    const instant = formatLocalDateTime(new Date());
    setForm((previous) => ({ ...previous, instant }));
    console.info(`[JEU] Recalcul depuis la position du jeu : ${position}`);
    void submit({ instant, start: position });
  };

  const boatName = polarMatch?.polar.boat || formPolarName || "Bateau";
  // Le formulaire de calcul s'ouvre depuis le bouton « Calculer une route » de la barre du haut.
  const tabs: { id: PanelTab; label: string; icon: string; disabled?: boolean; badge?: number }[] = [
    { id: "route", label: "Route", icon: "route", disabled: !hasRoute },
    ...(vrState?.preStart || departureWindow.running ? [{ id: "departure" as const, label: "Départ", icon: "schedule" }] : []),
    { id: "zones", label: "Zones", icon: "block", badge: enabledZoneCount },
  ];

  return (
    <div className="cockpit">
      <CockpitBar
        boatName={boatName}
        vrState={vrState}
        vrStatus={vrStatus}
        weatherModel={weatherModel}
        runLabel={weather.run ? shortRunLabel(weather.run) : ""}
        nextRun={nextRun}
        nextRunEta={upcomingRun}
        weatherWarning={horizonNote.warning}
        weatherOpen={weatherOpen}
        onToggleWeather={() => setWeatherOpen((open) => !open)}
        gribDownloading={gribDownloading}
        gribTooltip={
          gribDownloading
            ? gribProgressText || "Téléchargement des GRIB…"
            : weatherModel
              ? `Actualiser les GRIB\nRun ${weatherModel.runText} · +${weatherModel.horizonHours} h`
              : "Actualiser les GRIB"
        }
        refreshing={refreshing}
        onRefreshWeather={() => void refreshWeather()}
        calculating={calculating}
        formOpen={formOpen && !gameMode}
        onCalculate={() => {
          setGameMode(false);
          togglePanel("form");
        }}
      />
      {weatherOpen && weatherModel && (
        <div className="cockpit-weather-popover">
          <WeatherRunCard
            model={weatherModel}
            warning={horizonNote.warning}
            nextRun={nextRun}
            nextRunEta={upcomingRun}
            onClose={() => setWeatherOpen(false)}
          />
        </div>
      )}
      <div className={gameMode ? "cockpit-main is-game" : "cockpit-main"}>
        {gameMode &&
          (result && hasRoute ? (
            <GameModePanel
              result={result}
              runLabel={routeRun}
              followNow={followNow}
              onFollowNow={() => setFollowNow(true)}
              onSelect={(order) => {
                const time = Date.parse(order.instant);
                if (Number.isFinite(time)) moveCursor(time);
                if (order.latitude !== null && order.longitude !== null) {
                  setSelectedRow({ index: order.index, point: { latitude: order.latitude, longitude: order.longitude } });
                }
              }}
              onExit={() => setGameMode(false)}
            />
          ) : (
            <section className="game-panel game-panel-waiting" aria-live="polite">
              <p>Nouvelle route en cours de calcul…</p>
            </section>
          ))}
        <div className="cockpit-map">
          <MapView
            ref={mapHandle}
            endpoints={endpoints}
            raceStart={vrState?.legStart ? { latitude: vrState.legStart.latitude, longitude: vrState.legStart.longitude } : null}
            gameArrival={vrState?.arrival ? { latitude: vrState.arrival.latitude, longitude: vrState.arrival.longitude } : null}
            arrivalRadiusNM={vrState?.arrival?.radius ?? null}
            frame={frame}
            vrState={vrState}
            zones={allZones}
            highlight={selectedRow?.point ?? null}
            draft={draft}
            gpx={gpx?.overlay ?? null}
            onDraftClick={onDraftClick}
            showRoutePoints={showRoutePoints}
            windTime={windTime}
            windColors={showWind}
            windBarbs={showBarbs}
            timelineBoat={timelineBoat}
            timelineIsochrone={timelineIsochrone}
            liveTrack={visibleTrack}
            focusedZoneId={focusedZoneId}
            insets={mapInsets}
            onError={() => setMapError(true)}
          />
          {departureWindow.running && (
            <CalculationStatus
              id="departure-calculation-status"
              title="Comparaison des départs"
              unitLabel="Départ"
              preparingText="Préparation : départs possibles, météo et portes…"
              cancelLabel="Arrêter la comparaison"
              progress={departureWindow.progress}
              startedAt={departureWindow.startedAt}
              onCancel={departureWindow.cancel}
              cancelOnEscape={!calculating}
              stacked={calculating}
            />
          )}
          {calculating && (
            <CalculationStatus
              progress={progress}
              startedAt={calculationStartedAt}
              onCancel={cancelCalculation}
              weatherNote={routeWeather ? weatherSwitchMessage(routeWeather) : ""}
              weatherWarning={routeWeather?.affectsRoute ?? false}
            />
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
          {weatherNotice && !calculating && (
            <div className="gpx-toast weather-toast floating-card" role="status">
              <MaterialIcon name="cloud_sync" />
              <span>{weatherNotice}</span>
              <button type="button" aria-label="Fermer" onClick={() => setWeatherNotice("")}>
                <MaterialIcon name="close" />
              </button>
            </div>
          )}
          {(gpxError || weatherStatus) && (
            <div className="gpx-toast floating-card" role="alert">
              <MaterialIcon name="warning" />
              <span>{gpxError || weatherStatus}</span>
              <button
                type="button"
                aria-label="Fermer"
                onClick={() => {
                  setGpxError("");
                  setWeatherStatus("");
                }}
              >
                <MaterialIcon name="close" />
              </button>
            </div>
          )}
          {gameMode && (
            <LiveBoatCard vrState={vrState} route={route} calculating={calculating} onRecalculate={recalculateFromBoat} />
          )}
          {legendOpen && (
            <div id="map-corner" className="map-corner">
              <MapLegend
                onClose={() => setLegendOpen(false)}
                south={(vrState?.position?.latitude ?? endpoints.position.latitude) < 0}
                focusBarbs={legendFocusBarbs}
              />
            </div>
          )}
          <div ref={tools} className="map-tools" role="toolbar" aria-label="Affichage de la carte" aria-orientation="vertical">
            <button
              id="map-layers"
              className={layersOpen ? "map-tool-button is-active" : "map-tool-button"}
              type="button"
              aria-label="Calques de la carte"
              aria-expanded={layersOpen}
              aria-controls="map-layers-menu"
              data-tooltip={layersOpen ? "" : "Calques : points, vent, barbules, légende"}
              onClick={() => setLayersOpen((open) => !open)}
            >
              <MaterialIcon name="layers" />
            </button>
            <button
              id="recenter-map"
              className="map-tool-button"
              type="button"
              aria-label="Recentrer la carte"
              data-tooltip="Recentrer la carte"
              onClick={() => mapHandle.current?.recenter()}
            >
              <MaterialIcon name="my_location" />
            </button>
            {layersOpen && (
              <div id="map-layers-menu" className="map-layers-menu" role="group" aria-label="Calques de la carte">
                <p className="map-layers-title">Calques</p>
                <button
                  id="toggle-route-points"
                  className="map-layer-row"
                  type="button"
                  aria-pressed={showRoutePoints}
                  onClick={() => setShowRoutePoints((value) => !value)}
                >
                  <MaterialIcon name="scatter_plot" />
                  <span>Points de la route</span>
                  <i className="map-layer-switch" aria-hidden="true" />
                </button>
                <button
                  id="toggle-wind"
                  className="map-layer-row"
                  type="button"
                  aria-pressed={showWind}
                  onClick={() => setShowWind((value) => !value)}
                >
                  <MaterialIcon name="air" />
                  <span>
                    Vent
                    <small>échelle de Beaufort</small>
                  </span>
                  <i className="map-layer-switch" aria-hidden="true" />
                </button>
                <button
                  id="toggle-barbs"
                  className="map-layer-row"
                  type="button"
                  aria-pressed={showBarbs}
                  onClick={() => {
                    // À l'affichage, la légende s'ouvre pour expliquer la lecture des barbules.
                    if (!showBarbs) {
                      setLegendOpen(true);
                      setLegendFocusBarbs((count) => count + 1);
                    }
                    setShowBarbs((value) => !value);
                  }}
                >
                  <MaterialIcon name="flag" />
                  <span>
                    Barbules
                    <small>direction et force du vent</small>
                  </span>
                  <i className="map-layer-switch" aria-hidden="true" />
                </button>
                <button
                  id="toggle-legend"
                  className="map-layer-row"
                  type="button"
                  aria-pressed={legendOpen}
                  aria-controls="map-legend"
                  onClick={() => setLegendOpen((open) => !open)}
                >
                  <MaterialIcon name="info" />
                  <span>Légende et balisage</span>
                  <i className="map-layer-switch" aria-hidden="true" />
                </button>
              </div>
            )}
          </div>
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
          <Timeline
            range={timelineRange}
            cursor={cursorTime}
            onCursor={moveCursor}
            stepMinutes={timelineRoute ? timelineStepMinutes : 60}
            hasRoute={timelineRoute}
            disabled={calculating}
            marks={timelineRoute ? timelineMarks : []}
            bands={timelineRoute ? timelineBands : []}
            readout={readout}
            windShown={windShown}
            onShowWind={() => setShowWind(true)}
          />
        </div>
        {panel && !gameMode && (
          <div className="cockpit-side">
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
                onClose={closePanel}
                startInput={startInput}
              />
            )}
            {detailsOpen && result && (
              <RouteDetailsPanel
                result={result}
                selectedIndex={stepIndex >= 0 ? stepIndex : null}
                onSelect={(index, point) => {
                  const instant = Date.parse(route[index]?.instant ?? "");
                  if (Number.isFinite(instant)) setCursor(instant);
                  setSelectedRow({ index, point });
                }}
                onClose={closePanel}
                initiallyCollapsed={detailsSummaryOnly}
                runLabel={routeRun}
                onPlay={() => {
                  setFollowNow(true);
                  setGameMode(true);
                }}
              />
            )}
            {departureOpen && (
              <DeparturePanel
                model={departureWindow}
                gateNames={checkpoints.map(checkpointName)}
                polar={formPolarName}
                onClose={closePanel}
                onUse={(departure, owned) => {
                  // La route se calcule avec le même bateau que la comparaison.
                  setField("instant", formatLocalDateTime(departure));
                  setField("equipment", new Set(owned));
                  console.info(`[DÉPART] Départ retenu : ${departure.toISOString()}, options : ${owned.join(", ") || "aucune"}`);
                  openForm();
                }}
              />
            )}
            {zonesOpen && (
              <ZonesPanel
                zones={allZones}
                game={{ stateKnown: vrState !== undefined, count: gameZones.length, legShape: vrState?.legShape || [] }}
                boat={endpoints.position}
                focusedId={focusedZoneId}
                onDraw={startDrawing}
                onFocus={focusZone}
                onToggleAll={toggleGameZones}
                onToggle={(zone, enabled) => {
                  if (zone.source === "game") {
                    toggleGameZones([zone], enabled);
                  } else {
                    updateZones((previous) => previous.map((item) => (item.id === zone.id ? { ...item, enabled } : item)));
                  }
                }}
                onRename={(id, name) => setZones((previous) => previous.map((zone) => (zone.id === id ? { ...zone, name } : zone)))}
                onDelete={(id) => updateZones((previous) => previous.filter((zone) => zone.id !== id))}
                onClose={closePanel}
              />
            )}
          </div>
        )}
        <nav className="cockpit-rail" aria-label="Panneaux" hidden={gameMode}>
          {tabs.map((tab) => (
            <button
              key={tab.id}
              id={`cockpit-tab-${tab.id}`}
              type="button"
              className={panel === tab.id ? "cockpit-tab is-active" : "cockpit-tab"}
              aria-pressed={panel === tab.id}
              disabled={tab.disabled}
              title={tab.disabled ? "Calculez d'abord une route" : undefined}
              onClick={() => {
                if (tab.id === "route") setDetailsSummaryOnly(false);
                togglePanel(tab.id);
              }}
            >
              <MaterialIcon name={tab.icon} />
              <span>{tab.label}</span>
              {tab.badge ? <span className="dock-badge">{tab.badge}</span> : null}
            </button>
          ))}
          <div ref={gpxMenuAnchor} className="cockpit-tab-anchor">
            <button
              id="import-gpx"
              type="button"
              className={gpxMenuOpen ? "cockpit-tab is-active" : "cockpit-tab"}
              aria-expanded={gpx ? gpxMenuOpen : undefined}
              aria-controls={gpx ? "gpx-menu" : undefined}
              title={gpx ? `Trace GPX : ${gpx.name} · ${gpx.count} point(s)` : "Importer une trace GPX"}
              onClick={() => (gpx ? setGpxMenuOpen((open) => !open) : gpxInput.current?.click())}
            >
              <MaterialIcon name="timeline" />
              <span>GPX</span>
              {gpx && <span className="cockpit-tab-dot" aria-hidden="true" />}
            </button>
            {gpx && gpxMenuOpen && (
              <div id="gpx-menu" className="gpx-menu" role="group" aria-label="Trace GPX">
                <p className="gpx-menu-name">
                  <strong>{gpx.name}</strong>
                  <small>{`${gpx.count} point(s)`}</small>
                </p>
                <button
                  id="replace-gpx"
                  className="map-layer-row"
                  type="button"
                  onClick={() => {
                    setGpxMenuOpen(false);
                    gpxInput.current?.click();
                  }}
                >
                  <MaterialIcon name="upload_file" />
                  <span>Remplacer la trace</span>
                </button>
                <button
                  id="clear-gpx"
                  className="map-layer-row"
                  type="button"
                  onClick={() => {
                    setGpxMenuOpen(false);
                    clearGpx();
                  }}
                >
                  <MaterialIcon name="delete" />
                  <span>Retirer la trace</span>
                </button>
              </div>
            )}
          </div>
        </nav>
      </div>
    </div>
  );
}

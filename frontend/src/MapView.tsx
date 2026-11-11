import { forwardRef, useEffect, useImperativeHandle, useRef, useState } from "react";
import type { GeoJSONSource, ImageSource, LngLatBounds, Map as MapLibreMap, MapMouseEvent, Marker } from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import { approximateDistanceNM, circlePolygon, cutContourAtBreaks, forwardContour, isochroneDisplayInterval, isochroneIntervals, medianAdvanceNM, splitContour, trimStalledContour } from "./lib/geo.ts";
import type { Isochrone, Point, RouteResult } from "./lib/types.ts";
import { checkpointFeatures, checkpointKey, checkpointName, usableCheckpoints } from "./lib/vr.ts";
import { gateRingSvg, gateRounding, gateRoundingLabel } from "./lib/gate_marker.ts";
import { draftFeatureCollection, zonesFeatureCollection, type Zone } from "./lib/zones.ts";
import type { VrState } from "./lib/types.ts";
import { windFieldPixels, type WindField } from "./lib/beaufort.ts";

export const DEFAULT_DEPARTURE: Point = { latitude: 13.121111, longitude: 121.586667 };
export const DEFAULT_DESTINATION: Point = { latitude: 13.171111, longitude: 121.336667 };

const ISOCHRONE_FACING_ANGLE = 110;
// Part de l'avancée médiane en dessous de laquelle un bout de tracé est jugé
// collé au tracé affiché précédent, et retiré.
const ISOCHRONE_MIN_ADVANCE_SHARE = 0.35;

// Fond de carte marine assombri (carte de nuit), et sa version détaillée.
const BASEMAP_PAINT = { "raster-brightness-max": 0.32, "raster-saturation": -0.25, "raster-contrast": 0.15 };
// Côtes vectorielles, au-delà du zoom 10 : terre nettement plus claire que la
// mer pour distinguer les îles sur le fond de nuit.
const COASTLINE_SOURCE = "coastline";
const LAND_COLOR = "#4a4f57";
const WATER_COLOR = "#1f2a38";
const COASTLINE_FADE = ["interpolate", ["linear"], ["zoom"], 9.5, 0, 10.5, 1] as never;

// Couleur d'une isochrone selon l'avancement temporel : bleu (tôt) -> vert -> ambre (tard).
// Teintes claires, pour rester lisibles sur la carte marine sombre.
function lerpHex(a: string, b: string, t: number): string {
  const pa = [1, 3, 5].map((i) => parseInt(a.slice(i, i + 2), 16));
  const pb = [1, 3, 5].map((i) => parseInt(b.slice(i, i + 2), 16));
  const p = pa.map((v, i) => Math.round(v + (pb[i] - v) * t));
  return `rgb(${p[0]}, ${p[1]}, ${p[2]})`;
}
function isochroneTimeColor(fraction: number): string {
  const f = Math.max(0, Math.min(1, fraction));
  return f < 0.5 ? lerpHex("#4dabf7", "#38d9a9", f * 2) : lerpHex("#38d9a9", "#ffd43b", (f - 0.5) * 2);
}

// Grille des latitudes et longitudes, comme sur une carte marine : un trait
// par degré, plus marqué tous les 10 degrés.
function graticule() {
  const features = [];
  for (let longitude = -180; longitude <= 180; longitude += 1) {
    features.push({
      type: "Feature",
      properties: { major: longitude % 10 === 0 },
      geometry: { type: "LineString", coordinates: [[longitude, -85], [longitude, 85]] },
    });
  }
  for (let latitude = -80; latitude <= 80; latitude += 1) {
    features.push({
      type: "Feature",
      properties: { major: latitude % 10 === 0 },
      geometry: { type: "LineString", coordinates: [[-180, latitude], [180, latitude]] },
    });
  }
  return { type: "FeatureCollection", features };
}

export type Endpoints = { position: Point; destination: Point };

/** Ce que la carte doit afficher pour la route : un résultat partiel ou final. */
export type RouteFrame = {
  result: RouteResult;
  showRoute: boolean;
  fit: boolean;
  /** Change à chaque nouveau calcul pour repartir de zéro côté isochrones. */
  key: string;
};

export type MapViewHandle = {
  recenter: () => void;
  /** Cadre la carte sur ces points (une zone choisie dans la liste, par exemple). */
  focusPoints: (points: Point[]) => void;
};

/**
 * Place occupée par les panneaux au-dessus de la carte, en pixels depuis chaque
 * bord : à droite les panneaux, en bas à gauche la colonne légende + météo
 * (sa hauteur depuis le bas et sa largeur depuis la gauche).
 */
export type MapInsets = { right: number; bottom: number; left: number };

// Commandes MapLibre en français (infobulles et lecteurs d'écran).
const MAP_LOCALE = {
  "AttributionControl.ToggleAttribution": "Afficher les sources",
  "AttributionControl.MapFeedback": "Signaler une erreur de carte",
  "NavigationControl.ResetBearing": "Remettre le nord en haut (glisser pour tourner)",
  "NavigationControl.ZoomIn": "Zoomer",
  "NavigationControl.ZoomOut": "Dézoomer",
  "Map.Title": "Carte",
  "Marker.Title": "Repère",
};

const MAP_MARGIN = 60;

/** Trace GPX importée à afficher en référence sur la carte. */
export type GpxOverlay = {
  /** FeatureCollection GeoJSON (LineString pour les traces, Point pour les repères). */
  collection: FeatureCollection;
  /** Change à chaque import pour recadrer la carte sur la nouvelle trace. */
  key: string;
};

type MapLibreModule = typeof import("maplibre-gl");
type FeatureCollection = { type: "FeatureCollection"; features: unknown[] };

const EMPTY: FeatureCollection = { type: "FeatureCollection", features: [] };

function endpointCollection(endpoints: Endpoints): FeatureCollection {
  return {
    type: "FeatureCollection",
    features: [endpoints.position, endpoints.destination].map((point, index) => ({
      type: "Feature",
      properties: { kind: index === 0 ? "departure" : "destination" },
      geometry: { type: "Point", coordinates: [point.longitude, point.latitude] },
    })),
  };
}

const WIND_LAYER = "wind-field";
// Un point de vent tous les 12 px d'écran : la grille GFS (0,25°) est plus
// fine que ça seulement en zoom très large.
const WIND_CELL_PX = 12;

/** Peint le champ de vent dans une image posée sous les noms de lieux. */
function drawWindField(map: MapLibreMap, field: WindField, viewWidth: number, viewHeight: number) {
  const width = Math.max(2, Math.min(1024, Math.round(viewWidth / 2)));
  const height = Math.max(2, Math.min(1024, Math.round(viewHeight / 2)));
  const canvas = document.createElement("canvas");
  canvas.width = width;
  canvas.height = height;
  const context = canvas.getContext("2d");
  if (!context) return;
  context.putImageData(new ImageData(windFieldPixels(field, width, height) as never, width, height), 0, 0);
  const url = canvas.toDataURL();
  const coordinates: [[number, number], [number, number], [number, number], [number, number]] = [
    [field.west, field.north],
    [field.east, field.north],
    [field.east, field.south],
    [field.west, field.south],
  ];
  const source = map.getSource(WIND_LAYER) as ImageSource | undefined;
  if (source) {
    source.updateImage({ url, coordinates });
  } else {
    map.addSource(WIND_LAYER, { type: "image", url, coordinates });
    map.addLayer(
      {
        id: WIND_LAYER,
        type: "raster",
        source: WIND_LAYER,
        paint: { "raster-opacity": 0.35, "raster-fade-duration": 0, "raster-resampling": "linear" },
      },
      map.getLayer("basemap-labels") ? "basemap-labels" : undefined,
    );
  }
  map.setLayoutProperty(WIND_LAYER, "visibility", "visible");
}

function setData(map: MapLibreMap, id: string, data: FeatureCollection) {
  (map.getSource(id) as GeoJSONSource | undefined)?.setData(data as never);
}

/** « +30 min », « +2 h », « +2 h 30 ». */
function elapsedLabel(minutes: number): string {
  if (minutes < 60) return `+${minutes} min`;
  const hours = Math.floor(minutes / 60);
  const rest = minutes % 60;
  return rest ? `+${hours} h ${String(rest).padStart(2, "0")}` : `+${hours} h`;
}

/** Couleurs et contenus des épingles, repris par la légende. */
export const PINS = {
  departure: { color: "#1971c2", letter: "D", name: "Départ du routage" },
  destination: { color: "#6741d9", letter: "A", name: "Destination du routage" },
  gameArrival: { color: "#c2255c", icon: "flag", name: "Arrivée du jeu" },
  raceStart: { color: "#e8590c", icon: "sports_score", name: "Départ de la course" },
} as const;

type PinStyle = { color: string; name: string; letter?: string; icon?: string };

/**
 * Épingle avec une lettre (D, A) ou une icône au centre, à la place de
 * l'épingle anonyme de MapLibre. Elle porte aussi son nom pour l'infobulle et
 * les lecteurs d'écran.
 */
function pinElement(style: PinStyle): HTMLElement {
  const element = document.createElement("div");
  element.className = "map-pin";
  element.title = style.name;
  element.setAttribute("role", "img");
  element.setAttribute("aria-label", style.name);
  element.innerHTML = `<svg viewBox="0 0 28 40" width="28" height="40" aria-hidden="true">
    <path d="M14 0C6.3 0 0 6.2 0 13.9 0 24.3 14 40 14 40s14-15.7 14-26.1C28 6.2 21.7 0 14 0z" fill="${style.color}" stroke="#ffffff" stroke-width="1.5"/>
    <circle cx="14" cy="14" r="9" fill="#ffffff"/>
  </svg>`;
  const content = document.createElement("span");
  content.className = style.icon ? "map-pin-content material-symbols-outlined" : "map-pin-content";
  content.style.color = style.color;
  content.textContent = style.icon ?? style.letter ?? "";
  element.append(content);
  return element;
}

function elapsedMinutes(startedAt: string | undefined, instant: string | undefined): number {
  if (!startedAt || !instant) return 0;
  return Math.round((Date.parse(instant) - Date.parse(startedAt)) / 60000);
}

type Props = {
  endpoints: Endpoints;
  /** Départ officiel de la course : marqueur fixe, distinct du bateau. */
  raceStart?: Point | null;
  /** Arrivée officielle du jeu : marqueur fixe, distinct de la destination de routage. */
  gameArrival?: Point | null;
  /** Rayon d'arrivée du jeu, en milles nautiques, dessiné autour de l'arrivée. */
  arrivalRadiusNM?: number | null;
  frame: RouteFrame | null;
  vrState: VrState | undefined;
  /** Zones interdites dessinées par l'utilisateur. */
  zones: Zone[];
  /** Position choisie dans le tableau des détails, mise en évidence sur la route. */
  highlight: Point | null;
  /** Points du tracé en cours, ou null quand aucun tracé n'est actif. */
  draft: Point[] | null;
  /** Trace GPX importée, ou null quand aucune n'est chargée. */
  gpx: GpxOverlay | null;
  onDraftClick: (point: Point, closing: boolean) => void;
  /** Afficher les points d'étape de la route. */
  showRoutePoints: boolean;
  /** Instant (ISO) du vent à colorer selon l'échelle de Beaufort, ou null pour masquer. */
  windTime: string | null;
  /** Zone mise en évidence depuis la liste des zones, ou null. */
  focusedZoneId: string | null;
  /** Place prise par les panneaux ouverts, à laisser libre lors des cadrages. */
  insets: () => MapInsets;
  onError: () => void;
};

export const MapView = forwardRef<MapViewHandle, Props>(function MapView(
  { endpoints, raceStart, gameArrival, arrivalRadiusNM, frame, vrState, zones, highlight, draft, gpx, onDraftClick, showRoutePoints, windTime, focusedZoneId, insets, onError },
  handle,
) {
  const container = useRef<HTMLDivElement>(null);
  const mapRef = useRef<MapLibreMap | undefined>(undefined);
  const apiRef = useRef<MapLibreModule | undefined>(undefined);
  const departureMarker = useRef<Marker | undefined>(undefined);
  const destinationMarker = useRef<Marker | undefined>(undefined);
  const raceStartMarker = useRef<Marker | undefined>(undefined);
  const gateMarkers = useRef<Marker[]>([]);
  const gameArrivalMarker = useRef<Marker | undefined>(undefined);
  const boundsRef = useRef<LngLatBounds | undefined>(undefined);
  const checkpointKeyRef = useRef("");
  const renderedKey = useRef("");
  const renderedCount = useRef(0);
  const renderedFeatures = useRef<unknown[]>([]);
  const renderedGpxKey = useRef("");
  // Densité d'affichage des isochrones : avancée du front (milles par minute)
  // de chaque isochrone, pas de calcul et origine des temps (minutes epoch).
  const isochroneRates = useRef<number[]>([]);
  const isochroneStep = useRef(0);
  const isochroneOrigin = useRef(0);

  // N'affiche que les tracés d'un intervalle (10 min, 1 h, 3 h...) choisi pour
  // qu'au zoom courant deux tracés restent espacés d'une douzaine de pixels ;
  // un tracé sur six est renforcé. Au pas de 10 min, tout tracer donne une
  // bande pleine. Chaque intervalle a ses propres tracés (voir plus bas).
  function applyIsochroneDensity(map: MapLibreMap) {
    if (!map.getLayer("isochrones")) return;
    const step = isochroneStep.current;
    const rates = isochroneRates.current.filter((rate) => rate > 0).sort((a, b) => a - b);
    const rate = rates.length ? rates[Math.floor(rates.length / 2)] : 0;
    const latitude = map.getCenter().lat;
    const pixelsPerNM = (512 * 2 ** map.getZoom()) / (21600 * Math.max(0.05, Math.cos((latitude * Math.PI) / 180)));
    const interval = step > 0 ? isochroneDisplayInterval(step, rate, pixelsPerNM) : 1;
    const elapsed = ["-", ["get", "t"], isochroneOrigin.current];
    map.setFilter("isochrones", ["==", ["get", "interval"], interval] as never);
    const major = ["==", ["%", elapsed, interval * 6], 0];
    map.setPaintProperty("isochrones", "line-width", ["case", major, 2.2, 1.2] as never);
    map.setPaintProperty("isochrones", "line-opacity", ["case", major, 0.95, 0.7] as never);
  }
  const [ready, setReady] = useState(false);

  // Afficher / masquer les points d'étape de la route sans recalculer.
  useEffect(() => {
    const map = mapRef.current;
    if (!map || !ready) return;
    const visibility = showRoutePoints ? "visible" : "none";
    for (const id of ["route-points", "route-point-labels", "route-points-hit", "route-hover", "route-hover-label"]) {
      if (map.getLayer(id)) map.setLayoutProperty(id, "visibility", visibility);
    }
  }, [showRoutePoints, ready]);

  // Fond coloré selon la force du vent (Beaufort), recalculé pour la zone
  // affichée à chaque déplacement de la carte.
  useEffect(() => {
    const map = mapRef.current;
    if (!map || !ready) return;
    if (!windTime) {
      if (map.getLayer(WIND_LAYER)) map.setLayoutProperty(WIND_LAYER, "visibility", "none");
      return;
    }
    let controller: AbortController | undefined;
    let timer: number | undefined;
    const load = () => {
      window.clearTimeout(timer);
      timer = window.setTimeout(async () => {
        controller?.abort();
        controller = new AbortController();
        const bounds = map.getBounds();
        const width = map.getContainer().clientWidth;
        const height = map.getContainer().clientHeight;
        const cells = (pixels: number) => Math.max(2, Math.min(256, Math.ceil(pixels / WIND_CELL_PX) + 1));
        const params = new URLSearchParams({
          time: windTime,
          south: String(Math.max(-85, bounds.getSouth())),
          north: String(Math.min(85, bounds.getNorth())),
          west: String(Math.max(-180, bounds.getWest())),
          east: String(Math.min(180, bounds.getEast())),
          cols: String(cells(width)),
          rows: String(cells(height)),
        });
        try {
          const response = await fetch(`/api/weather/wind?${params}`, { signal: controller.signal });
          if (!response.ok) return;
          drawWindField(map, (await response.json()) as WindField, width, height);
        } catch {
          // Demande remplacée par une plus récente, ou backend injoignable : on garde l'image actuelle.
        }
      }, 250);
    };
    load();
    map.on("moveend", load);
    return () => {
      map.off("moveend", load);
      window.clearTimeout(timer);
      controller?.abort();
    };
  }, [windTime, ready]);

  const insetsRef = useRef(insets);
  insetsRef.current = insets;

  // Place à laisser libre : les panneaux à droite, et la colonne du coin bas
  // gauche réservée soit en bas soit à gauche, du côté qui coûte le moins de
  // carte (à gauche quand la légende la rend haute sur un écran large).
  const reservedSpace = () => {
    const map = mapRef.current;
    const { right, bottom, left } = insetsRef.current();
    const width = map?.getContainer().clientWidth ?? window.innerWidth;
    const height = map?.getContainer().clientHeight ?? window.innerHeight;
    const usableWidth = Math.max(1, width - right);
    const onLeft = left > 0 && left / usableWidth < bottom / height;
    return { width, height, right, bottom: onLeft ? 0 : bottom, left: onLeft ? left : 0 };
  };

  // Marges de cadrage. Sur un écran trop petit pour réserver toute cette place,
  // on garde une marge fixe plutôt qu'un cadre minuscule.
  const padding = () => {
    const { width, height, right, bottom, left } = reservedSpace();
    const padRight = right > 0 && width - right - left > 240 ? right + 20 : MAP_MARGIN;
    const padLeft = left > 0 && width - right - left > 240 ? left + 20 : MAP_MARGIN;
    const padBottom = bottom > 0 && height - bottom > 240 ? bottom + 20 : MAP_MARGIN;
    return { top: MAP_MARGIN, left: padLeft, right: padRight, bottom: padBottom };
  };

  useImperativeHandle(handle, () => ({
    recenter() {
      const map = mapRef.current;
      if (map && boundsRef.current) {
        map.fitBounds(boundsRef.current, { padding: padding(), maxZoom: 12, duration: 500 });
      }
    },
    focusPoints(points: Point[]) {
      const map = mapRef.current;
      const api = apiRef.current;
      if (!map || !api || points.length === 0) return;
      const bounds = new api.LngLatBounds();
      points.forEach((point) => bounds.extend([point.longitude, point.latitude]));
      map.fitBounds(bounds, { padding: padding(), maxZoom: 11, duration: 600 });
    },
  }));

  // Création de la carte, une seule fois.
  useEffect(() => {
    let disposed = false;
    let map: MapLibreMap | undefined;

    async function initialize() {
      let maplibregl: MapLibreModule;
      try {
        maplibregl = await import("maplibre-gl");
      } catch {
        if (!disposed) onError();
        return;
      }
      if (disposed || !container.current) return;
      apiRef.current = maplibregl;
      map = new maplibregl.Map({
        container: container.current,
        style: {
          version: 8,
          sources: {
            // Carte marine de nuit sans clé d'API : relief sous-marin Esri Ocean
            // assombri (publié jusqu'au zoom 10, agrandi au-delà), noms de lieux
            // clairs, balisage OpenSeaMap (phares, bouées) et grille des
            // latitudes et longitudes.
            basemap: {
              type: "raster",
              tiles: [
                "https://services.arcgisonline.com/ArcGIS/rest/services/Ocean/World_Ocean_Base/MapServer/tile/{z}/{y}/{x}",
              ],
              tileSize: 256,
              maxzoom: 10,
              attribution: "Tuiles © Esri — GEBCO, NOAA, Garmin, HERE, © OpenStreetMap contributors",
            },
            // Terre et mer d'OpenStreetMap en vectoriel (OpenFreeMap, sans clé) :
            // nettes à tous les zooms et partout dans le monde, elles prennent le
            // relais du relief Esri, qui n'est publié en entier que jusqu'au zoom 10.
            [COASTLINE_SOURCE]: {
              type: "vector",
              url: "https://tiles.openfreemap.org/planet",
              attribution: "Côtes © OpenFreeMap, © OpenStreetMap contributors",
            },
            "basemap-labels": {
              type: "raster",
              tiles: [
                "https://services.arcgisonline.com/ArcGIS/rest/services/Canvas/World_Dark_Gray_Reference/MapServer/tile/{z}/{y}/{x}",
              ],
              tileSize: 256,
              maxzoom: 16,
            },
            seamarks: {
              type: "raster",
              tiles: ["https://tiles.openseamap.org/seamark/{z}/{x}/{y}.png"],
              tileSize: 256,
              minzoom: 9,
              maxzoom: 18,
              attribution: "Balisage © OpenSeaMap contributors",
            },
            graticule: { type: "geojson", data: graticule() as never },
          },
          layers: [
            // Le relief marin est assombri : carte marine de nuit.
            {
              id: "basemap",
              type: "raster",
              source: "basemap",
              paint: BASEMAP_PAINT,
            },
            // Au-delà du zoom 10, la terre puis l'eau sont peintes en aplats
            // par-dessus le relief agrandi, avec un fondu entre 9,5 et 10,5.
            {
              id: "coast-land",
              type: "background",
              minzoom: 9.5,
              paint: { "background-color": LAND_COLOR, "background-opacity": COASTLINE_FADE },
            },
            {
              id: "coast-water",
              type: "fill",
              source: COASTLINE_SOURCE,
              "source-layer": "water",
              minzoom: 9.5,
              paint: { "fill-color": WATER_COLOR, "fill-opacity": COASTLINE_FADE, "fill-antialias": true },
            },
            { id: "basemap-labels", type: "raster", source: "basemap-labels", paint: { "raster-opacity": 0.85 } },
            {
              id: "graticule",
              type: "line",
              source: "graticule",
              paint: {
                "line-color": "#a5d8ff",
                "line-opacity": ["case", ["boolean", ["get", "major"], false], 0.35, 0.18],
                "line-width": ["case", ["boolean", ["get", "major"], false], 1.2, 0.8],
                "line-dasharray": [4, 3],
              },
            },
            // Atténué pour ne pas rivaliser avec la route.
            { id: "seamarks", type: "raster", source: "seamarks", minzoom: 9, paint: { "raster-opacity": 0.6 } },
          ],
        },
        center: [DEFAULT_DEPARTURE.longitude, DEFAULT_DEPARTURE.latitude],
        zoom: 10,
        minZoom: 5,
        attributionControl: {},
        locale: MAP_LOCALE,
      });
      mapRef.current = map;
      // À gauche : les panneaux et le bouton des détails occupent la droite.
      map.addControl(new maplibregl.NavigationControl(), "top-left");

      map.on("load", () => {
        if (!map) return;
        departureMarker.current = new maplibregl.Marker({ element: pinElement(PINS.departure), anchor: "bottom" })
          .setLngLat([DEFAULT_DEPARTURE.longitude, DEFAULT_DEPARTURE.latitude])
          .addTo(map);
        destinationMarker.current = new maplibregl.Marker({ element: pinElement(PINS.destination), anchor: "bottom" })
          .setLngLat([DEFAULT_DESTINATION.longitude, DEFAULT_DESTINATION.latitude])
          .addTo(map);

        // Rayon d'arrivée du jeu : la course est finie dès qu'on entre dans ce cercle.
        map.addSource("arrival-radius", { type: "geojson", data: EMPTY as never });
        map.addLayer({
          id: "arrival-radius-fill",
          type: "fill",
          source: "arrival-radius",
          paint: { "fill-color": "#c2255c", "fill-opacity": 0.1 },
        });
        map.addLayer({
          id: "arrival-radius-line",
          type: "line",
          source: "arrival-radius",
          paint: { "line-color": "#f06595", "line-width": 2, "line-dasharray": [3, 2] },
        });

        map.addSource("points", {
          type: "geojson",
          data: endpointCollection({ position: DEFAULT_DEPARTURE, destination: DEFAULT_DESTINATION }) as never,
        });
        map.addLayer({
          id: "points",
          type: "circle",
          source: "points",
          paint: {
            "circle-radius": 9,
            "circle-color": ["match", ["get", "kind"], "departure", "#1971c2", "#6741d9"],
            "circle-stroke-color": "#ffffff",
            "circle-stroke-width": 3,
          },
        });

        // Portes : un repère HTML animé sur la marque de départ (voir plus bas) ;
        // la carte n'affiche que leur nom, à côté du repère.
        map.addSource("vr-checkpoints", { type: "geojson", data: EMPTY as never });
        map.addLayer({
          id: "vr-checkpoint-labels",
          type: "symbol",
          source: "vr-checkpoints",
          filter: ["all", ["==", ["geometry-type"], "Point"], ["==", ["get", "kind"], "label"]],
          // Les noms se placent à côté du repère de la porte, et MapLibre masque
          // ceux qui se chevaucheraient.
          layout: {
            "text-field": ["get", "label"],
            "text-size": 12,
            "text-variable-anchor": ["left", "right", "bottom", "top"],
            "text-radial-offset": 2,
            "text-justify": "auto",
            "text-allow-overlap": false,
            "text-padding": 4,
          },
          paint: { "text-color": "#d0bfff", "text-halo-color": "#101418", "text-halo-width": 2 },
        });

        // Zones interdites et tracé en cours : sous la route.
        map.addSource("zones", { type: "geojson", data: EMPTY as never });
        map.addLayer({
          id: "zones-fill",
          type: "fill",
          source: "zones",
          paint: {
            "fill-color": ["case", ["boolean", ["get", "enabled"], true], "#ff6b6b", "#868e96"],
            "fill-opacity": ["case", ["boolean", ["get", "enabled"], true], 0.28, 0.12],
          },
        });
        map.addLayer({
          id: "zones-line",
          type: "line",
          source: "zones",
          paint: {
            "line-color": [
              "case",
              ["boolean", ["get", "focused"], false],
              "#ffd43b",
              ["boolean", ["get", "enabled"], true],
              "#ff6b6b",
              "#868e96",
            ],
            "line-width": ["case", ["boolean", ["get", "focused"], false], 4, 2],
          },
        });
        map.addSource("zone-draft", { type: "geojson", data: EMPTY as never });
        map.addLayer({
          id: "zone-draft-line",
          type: "line",
          source: "zone-draft",
          filter: ["==", ["geometry-type"], "LineString"],
          paint: { "line-color": "#ff6b6b", "line-width": 2, "line-dasharray": [2, 2] },
        });
        map.addLayer({
          id: "zone-draft-points",
          type: "circle",
          source: "zone-draft",
          filter: ["==", ["geometry-type"], "Point"],
          paint: {
            "circle-radius": ["case", ["boolean", ["get", "first"], false], 8, 4],
            "circle-color": ["case", ["boolean", ["get", "first"], false], "#ffffff", "#ff6b6b"],
            "circle-stroke-color": "#ff6b6b",
            "circle-stroke-width": 2,
          },
        });

        // Trace GPX importée (référence) : sous la route calculée, au-dessus des zones.
        map.addSource("gpx", { type: "geojson", data: EMPTY as never });
        map.addLayer({
          id: "gpx-line-casing",
          type: "line",
          source: "gpx",
          filter: ["==", ["geometry-type"], "LineString"],
          layout: { "line-cap": "round", "line-join": "round" },
          paint: { "line-color": "#101418", "line-width": 6, "line-opacity": 0.85 },
        });
        map.addLayer({
          id: "gpx-line",
          type: "line",
          source: "gpx",
          filter: ["==", ["geometry-type"], "LineString"],
          layout: { "line-cap": "round", "line-join": "round" },
          paint: { "line-color": "#f783ac", "line-width": 3, "line-opacity": 0.95 },
        });
        map.addLayer({
          id: "gpx-waypoints",
          type: "circle",
          source: "gpx",
          filter: ["==", ["geometry-type"], "Point"],
          paint: {
            "circle-radius": 5,
            "circle-color": "#f783ac",
            "circle-stroke-color": "#ffffff",
            "circle-stroke-width": 2,
          },
        });
        map.addLayer({
          id: "gpx-waypoint-labels",
          type: "symbol",
          source: "gpx",
          filter: ["==", ["geometry-type"], "Point"],
          layout: {
            "text-field": ["get", "name"],
            "text-size": 11,
            "text-offset": [0, -1.4],
            "text-anchor": "bottom",
          },
          paint: { "text-color": "#f783ac", "text-halo-color": "#101418", "text-halo-width": 2 },
        });

        // Route, isochrones et points de route : au-dessus des portes.
        map.addSource("route", { type: "geojson", data: EMPTY as never });
        map.addLayer({
          id: "route",
          type: "line",
          source: "route",
          paint: { "line-color": "#f59f00", "line-width": 5, "line-opacity": 0.95 },
        });
        map.addSource("isochrones", { type: "geojson", data: EMPTY as never });
        map.addLayer({
          id: "isochrones",
          type: "line",
          source: "isochrones",
          paint: {
            "line-color": ["get", "color"],
            "line-width": ["case", ["boolean", ["get", "major"], false], 2.2, 1.2],
            "line-opacity": ["case", ["boolean", ["get", "major"], false], 0.95, 0.6],
          },
        });
        map.addSource("route-points", { type: "geojson", data: EMPTY as never });
        map.addLayer({
          id: "route-points",
          type: "circle",
          source: "route-points",
          // Seuls les points marquants (manœuvres, heures rondes) sont dessinés ;
          // les autres apparaissent au survol.
          filter: ["boolean", ["get", "key"], false],
          paint: {
            "circle-radius": 6,
            "circle-color": ["case", ["boolean", ["get", "maneuver"], false], "#ffd43b", "#f59f00"],
            "circle-stroke-color": "#ffffff",
            "circle-stroke-width": 2,
          },
        });
        // Zone de survol invisible autour de chaque point de la route.
        map.addLayer({
          id: "route-points-hit",
          type: "circle",
          source: "route-points",
          paint: { "circle-radius": 9, "circle-opacity": 0, "circle-stroke-opacity": 0 },
        });
        map.addLayer({
          id: "route-point-labels",
          type: "symbol",
          source: "route-points",
          layout: {
            "text-field": ["get", "label"],
            "text-size": 12,
            "text-offset": [0, -1.5],
            "text-anchor": "bottom",
            "text-allow-overlap": false,
          },
          paint: { "text-color": "#a5d8ff", "text-halo-color": "#101418", "text-halo-width": 2 },
        });

        // Point survolé : repère et résumé du pas (temps, cap, voile).
        map.addSource("route-hover", { type: "geojson", data: EMPTY as never });
        map.addLayer({
          id: "route-hover",
          type: "circle",
          source: "route-hover",
          paint: {
            "circle-radius": 5,
            "circle-color": ["case", ["boolean", ["get", "maneuver"], false], "#ffd43b", "#f59f00"],
            "circle-stroke-color": "#ffffff",
            "circle-stroke-width": 2,
          },
        });
        map.addLayer({
          id: "route-hover-label",
          type: "symbol",
          source: "route-hover",
          layout: {
            "text-field": ["get", "tip"],
            "text-size": 12,
            "text-offset": [0, -1.4],
            "text-anchor": "bottom",
            "text-allow-overlap": true,
            "text-ignore-placement": true,
          },
          paint: { "text-color": "#ffffff", "text-halo-color": "#101418", "text-halo-width": 2.5 },
        });
        const clearHover = () => {
          setData(map!, "route-hover", EMPTY);
          map!.getCanvas().style.cursor = "";
        };
        map.on("mousemove", "route-points-hit", (event) => {
          const feature = event.features?.[0];
          if (!feature || draftRef.current !== null || feature.geometry.type !== "Point") return;
          // L'objet renvoyé par la carte n'est pas un GeoJSON simple : on en
          // recopie les valeurs, sinon MapLibre ne peut pas le transmettre.
          const [longitude, latitude] = feature.geometry.coordinates;
          setData(map!, "route-hover", {
            type: "FeatureCollection",
            features: [
              {
                type: "Feature",
                properties: { tip: String(feature.properties?.tip ?? ""), maneuver: feature.properties?.maneuver === true },
                geometry: { type: "Point", coordinates: [longitude, latitude] },
              },
            ],
          });
          map!.getCanvas().style.cursor = "pointer";
        });
        map.on("mouseleave", "route-points-hit", clearHover);
        map.on("zoomend", () => applyIsochroneDensity(map!));

        // Position choisie dans le tableau des détails : au-dessus de tout le reste.
        map.addSource("route-highlight", { type: "geojson", data: EMPTY as never });
        map.addLayer({
          id: "route-highlight-halo",
          type: "circle",
          source: "route-highlight",
          paint: { "circle-radius": 16, "circle-color": "#4dabf7", "circle-opacity": 0.3 },
        });
        map.addLayer({
          id: "route-highlight",
          type: "circle",
          source: "route-highlight",
          paint: {
            "circle-radius": 8,
            "circle-color": "#4dabf7",
            "circle-stroke-color": "#ffffff",
            "circle-stroke-width": 3,
          },
        });

        // Sur petit écran, MapLibre ouvre l'attribution compacte au chargement :
        // elle passerait sous l'encart météo. Elle reste accessible par son bouton « i ».
        container.current?.querySelector(".maplibregl-compact-show")?.classList.remove("maplibregl-compact-show");

        const bounds = new maplibregl.LngLatBounds();
        bounds.extend([DEFAULT_DEPARTURE.longitude, DEFAULT_DEPARTURE.latitude]);
        bounds.extend([DEFAULT_DESTINATION.longitude, DEFAULT_DESTINATION.latitude]);
        boundsRef.current = bounds;
        map.fitBounds(bounds, { padding: 80, maxZoom: 12 });
        setReady(true);
      });
    }

    void initialize();
    return () => {
      disposed = true;
      setReady(false);
      map?.remove();
      mapRef.current = undefined;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Marqueurs de départ et de destination.
  useEffect(() => {
    const map = mapRef.current;
    const api = apiRef.current;
    if (!ready || !map || !api) return;
    departureMarker.current?.setLngLat([endpoints.position.longitude, endpoints.position.latitude]);
    destinationMarker.current?.setLngLat([endpoints.destination.longitude, endpoints.destination.latitude]);
    setData(map, "points", endpointCollection(endpoints));
    const bounds = new api.LngLatBounds();
    bounds.extend([endpoints.position.longitude, endpoints.position.latitude]);
    bounds.extend([endpoints.destination.longitude, endpoints.destination.latitude]);
    boundsRef.current = bounds;
    map.triggerRepaint();
  }, [ready, endpoints]);

  // Départ officiel de la course (fixe), affiché seulement s'il est connu.
  const raceStartLat = raceStart?.latitude;
  const raceStartLon = raceStart?.longitude;
  useEffect(() => {
    const map = mapRef.current;
    const api = apiRef.current;
    if (!ready || !map || !api) return;
    if (raceStartLat === undefined || raceStartLon === undefined) {
      raceStartMarker.current?.remove();
      raceStartMarker.current = undefined;
      return;
    }
    if (!raceStartMarker.current) {
      raceStartMarker.current = new api.Marker({ element: pinElement(PINS.raceStart), anchor: "bottom" })
        .setLngLat([raceStartLon, raceStartLat])
        .addTo(map);
    }
    raceStartMarker.current.setLngLat([raceStartLon, raceStartLat]);
    map.triggerRepaint();
  }, [ready, raceStartLat, raceStartLon]);

  // Arrivée officielle du jeu (fixe), indépendante de la destination de routage.
  const gameArrivalLat = gameArrival?.latitude;
  const gameArrivalLon = gameArrival?.longitude;
  useEffect(() => {
    const map = mapRef.current;
    const api = apiRef.current;
    if (!ready || !map || !api) return;
    if (gameArrivalLat === undefined || gameArrivalLon === undefined) {
      gameArrivalMarker.current?.remove();
      gameArrivalMarker.current = undefined;
      return;
    }
    if (!gameArrivalMarker.current) {
      gameArrivalMarker.current = new api.Marker({ element: pinElement(PINS.gameArrival), anchor: "bottom" })
        .setLngLat([gameArrivalLon, gameArrivalLat])
        .addTo(map);
    }
    gameArrivalMarker.current.setLngLat([gameArrivalLon, gameArrivalLat]);
    map.triggerRepaint();
  }, [ready, gameArrivalLat, gameArrivalLon]);

  // Quand la destination du routage est l'arrivée du jeu ou la marque d'une
  // porte, une seule épingle suffit : le drapeau ou l'anneau de la porte
  // masque le « A ».
  const destinationLat = endpoints.destination.latitude;
  const destinationLon = endpoints.destination.longitude;
  useEffect(() => {
    if (!ready) return;
    const element = destinationMarker.current?.getElement();
    if (!element) return;
    const destination = { latitude: destinationLat, longitude: destinationLon };
    const sameAsArrival =
      gameArrivalLat !== undefined &&
      gameArrivalLon !== undefined &&
      approximateDistanceNM(destination, { latitude: gameArrivalLat, longitude: gameArrivalLon }) < 0.1;
    const onGateMark = usableCheckpoints(vrState).some(
      (checkpoint) =>
        checkpoint.start &&
        Number.isFinite(Number(checkpoint.start.latitude)) &&
        Number.isFinite(Number(checkpoint.start.longitude)) &&
        approximateDistanceNM(destination, {
          latitude: Number(checkpoint.start.latitude),
          longitude: Number(checkpoint.start.longitude),
        }) < 0.1,
    );
    element.style.visibility = sameAsArrival || onGateMark ? "hidden" : "";
  }, [ready, destinationLat, destinationLon, gameArrivalLat, gameArrivalLon, vrState]);

  // Cercle du rayon d'arrivée, quand le jeu le fournit.
  useEffect(() => {
    const map = mapRef.current;
    if (!ready || !map) return;
    if (gameArrivalLat === undefined || gameArrivalLon === undefined || !arrivalRadiusNM || arrivalRadiusNM <= 0) {
      setData(map, "arrival-radius", EMPTY);
      return;
    }
    const ring = circlePolygon({ latitude: gameArrivalLat, longitude: gameArrivalLon }, arrivalRadiusNM).map((point) => [
      point.longitude,
      point.latitude,
    ]);
    setData(map, "arrival-radius", {
      type: "FeatureCollection",
      features: [{ type: "Feature", properties: {}, geometry: { type: "Polygon", coordinates: [ring] } }],
    });
  }, [ready, gameArrivalLat, gameArrivalLon, arrivalRadiusNM]);

  // Portes Virtual Regatta.
  useEffect(() => {
    const map = mapRef.current;
    const api = apiRef.current;
    if (!ready || !map || !api) return;
    // Noms des portes (couche de texte) et repères animés sur leur marque de départ.
    const features = checkpointFeatures(vrState).filter((feature) => feature.properties.kind === "label");
    setData(map, "vr-checkpoints", { type: "FeatureCollection", features });
    gateMarkers.current.forEach((marker) => marker.remove());
    gateMarkers.current = usableCheckpoints(vrState).flatMap((checkpoint, index) => {
      const start = checkpoint.start;
      if (!start || !Number.isFinite(Number(start.latitude)) || !Number.isFinite(Number(start.longitude))) return [];
      const rounding = gateRounding(checkpoint.side, checkpoint.rhumb);
      const element = document.createElement("div");
      element.className = `gate-marker is-${rounding}`;
      const name = `Porte ${index + 1} (P${checkpointKey(checkpoint, index)} — ${checkpointName(checkpoint)}) : ${gateRoundingLabel(rounding)}`;
      element.title = name;
      element.setAttribute("role", "img");
      element.setAttribute("aria-label", name);
      element.innerHTML = `<span class="gate-ring">${gateRingSvg(rounding)}</span><span class="gate-order">${index + 1}</span>`;
      return [new api.Marker({ element }).setLngLat([Number(start.longitude), Number(start.latitude)]).addTo(map)];
    });
    // Cadrage sur les marques de départ : les secondes extrémités, souvent
    // lointaines, ne sont plus dessinées.
    const coordinates = features.map((feature) => feature.geometry.coordinates as number[]);
    const nextKey = JSON.stringify(coordinates);
    if (nextKey !== checkpointKeyRef.current && coordinates.length > 0) {
      const bounds = new api.LngLatBounds();
      coordinates.forEach((coordinate) => bounds.extend(coordinate as [number, number]));
      if (vrState?.position) bounds.extend([vrState.position.longitude, vrState.position.latitude]);
      boundsRef.current = bounds;
      map.fitBounds(bounds, { padding: padding(), maxZoom: 10, duration: 500 });
    }
    checkpointKeyRef.current = nextKey;
  }, [ready, vrState]);

  // Zones interdites enregistrées et tracé en cours.
  useEffect(() => {
    const map = mapRef.current;
    if (!ready || !map) return;
    const collection = zonesFeatureCollection(zones);
    collection.features.forEach((feature) => {
      feature.properties.focused = feature.properties.id === focusedZoneId;
    });
    setData(map, "zones", collection);
  }, [ready, zones, focusedZoneId]);

  useEffect(() => {
    const map = mapRef.current;
    if (!ready || !map) return;
    setData(map, "zone-draft", draftFeatureCollection(draft || []));
  }, [ready, draft]);

  // Trace GPX importée : affichage et recadrage à chaque nouvel import.
  useEffect(() => {
    const map = mapRef.current;
    const api = apiRef.current;
    if (!ready || !map || !api) return;
    if (!gpx) {
      renderedGpxKey.current = "";
      setData(map, "gpx", EMPTY);
      return;
    }
    setData(map, "gpx", gpx.collection);
    if (gpx.key !== renderedGpxKey.current) {
      renderedGpxKey.current = gpx.key;
      const bounds = new api.LngLatBounds();
      let extended = false;
      for (const feature of gpx.collection.features as Array<{
        geometry: { type: string; coordinates: number[] | number[][] };
      }>) {
        const geometry = feature.geometry;
        if (geometry.type === "LineString") {
          for (const coordinate of geometry.coordinates as number[][]) {
            bounds.extend(coordinate as [number, number]);
            extended = true;
          }
        } else if (geometry.type === "Point") {
          bounds.extend(geometry.coordinates as [number, number]);
          extended = true;
        }
      }
      if (extended) {
        boundsRef.current = bounds;
        map.fitBounds(bounds, { padding: padding(), maxZoom: 12, duration: 500 });
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ready, gpx]);

  // Position choisie dans le tableau : repère sur la route, et la carte la
  // recentre dans la partie non couverte par les panneaux.
  useEffect(() => {
    const map = mapRef.current;
    if (!ready || !map) return;
    if (!highlight) {
      setData(map, "route-highlight", EMPTY);
      return;
    }
    const center: [number, number] = [highlight.longitude, highlight.latitude];
    setData(map, "route-highlight", {
      type: "FeatureCollection",
      features: [{ type: "Feature", properties: {}, geometry: { type: "Point", coordinates: center } }],
    });
    const { width, height, right, bottom, left } = reservedSpace();
    const visible = map.getBounds();
    const point = map.project(center);
    const hidden =
      point.x > width - right - 40 || point.x < left + 40 || point.y > height - bottom - 40 || !visible.contains(center);
    if (hidden) {
      map.easeTo({ center, padding: { top: 0, left, right, bottom }, duration: 400 });
    }
  }, [ready, highlight]);

  // Clic sur la carte pendant le tracé : ajoute un point, ou ferme la zone en
  // cliquant près du premier point.
  const draftRef = useRef(draft);
  draftRef.current = draft;
  const onDraftClickRef = useRef(onDraftClick);
  onDraftClickRef.current = onDraftClick;
  const drawing = draft !== null;
  useEffect(() => {
    const map = mapRef.current;
    if (!ready || !map || !drawing) return;
    const canvas = map.getCanvas();
    const previousCursor = canvas.style.cursor;
    canvas.style.cursor = "crosshair";
    map.doubleClickZoom.disable();
    const onClick = (event: MapMouseEvent) => {
      const current = draftRef.current || [];
      let closing = false;
      if (current.length >= 3) {
        const first = map.project([current[0].longitude, current[0].latitude]);
        closing = Math.hypot(first.x - event.point.x, first.y - event.point.y) <= 12;
      }
      const longitude = ((((event.lngLat.lng + 180) % 360) + 360) % 360) - 180;
      onDraftClickRef.current({ latitude: event.lngLat.lat, longitude }, closing);
    };
    map.on("click", onClick);
    return () => {
      map.off("click", onClick);
      canvas.style.cursor = previousCursor;
      map.doubleClickZoom.enable();
    };
  }, [ready, drawing]);

  // Route et isochrones.
  useEffect(() => {
    const map = mapRef.current;
    const api = apiRef.current;
    if (!ready || !map || !api) return;
    if (!frame) {
      renderedKey.current = "";
      renderedCount.current = 0;
      renderedFeatures.current = [];
      setData(map, "route", EMPTY);
      setData(map, "isochrones", EMPTY);
      setData(map, "route-points", EMPTY);
      setData(map, "route-hover", EMPTY);
      return;
    }

    const { result } = frame;
    const route = frame.showRoute ? result.route || [] : [];
    const position = result.position || DEFAULT_DEPARTURE;
    const destination = result.destination || DEFAULT_DESTINATION;

    if (frame.key !== renderedKey.current || (result.isochrones?.length || 0) < renderedCount.current) {
      renderedKey.current = frame.key;
      renderedCount.current = 0;
      renderedFeatures.current = [];
      isochroneRates.current = [];
    }
    const direction = {
      x: destination.longitude - position.longitude,
      y: destination.latitude - position.latitude,
    };
    const isochrones: Isochrone[] = result.isochrones || [];
    const instantMinutes = (isochrone: Isochrone) => Math.round(Date.parse(isochrone.instant) / 60000);
    // Pas de calcul déduit des deux premiers fronts ; l'origine des temps est
    // le départ, un pas avant le premier front.
    const step = isochrones.length > 1 ? instantMinutes(isochrones[1]) - instantMinutes(isochrones[0]) : 0;
    isochroneStep.current = step > 0 ? step : 0;
    isochroneOrigin.current = isochrones.length ? instantMinutes(isochrones[0]) - isochroneStep.current : 0;
    const intervals = isochroneIntervals(isochroneStep.current);
    // Le pas n'est connu qu'au deuxième front : on attend pour tracer.
    const available = isochroneStep.current > 0 ? isochrones.length : 0;
    for (let index = renderedCount.current; index < available; index += 1) {
      const isochrone = isochrones[index];
      // 110° plutôt que 90° : les flancs de l'enveloppe restent tracés, sinon
      // le front paraît troué entre l'arc de face et ses côtés.
      const contour = forwardContour(isochrone.contour || [], direction, ISOCHRONE_FACING_ANGLE);
      const visibleContour = contour.length > 1 ? contour : isochrone.contour || [];
      // Le contour est coupé là où il traverserait une terre ou une zone
      // interdite. Le backend signale ces côtés ; un backend plus ancien ne
      // le fait pas, et on retombe alors sur la coupe des sauts anormaux.
      const cutParts = isochrone.contourBreaks
        ? cutContourAtBreaks(visibleContour, isochrone.contour || [], isochrone.contourBreaks)
        : splitContour(visibleContour);
      const previousRing = index > 0 ? isochrones[index - 1].contour || [] : [position];
      isochroneRates.current[index] = medianAdvanceNM(visibleContour, previousRing) / isochroneStep.current;
      const total = isochrone.totalLayers || isochrones.length || 1;
      const color = isochroneTimeColor((index + 1) / total);
      const t = instantMinutes(isochrone);
      // Un tracé par intervalle d'affichage dont ce front fait partie. Les
      // flancs avancent bien moins vite que la face du front : comparés au
      // tracé affiché précédent, ceux qui n'ont presque pas bougé sont retirés,
      // sinon ils s'empilent en bande pleine sur les bords.
      for (const interval of intervals) {
        if ((t - isochroneOrigin.current) % interval !== 0) continue;
        const previousIndex = index - interval / isochroneStep.current;
        const previousShown = previousIndex >= 0 ? isochrones[previousIndex].contour || [] : [position];
        const parts = trimStalledContour(cutParts, previousShown, ISOCHRONE_MIN_ADVANCE_SHARE);
        const coordinates = parts.map((part) => part.map((point) => [point.longitude, point.latitude]));
        if (coordinates.length === 0) continue;
        renderedFeatures.current.push({
          type: "Feature",
          properties: { t, color, interval },
          geometry: { type: "MultiLineString", coordinates },
        });
      }
    }
    renderedCount.current = available;

    const routeCoordinates = route.map((point) => [point.position.longitude, point.position.latitude]);
    setData(map, "points", endpointCollection({ position, destination }));
    setData(map, "route", {
      type: "FeatureCollection",
      features:
        routeCoordinates.length > 1
          ? [{ type: "Feature", properties: {}, geometry: { type: "LineString", coordinates: routeCoordinates } }]
          : [],
    });
    setData(map, "isochrones", { type: "FeatureCollection", features: renderedFeatures.current });
    applyIsochroneDensity(map);
    setData(map, "route-points", {
      type: "FeatureCollection",
      features: route
        .filter((point, index) => index > 0 && !point.terminal)
        .map((point) => {
          const minutes = elapsedMinutes(route[0]?.instant, point.instant);
          const maneuver = Boolean(point.maneuver);
          const tip = [
            elapsedLabel(minutes),
            Number.isFinite(point.heading) ? `${Math.round(Number(point.heading))}°` : "",
            point.sail || "",
          ]
            .filter(Boolean)
            .join(" · ");
          return {
            type: "Feature",
            properties: {
              label: minutes > 0 && minutes % 30 === 0 ? elapsedLabel(minutes) : "",
              key: maneuver || (minutes > 0 && minutes % 60 === 0),
              maneuver,
              tip,
            },
            geometry: { type: "Point", coordinates: [point.position.longitude, point.position.latitude] },
          };
        }),
    });
    departureMarker.current?.setLngLat([position.longitude, position.latitude]);
    destinationMarker.current?.setLngLat([destination.longitude, destination.latitude]);

    if (frame.fit) {
      const bounds = new api.LngLatBounds();
      [position, destination, ...route.map((point) => point.position)].forEach((point) =>
        bounds.extend([point.longitude, point.latitude]),
      );
      boundsRef.current = bounds;
      map.fitBounds(bounds, { padding: padding(), maxZoom: 12, duration: 500 });
    }
  }, [ready, frame]);

  return <div id="map" ref={container} aria-label="Carte de navigation Virtual Regatta" />;
});

import { forwardRef, useEffect, useImperativeHandle, useRef, useState } from "react";
import type { GeoJSONSource, LngLatBounds, Map as MapLibreMap, MapMouseEvent, Marker } from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import { forwardContour } from "./lib/geo.ts";
import type { Isochrone, Point, RouteResult } from "./lib/types.ts";
import { checkpointFeatures } from "./lib/vr.ts";
import { draftFeatureCollection, zonesFeatureCollection, type Zone } from "./lib/zones.ts";
import type { VrState } from "./lib/types.ts";

export const DEFAULT_DEPARTURE: Point = { latitude: 13.121111, longitude: 121.586667 };
export const DEFAULT_DESTINATION: Point = { latitude: 13.171111, longitude: 121.336667 };

// Couleur d'une isochrone selon l'avancement temporel : bleu (tôt) -> vert -> ambre (tard).
function lerpHex(a: string, b: string, t: number): string {
  const pa = [1, 3, 5].map((i) => parseInt(a.slice(i, i + 2), 16));
  const pb = [1, 3, 5].map((i) => parseInt(b.slice(i, i + 2), 16));
  const p = pa.map((v, i) => Math.round(v + (pb[i] - v) * t));
  return `rgb(${p[0]}, ${p[1]}, ${p[2]})`;
}
function isochroneTimeColor(fraction: number): string {
  const f = Math.max(0, Math.min(1, fraction));
  return f < 0.5 ? lerpHex("#1f6fb2", "#1aa589", f * 2) : lerpHex("#1aa589", "#e0a419", (f - 0.5) * 2);
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

export type MapViewHandle = { recenter: () => void };

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

function setData(map: MapLibreMap, id: string, data: FeatureCollection) {
  (map.getSource(id) as GeoJSONSource | undefined)?.setData(data as never);
}

function elapsedMinutes(startedAt: string | undefined, instant: string | undefined): number {
  if (!startedAt || !instant) return 0;
  return Math.round((Date.parse(instant) - Date.parse(startedAt)) / 60000);
}

type Props = {
  endpoints: Endpoints;
  /** Départ officiel de la course : marqueur fixe, distinct du bateau. */
  raceStart?: Point | null;
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
  /** Largeur à réserver à droite pour le formulaire ouvert, 0 sinon. */
  rightPadding: () => number;
  onError: () => void;
};

export const MapView = forwardRef<MapViewHandle, Props>(function MapView(
  { endpoints, raceStart, frame, vrState, zones, highlight, draft, gpx, onDraftClick, showRoutePoints, rightPadding, onError },
  handle,
) {
  const container = useRef<HTMLDivElement>(null);
  const mapRef = useRef<MapLibreMap | undefined>(undefined);
  const apiRef = useRef<MapLibreModule | undefined>(undefined);
  const departureMarker = useRef<Marker | undefined>(undefined);
  const destinationMarker = useRef<Marker | undefined>(undefined);
  const raceStartMarker = useRef<Marker | undefined>(undefined);
  const boundsRef = useRef<LngLatBounds | undefined>(undefined);
  const checkpointKeyRef = useRef("");
  const renderedKey = useRef("");
  const renderedCount = useRef(0);
  const renderedFeatures = useRef<unknown[]>([]);
  const renderedGpxKey = useRef("");
  const [ready, setReady] = useState(false);

  // Afficher / masquer les points d'étape de la route sans recalculer.
  useEffect(() => {
    const map = mapRef.current;
    if (!map || !ready) return;
    const visibility = showRoutePoints ? "visible" : "none";
    for (const id of ["route-points", "route-point-labels"]) {
      if (map.getLayer(id)) map.setLayoutProperty(id, "visibility", visibility);
    }
  }, [showRoutePoints, ready]);
  const rightPaddingRef = useRef(rightPadding);
  rightPaddingRef.current = rightPadding;

  const padding = () => {
    const right = rightPaddingRef.current();
    return right > 0 ? { top: 80, bottom: 80, left: 80, right: right + 40 } : 80;
  };

  useImperativeHandle(handle, () => ({
    recenter() {
      const map = mapRef.current;
      if (map && boundsRef.current) {
        map.fitBounds(boundsRef.current, { padding: padding(), maxZoom: 12, duration: 500 });
      }
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
            openstreetmap: {
              type: "raster",
              tiles: ["https://tile.openstreetmap.org/{z}/{x}/{y}.png"],
              tileSize: 256,
              attribution: "© OpenStreetMap contributors",
            },
          },
          layers: [{ id: "openstreetmap", type: "raster", source: "openstreetmap" }],
        },
        center: [DEFAULT_DEPARTURE.longitude, DEFAULT_DEPARTURE.latitude],
        zoom: 10,
        minZoom: 5,
        attributionControl: {},
      });
      mapRef.current = map;
      map.addControl(new maplibregl.NavigationControl(), "top-right");

      map.on("load", () => {
        if (!map) return;
        departureMarker.current = new maplibregl.Marker({ color: "#1971c2" })
          .setLngLat([DEFAULT_DEPARTURE.longitude, DEFAULT_DEPARTURE.latitude])
          .addTo(map);
        destinationMarker.current = new maplibregl.Marker({ color: "#6741d9" })
          .setLngLat([DEFAULT_DESTINATION.longitude, DEFAULT_DESTINATION.latitude])
          .addTo(map);

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

        map.addSource("vr-checkpoints", { type: "geojson", data: EMPTY as never });
        map.addLayer({
          id: "vr-checkpoint-lines",
          type: "line",
          source: "vr-checkpoints",
          filter: ["all", ["==", ["geometry-type"], "LineString"], ["==", ["get", "rhumb"], false]],
          paint: {
            "line-color": ["match", ["get", "side"], "stbd", "#2f9e44", "port", "#d9480f", "#7048e8"],
            "line-width": 3,
            "line-opacity": 0.9,
            "line-dasharray": [1, 0],
          },
        });
        map.addLayer({
          id: "vr-checkpoint-rhumb-lines",
          type: "line",
          source: "vr-checkpoints",
          filter: ["all", ["==", ["geometry-type"], "LineString"], ["==", ["get", "rhumb"], true]],
          paint: { "line-color": "#7048e8", "line-width": 3, "line-opacity": 0.9, "line-dasharray": [2, 2] },
        });
        map.addLayer({
          id: "vr-checkpoint-marks",
          type: "circle",
          source: "vr-checkpoints",
          filter: ["all", ["==", ["geometry-type"], "Point"], ["==", ["get", "kind"], "mark"]],
          paint: {
            "circle-radius": 7,
            "circle-color": ["match", ["get", "mark"], "start", "#2f9e44", "end", "#e03131", "#7048e8"],
            "circle-stroke-color": "#ffffff",
            "circle-stroke-width": 2,
          },
        });
        map.addLayer({
          id: "vr-checkpoint-mark-labels",
          type: "symbol",
          source: "vr-checkpoints",
          filter: ["all", ["==", ["geometry-type"], "Point"], ["==", ["get", "kind"], "mark"]],
          layout: { "text-field": ["get", "markLabel"], "text-size": 10, "text-allow-overlap": true },
          paint: { "text-color": "#ffffff", "text-halo-color": "#212529", "text-halo-width": 1 },
        });
        map.addLayer({
          id: "vr-checkpoint-labels",
          type: "symbol",
          source: "vr-checkpoints",
          filter: ["all", ["==", ["geometry-type"], "Point"], ["==", ["get", "kind"], "label"]],
          layout: {
            "text-field": ["get", "label"],
            "text-size": 12,
            "text-offset": [0, -1.2],
            "text-anchor": "bottom",
            "text-allow-overlap": true,
          },
          paint: { "text-color": "#7048e8", "text-halo-color": "#ffffff", "text-halo-width": 2 },
        });

        // Zones interdites et tracé en cours : sous la route.
        map.addSource("zones", { type: "geojson", data: EMPTY as never });
        map.addLayer({
          id: "zones-fill",
          type: "fill",
          source: "zones",
          paint: {
            "fill-color": ["case", ["boolean", ["get", "enabled"], true], "#e03131", "#868e96"],
            "fill-opacity": ["case", ["boolean", ["get", "enabled"], true], 0.28, 0.12],
          },
        });
        map.addLayer({
          id: "zones-line",
          type: "line",
          source: "zones",
          paint: {
            "line-color": ["case", ["boolean", ["get", "enabled"], true], "#e03131", "#868e96"],
            "line-width": 2,
          },
        });
        map.addSource("zone-draft", { type: "geojson", data: EMPTY as never });
        map.addLayer({
          id: "zone-draft-line",
          type: "line",
          source: "zone-draft",
          filter: ["==", ["geometry-type"], "LineString"],
          paint: { "line-color": "#e03131", "line-width": 2, "line-dasharray": [2, 2] },
        });
        map.addLayer({
          id: "zone-draft-points",
          type: "circle",
          source: "zone-draft",
          filter: ["==", ["geometry-type"], "Point"],
          paint: {
            "circle-radius": ["case", ["boolean", ["get", "first"], false], 8, 4],
            "circle-color": ["case", ["boolean", ["get", "first"], false], "#ffffff", "#e03131"],
            "circle-stroke-color": "#e03131",
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
          paint: { "line-color": "#ffffff", "line-width": 6, "line-opacity": 0.85 },
        });
        map.addLayer({
          id: "gpx-line",
          type: "line",
          source: "gpx",
          filter: ["==", ["geometry-type"], "LineString"],
          layout: { "line-cap": "round", "line-join": "round" },
          paint: { "line-color": "#e64980", "line-width": 3, "line-opacity": 0.95 },
        });
        map.addLayer({
          id: "gpx-waypoints",
          type: "circle",
          source: "gpx",
          filter: ["==", ["geometry-type"], "Point"],
          paint: {
            "circle-radius": 5,
            "circle-color": "#e64980",
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
          paint: { "text-color": "#e64980", "text-halo-color": "#ffffff", "text-halo-width": 2 },
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
          paint: {
            "circle-radius": 7,
            "circle-color": "#f59f00",
            "circle-stroke-color": "#ffffff",
            "circle-stroke-width": 2,
          },
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
          },
          paint: { "text-color": "#145cff", "text-halo-color": "#ffffff", "text-halo-width": 2 },
        });

        // Position choisie dans le tableau des détails : au-dessus de tout le reste.
        map.addSource("route-highlight", { type: "geojson", data: EMPTY as never });
        map.addLayer({
          id: "route-highlight-halo",
          type: "circle",
          source: "route-highlight",
          paint: { "circle-radius": 16, "circle-color": "#145cff", "circle-opacity": 0.25 },
        });
        map.addLayer({
          id: "route-highlight",
          type: "circle",
          source: "route-highlight",
          paint: {
            "circle-radius": 8,
            "circle-color": "#145cff",
            "circle-stroke-color": "#ffffff",
            "circle-stroke-width": 3,
          },
        });

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
      raceStartMarker.current = new api.Marker({ color: "#f59f00" }).addTo(map);
      raceStartMarker.current.getElement().title = "Départ de la course";
    }
    raceStartMarker.current.setLngLat([raceStartLon, raceStartLat]);
    map.triggerRepaint();
  }, [ready, raceStartLat, raceStartLon]);

  // Portes Virtual Regatta.
  useEffect(() => {
    const map = mapRef.current;
    const api = apiRef.current;
    if (!ready || !map || !api) return;
    const features = checkpointFeatures(vrState);
    setData(map, "vr-checkpoints", { type: "FeatureCollection", features });
    const coordinates = features
      .filter((feature) => feature.geometry.type === "LineString")
      .flatMap((feature) => feature.geometry.coordinates as number[][]);
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
    setData(map, "zones", zonesFeatureCollection(zones));
  }, [ready, zones]);

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
    const right = rightPaddingRef.current();
    const visible = map.getBounds();
    const size = map.getContainer();
    const point = map.project(center);
    const hidden = point.x > size.clientWidth - right - 40 || !visible.contains(center);
    if (hidden) {
      map.easeTo({ center, padding: { top: 0, bottom: 0, left: 0, right }, duration: 400 });
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
      return;
    }

    const { result } = frame;
    const route = frame.showRoute ? result.route || [] : [];
    const position = result.position || DEFAULT_DEPARTURE;
    const destination = result.destination || DEFAULT_DESTINATION;
    const startedAt = result.route?.[0]?.instant;

    if (frame.key !== renderedKey.current || (result.isochrones?.length || 0) < renderedCount.current) {
      renderedKey.current = frame.key;
      renderedCount.current = 0;
      renderedFeatures.current = [];
    }
    const direction = {
      x: destination.longitude - position.longitude,
      y: destination.latitude - position.latitude,
    };
    const isochrones: Isochrone[] = result.isochrones || [];
    const startedAtMilliseconds = startedAt ? Date.parse(startedAt) : Number.NaN;
    for (let index = renderedCount.current; index < isochrones.length; index += 1) {
      const isochrone = isochrones[index];
      const contour = forwardContour(isochrone.contour || [], direction);
      const visibleContour = contour.length > 1 ? contour : isochrone.contour || [];
      const minutes = Number.isFinite(startedAtMilliseconds)
        ? Math.round((Date.parse(isochrone.instant) - startedAtMilliseconds) / 60000)
        : 0;
      const coordinates = visibleContour.map((point) => [point.longitude, point.latitude]);
      const total = isochrone.totalLayers || isochrones.length || 1;
      const color = isochroneTimeColor((index + 1) / total);
      if (coordinates.length > 1) {
        renderedFeatures.current.push({
          type: "Feature",
          properties: { major: (index + 1) % 6 === 0, color },
          geometry: { type: "LineString", coordinates },
        });
      }
    }
    renderedCount.current = isochrones.length;

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
    setData(map, "route-points", {
      type: "FeatureCollection",
      features: route
        .filter((point, index) => index > 0 && !point.terminal)
        .map((point) => {
          const minutes = elapsedMinutes(route[0]?.instant, point.instant);
          return {
            type: "Feature",
            properties: { label: minutes > 0 && minutes % 30 === 0 ? `+${minutes} min` : "" },
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

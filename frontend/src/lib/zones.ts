import type { Point } from "./types.ts";

/** Polygone tracé sur la carte, que la route ne doit pas traverser. */
export type Zone = {
  id: string;
  name: string;
  enabled: boolean;
  points: Point[];
  /** "game" pour une zone reçue de Virtual Regatta, absent pour une zone dessinée. */
  source?: "game";
};

export const DISABLED_GAME_ZONES_KEY = "routeur.disabledGameZones";

/** Identifiant stable d'une zone du jeu : son nom et son premier sommet. */
export function gameZoneId(zone: { name: string; points: Point[] }): string {
  const first = zone.points[0];
  return `vr:${zone.name}:${first.latitude.toFixed(4)},${first.longitude.toFixed(4)}`;
}

/** Zones reçues du jeu ; les identifiants désactivés par l'utilisateur sont décochés. */
export function gameZonesFrom(
  state: { zones?: { name: string; points: Point[] }[] } | undefined,
  disabled: Set<string>,
): Zone[] {
  const seen = new Set<string>();
  const zones: Zone[] = [];
  for (const [index, zone] of (state?.zones || []).entries()) {
    const valid = Array.isArray(zone?.points) ? zone.points.filter(isPoint) : [];
    if (valid.length < MIN_ZONE_POINTS) continue;
    const name = (zone.name || "").trim() || `Zone de la course ${index + 1}`;
    let id = gameZoneId({ name, points: valid });
    while (seen.has(id)) id += "+";
    seen.add(id);
    zones.push({ id, name, enabled: !disabled.has(id), points: valid, source: "game" });
  }
  return zones;
}

export function parseDisabledIds(raw: string | null): Set<string> {
  try {
    const parsed: unknown = raw ? JSON.parse(raw) : [];
    return new Set(Array.isArray(parsed) ? parsed.filter((item): item is string => typeof item === "string") : []);
  } catch {
    return new Set();
  }
}

export const ZONES_STORAGE_KEY = "routeur.exclusionZones";
export const MIN_ZONE_POINTS = 3;
const MAX_ZONES = 50;
const MAX_POINTS = 500;

function isPoint(value: unknown): value is Point {
  const point = value as Point | null;
  return (
    !!point &&
    Number.isFinite(point.latitude) &&
    Number.isFinite(point.longitude) &&
    Math.abs(point.latitude) <= 90 &&
    Math.abs(point.longitude) <= 180
  );
}

/** Relit les zones enregistrées en ignorant tout ce qui est invalide. */
export function parseStoredZones(raw: string | null): Zone[] {
  if (!raw) return [];
  try {
    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    const zones: Zone[] = [];
    for (const item of parsed.slice(0, MAX_ZONES)) {
      const zone = item as Partial<Zone> | null;
      if (!zone || typeof zone.id !== "string" || !Array.isArray(zone.points)) continue;
      const points = zone.points.filter(isPoint).slice(0, MAX_POINTS);
      if (points.length < MIN_ZONE_POINTS) continue;
      zones.push({
        id: zone.id,
        name: typeof zone.name === "string" ? zone.name : "Zone",
        enabled: zone.enabled !== false,
        points,
      });
    }
    return zones;
  } catch {
    return [];
  }
}

export function nextZoneName(zones: Zone[]): string {
  const used = new Set(zones.map((zone) => zone.name));
  for (let index = zones.length + 1; ; index += 1) {
    const name = `Zone ${index}`;
    if (!used.has(name)) return name;
  }
}

export function createZone(zones: Zone[], points: Point[], id: string): Zone | null {
  if (points.length < MIN_ZONE_POINTS || points.length > MAX_POINTS || zones.length >= MAX_ZONES) return null;
  return { id, name: nextZoneName(zones), enabled: true, points: points.map((point) => ({ ...point })) };
}

/** Zones actives, au format attendu par l'API. */
export function zonesForRequest(zones: Zone[]): { name: string; points: Point[] }[] {
  return zones
    .filter((zone) => zone.enabled && zone.points.length >= MIN_ZONE_POINTS)
    .map((zone) => ({ name: zone.name, points: zone.points }));
}

type Feature = {
  type: "Feature";
  properties: Record<string, unknown>;
  geometry: { type: "Polygon"; coordinates: number[][][] } | { type: "LineString"; coordinates: number[][] } | { type: "Point"; coordinates: number[] };
};

export function zonesFeatureCollection(zones: Zone[]): { type: "FeatureCollection"; features: Feature[] } {
  return {
    type: "FeatureCollection",
    features: zones.map((zone) => {
      const ring = zone.points.map((point) => [point.longitude, point.latitude]);
      return {
        type: "Feature",
        properties: { id: zone.id, name: zone.name, enabled: zone.enabled },
        geometry: { type: "Polygon", coordinates: [[...ring, ring[0]]] },
      };
    }),
  };
}

/** Tracé en cours : ligne reliant les points, et un repère par sommet. */
export function draftFeatureCollection(draft: Point[]): { type: "FeatureCollection"; features: Feature[] } {
  const features: Feature[] = [];
  const coordinates = draft.map((point) => [point.longitude, point.latitude]);
  if (coordinates.length > 1) {
    const closing = coordinates.length >= MIN_ZONE_POINTS ? [...coordinates, coordinates[0]] : coordinates;
    features.push({ type: "Feature", properties: {}, geometry: { type: "LineString", coordinates: closing } });
  }
  coordinates.forEach((coordinate, index) => {
    features.push({
      type: "Feature",
      properties: { first: index === 0 },
      geometry: { type: "Point", coordinates: coordinate },
    });
  });
  return { type: "FeatureCollection", features };
}

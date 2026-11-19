import type { Point } from "./types.ts";

/** Une trace GPX décodée : des polylignes (segments de trace ou routes) et des
 * points de passage nommés. */
export type GpxWaypoint = { point: Point; name?: string };
export type GpxGeometry = { lines: Point[][]; waypoints: GpxWaypoint[] };

type GpxFeature = {
  type: "Feature";
  properties: Record<string, unknown>;
  geometry:
    | { type: "LineString"; coordinates: number[][] }
    | { type: "Point"; coordinates: number[] };
};
export type GpxFeatureCollection = { type: "FeatureCollection"; features: GpxFeature[] };

// Au-delà de cette taille, une trace est échantillonnée : MapLibre reste fluide
// et la forme générale est préservée.
const MAX_POINTS_PER_LINE = 20000;

function decodeXml(value: string): string {
  return value
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&quot;/g, '"')
    .replace(/&apos;/g, "'")
    .replace(/&#(\d+);/g, (_, code: string) => String.fromCodePoint(Number(code)))
    .replace(/&#x([0-9a-fA-F]+);/g, (_, code: string) => String.fromCodePoint(parseInt(code, 16)))
    .replace(/&amp;/g, "&");
}

/** Lit lat/lon sur une balise ouvrante (trkpt, rtept ou wpt), quel que soit
 * l'ordre des attributs ou le type de guillemets. */
function readLatLon(openingTag: string): Point | null {
  const lat = /\blat\s*=\s*["']([^"']+)["']/i.exec(openingTag);
  const lon = /\blon\s*=\s*["']([^"']+)["']/i.exec(openingTag);
  if (!lat || !lon) return null;
  const latitude = Number(lat[1]);
  const longitude = Number(lon[1]);
  if (!Number.isFinite(latitude) || !Number.isFinite(longitude)) return null;
  if (latitude < -90 || latitude > 90 || longitude < -180 || longitude > 180) return null;
  return { latitude, longitude };
}

/** Contenu de chaque bloc <tag>…</tag>, dans l'ordre du document. */
function innerBlocks(xml: string, tag: string): string[] {
  const pattern = new RegExp(`<${tag}\\b[^>]*>([\\s\\S]*?)</${tag}>`, "gi");
  const blocks: string[] = [];
  for (let match = pattern.exec(xml); match; match = pattern.exec(xml)) {
    blocks.push(match[1]);
  }
  return blocks;
}

/** Toutes les balises ouvrantes <tag …> d'un fragment. */
function openingTags(xml: string, tag: string): string[] {
  return xml.match(new RegExp(`<${tag}\\b[^>]*?/?>`, "gi")) ?? [];
}

function decimate(points: Point[], max: number): Point[] {
  const stride = Math.ceil(points.length / max);
  const kept = points.filter((_, index) => index % stride === 0);
  const last = points[points.length - 1];
  if (kept[kept.length - 1] !== last) kept.push(last);
  return kept;
}

function pointsOf(container: string, tag: string): Point[] {
  const points: Point[] = [];
  for (const opening of openingTags(container, tag)) {
    const point = readLatLon(opening);
    if (point) points.push(point);
  }
  return points.length > MAX_POINTS_PER_LINE ? decimate(points, MAX_POINTS_PER_LINE) : points;
}

/** Décode un fichier GPX. Les segments de trace et les routes deviennent des
 * polylignes (au moins deux points) ; les points de passage sont conservés
 * avec leur nom éventuel. Tolérant aux fichiers partiellement malformés. */
export function parseGpx(text: string): GpxGeometry {
  const lines: Point[][] = [];
  for (const segment of innerBlocks(text, "trkseg")) {
    const points = pointsOf(segment, "trkpt");
    if (points.length >= 2) lines.push(points);
  }
  for (const route of innerBlocks(text, "rte")) {
    const points = pointsOf(route, "rtept");
    if (points.length >= 2) lines.push(points);
  }

  const waypoints: GpxWaypoint[] = [];
  const pattern = /<wpt\b[^>]*?(?:\/>|>([\s\S]*?)<\/wpt>)/gi;
  for (let match = pattern.exec(text); match; match = pattern.exec(text)) {
    const point = readLatLon(match[0]);
    if (!point) continue;
    let name: string | undefined;
    if (match[1]) {
      const nameMatch = /<name>([\s\S]*?)<\/name>/i.exec(match[1]);
      if (nameMatch) {
        const decoded = decodeXml(nameMatch[1]).trim();
        if (decoded) name = decoded;
      }
    }
    waypoints.push(name ? { point, name } : { point });
  }

  return { lines, waypoints };
}

/** Convertit une géométrie GPX en FeatureCollection pour MapLibre. */
export function gpxFeatureCollection(geometry: GpxGeometry): GpxFeatureCollection {
  const features: GpxFeature[] = [];
  for (const line of geometry.lines) {
    features.push({
      type: "Feature",
      properties: { kind: "track" },
      geometry: { type: "LineString", coordinates: line.map((p) => [p.longitude, p.latitude]) },
    });
  }
  for (const waypoint of geometry.waypoints) {
    features.push({
      type: "Feature",
      properties: { kind: "waypoint", name: waypoint.name ?? "" },
      geometry: { type: "Point", coordinates: [waypoint.point.longitude, waypoint.point.latitude] },
    });
  }
  return { type: "FeatureCollection", features };
}

/** Nombre total de points d'une géométrie (traces + points de passage). */
export function gpxPointCount(geometry: GpxGeometry): number {
  return geometry.lines.reduce((total, line) => total + line.length, 0) + geometry.waypoints.length;
}

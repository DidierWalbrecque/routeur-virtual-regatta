import type { Point } from "./types.ts";

// Accepte deux écritures DMS : l'hémisphère juste après les degrés
// (13°N 07'16") ou en fin de coordonnée (13°07'16.00"N), avec des secondes
// éventuellement décimales. Les deux coordonnées sont séparées par un tiret.
const COORDINATE_PATTERN = /^\s*(\d{1,3})\s*°\s*(\d{1,2})\s*['’′]\s*(\d+(?:\.\d+)?)\s*["”″]\s*$/;

function parseCoordinate(token: string, isLatitude: boolean): number | null {
  const hemispheres = token.match(/[NSEW]/gi);
  if (!hemispheres || hemispheres.length !== 1) return null;
  const hemisphere = hemispheres[0].toUpperCase();
  const allowed = isLatitude ? ["N", "S"] : ["E", "W"];
  if (!allowed.includes(hemisphere)) return null;
  const match = token.replace(/[NSEW]/gi, " ").match(COORDINATE_PATTERN);
  if (!match) return null;
  const [, degrees, minutes, seconds] = match;
  if (Number(minutes) >= 60 || Number(seconds) >= 60) return null;
  const value = Number(degrees) + Number(minutes) / 60 + Number(seconds) / 3600;
  return hemisphere === "S" || hemisphere === "W" ? -value : value;
}

export function parseVirtualRegattaPosition(value: string): Point | null {
  const parts = value.trim().split(/\s*[-–—]\s*/);
  if (parts.length !== 2) return null;
  const latitude = parseCoordinate(parts[0], true);
  const longitude = parseCoordinate(parts[1], false);
  if (latitude === null || longitude === null) return null;
  return { latitude, longitude };
}

function formatVirtualRegattaCoordinate(value: number, isLatitude: boolean): string {
  const absolute = Math.abs(Number(value));
  let degrees = Math.floor(absolute);
  const minutesValue = (absolute - degrees) * 60;
  let minutes = Math.floor(minutesValue);
  let seconds = Math.round((minutesValue - minutes) * 60 * 100) / 100;
  if (seconds >= 60) {
    seconds -= 60;
    minutes += 1;
  }
  if (minutes >= 60) {
    minutes -= 60;
    degrees += 1;
  }
  const hemisphere = isLatitude ? (Number(value) < 0 ? "S" : "N") : Number(value) < 0 ? "W" : "E";
  const secondsText = seconds.toFixed(2).padStart(5, "0");
  return `${degrees}°${String(minutes).padStart(2, "0")}'${secondsText}"${hemisphere}`;
}

export function formatVirtualRegattaPosition(position: Partial<Point> | null | undefined): string {
  if (!position || !Number.isFinite(Number(position.latitude)) || !Number.isFinite(Number(position.longitude))) return "";
  return `${formatVirtualRegattaCoordinate(Number(position.latitude), true)} - ${formatVirtualRegattaCoordinate(Number(position.longitude), false)}`;
}

export function formatLocalDateTime(date: Date): string {
  const pad = (value: number) => String(value).padStart(2, "0");
  return `${pad(date.getDate())}/${pad(date.getMonth() + 1)}/${date.getFullYear()} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

export function parseLocalDate(value: string): string {
  const match = value.trim().match(/^(\d{2})\/(\d{2})\/(\d{4})\s+(\d{2}):(\d{2})$/);
  if (!match) throw new Error("Date invalide : utilisez JJ/MM/AAAA HH:MM.");
  const [, day, month, year, hour, minute] = match;
  const date = new Date(Number(year), Number(month) - 1, Number(day), Number(hour), Number(minute));
  if (Number.isNaN(date.getTime())) throw new Error("Date invalide : utilisez JJ/MM/AAAA HH:MM.");
  return date.toISOString();
}

/** Distance approchée entre deux points, en milles nautiques (suffisante à l'échelle d'un pas). */
export function approximateDistanceNM(a: Point, b: Point): number {
  const latitude = ((a.latitude + b.latitude) / 2) * (Math.PI / 180);
  let deltaLongitude = b.longitude - a.longitude;
  if (deltaLongitude > 180) deltaLongitude -= 360;
  if (deltaLongitude < -180) deltaLongitude += 360;
  return 60 * Math.hypot(b.latitude - a.latitude, deltaLongitude * Math.cos(latitude));
}

/**
 * Coupe un contour d'isochrone là où deux points voisins sont anormalement
 * éloignés : ce saut correspond à une île ou une zone que le front contourne,
 * et le relier tracerait une ligne à travers la terre. Les points d'un front
 * sont espacés irrégulièrement : un saut n'est coupé que s'il est à la fois
 * très long devant l'espacement habituel et une part notable du contour.
 */
export function splitContour(points: Point[], factor = 6, shareOfContour = 0.2): Point[][] {
  if (points.length < 2) return points.length ? [points] : [];
  const lengths = points.slice(1).map((point, index) => approximateDistanceNM(points[index], point));
  const sorted = [...lengths].sort((a, b) => a - b);
  const median = sorted[Math.floor(sorted.length / 2)];
  const total = lengths.reduce((sum, length) => sum + length, 0);
  const limit = Math.max(median * factor, total * shareOfContour);
  const parts: Point[][] = [[points[0]]];
  lengths.forEach((length, index) => {
    if (median > 0 && length > limit) parts.push([]);
    parts[parts.length - 1].push(points[index + 1]);
  });
  return parts.filter((part) => part.length > 1);
}

// Garde l'arc de l'isochrone tourné vers la destination.
export function forwardContour(ring: Point[], direction: { x: number; y: number }): Point[] {
  const vertices =
    ring.length > 1 &&
    ring[0].latitude === ring[ring.length - 1].latitude &&
    ring[0].longitude === ring[ring.length - 1].longitude
      ? ring.slice(0, -1)
      : ring;
  if (vertices.length < 3 || Math.hypot(direction.x, direction.y) < 0.001) return [];
  const area = vertices.reduce((sum, point, index) => {
    const next = vertices[(index + 1) % vertices.length];
    return sum + point.longitude * next.latitude - next.longitude * point.latitude;
  }, 0);
  const visible = vertices.map((point, index) => {
    const next = vertices[(index + 1) % vertices.length];
    return Math.sign(area) * ((next.latitude - point.latitude) * direction.x - (next.longitude - point.longitude) * direction.y) > 0;
  });
  const start = visible.findIndex((value, index) => value && !visible[(index + visible.length - 1) % visible.length]);
  if (start < 0) return [];
  const arc = [vertices[start]];
  for (let offset = 0; offset < vertices.length && visible[(start + offset) % vertices.length]; offset++) {
    arc.push(vertices[(start + offset + 1) % vertices.length]);
  }
  return arc;
}

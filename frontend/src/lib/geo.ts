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

/**
 * Coupe un arc du contour aux côtés que le backend signale comme traversant
 * une terre ou une zone interdite (`breaks` : indices dans `ring`). Les autres
 * côtés, même longs, sont de vrais bords du front et restent tracés.
 */
export function cutContourAtBreaks(arc: Point[], ring: Point[], breaks: number[]): Point[][] {
  const key = (a: Point, b: Point) => `${a.latitude},${a.longitude}|${b.latitude},${b.longitude}`;
  const blocked = new Set<string>();
  for (const index of breaks) {
    if (index < 0 || index + 1 >= ring.length) continue;
    blocked.add(key(ring[index], ring[index + 1]));
    blocked.add(key(ring[index + 1], ring[index]));
  }
  if (arc.length < 2) return [];
  const parts: Point[][] = [[arc[0]]];
  for (let index = 1; index < arc.length; index += 1) {
    if (blocked.has(key(arc[index - 1], arc[index]))) parts.push([]);
    parts[parts.length - 1].push(arc[index]);
  }
  return parts.filter((part) => part.length > 1);
}

// Distance en milles d'un point au segment [a, b], dans un repère local
// équirectangulaire centré sur le point (suffisant à l'échelle d'un front).
function distanceToSegmentNM(point: Point, a: Point, b: Point): number {
  const scale = Math.cos(point.latitude * (Math.PI / 180));
  const wrap = (delta: number) => (delta > 180 ? delta - 360 : delta < -180 ? delta + 360 : delta);
  const ax = wrap(a.longitude - point.longitude) * scale;
  const ay = a.latitude - point.latitude;
  const bx = wrap(b.longitude - point.longitude) * scale;
  const by = b.latitude - point.latitude;
  const dx = bx - ax;
  const dy = by - ay;
  const lengthSquared = dx * dx + dy * dy;
  const t = lengthSquared > 0 ? Math.max(0, Math.min(1, -(ax * dx + ay * dy) / lengthSquared)) : 0;
  return 60 * Math.hypot(ax + t * dx, ay + t * dy);
}

function distanceToRingNM(point: Point, ring: Point[]): number {
  if (ring.length === 1) return approximateDistanceNM(point, ring[0]);
  let best = Number.POSITIVE_INFINITY;
  for (let index = 0; index + 1 < ring.length; index += 1) {
    best = Math.min(best, distanceToSegmentNM(point, ring[index], ring[index + 1]));
  }
  return best;
}

/** Avancée médiane (milles) des points d'un arc depuis le front précédent. */
export function medianAdvanceNM(points: Point[], previousRing: Point[]): number {
  if (points.length === 0 || previousRing.length === 0) return 0;
  const sorted = points.map((point) => distanceToRingNM(point, previousRing)).sort((a, b) => a - b);
  return sorted[Math.floor(sorted.length / 2)];
}

/** Intervalles d'affichage possibles (minutes), multiples du pas de calcul, croissants. */
export function isochroneIntervals(stepMinutes: number): number[] {
  if (!(stepMinutes > 0)) return [];
  const intervals = [10, 20, 30, 60, 120, 180, 360, 720, 1440].map((minutes) =>
    Math.max(stepMinutes, Math.ceil(minutes / stepMinutes) * stepMinutes),
  );
  return [...new Set(intervals)];
}

/**
 * Plus petit intervalle d'affichage des isochrones (minutes, multiple du pas
 * de calcul) qui laisse au moins `minSpacingPx` pixels entre deux tracés,
 * pour une vitesse d'avancée du front donnée et une échelle de carte donnée.
 */
export function isochroneDisplayInterval(
  stepMinutes: number,
  advanceNMPerMinute: number,
  pixelsPerNM: number,
  minSpacingPx = 12,
): number {
  if (!(stepMinutes > 0)) return 1;
  if (!(advanceNMPerMinute > 0) || !(pixelsPerNM > 0)) return stepMinutes;
  const candidates = isochroneIntervals(stepMinutes);
  for (const interval of candidates) {
    if (advanceNMPerMinute * interval * pixelsPerNM >= minSpacingPx) return interval;
  }
  return candidates[candidates.length - 1];
}

/**
 * Retire des morceaux d'isochrone les côtés où le front n'a pas avancé depuis
 * l'isochrone précédente : le long d'une côte, d'une zone interdite ou d'un
 * flanc bloqué, les enveloppes successives se superposent et forment une bande
 * pleine. Un côté est gardé dès qu'une de ses extrémités s'est écartée du front
 * précédent d'au moins `share` fois l'avancée médiane de l'arc.
 */
export function trimStalledContour(parts: Point[][], previousRing: Point[], share = 0.25): Point[][] {
  if (previousRing.length === 0) return parts;
  const advances = parts.map((part) => part.map((point) => distanceToRingNM(point, previousRing)));
  const sorted = advances.flat().sort((a, b) => a - b);
  if (sorted.length === 0) return parts;
  const median = sorted[Math.floor(sorted.length / 2)];
  if (!(median > 0)) return parts;
  const limit = median * share;
  // Les côtés de l'enveloppe peuvent être très longs : une extrémité avance
  // alors que le reste longe le front précédent. On les redécoupe en morceaux
  // de la taille du seuil pour juger chaque portion.
  const dense = parts.map((part) => {
    const points: Point[] = part.length ? [part[0]] : [];
    for (let index = 1; index < part.length; index += 1) {
      const a = part[index - 1];
      const b = part[index];
      const pieces = Math.min(200, Math.ceil(approximateDistanceNM(a, b) / limit));
      for (let piece = 1; piece < pieces; piece += 1) {
        const f = piece / pieces;
        points.push({ latitude: a.latitude + (b.latitude - a.latitude) * f, longitude: a.longitude + (b.longitude - a.longitude) * f });
      }
      points.push(b);
    }
    return points;
  });
  const trimmed: Point[][] = [];
  dense.forEach((part) => {
    const advance = part.map((point) => distanceToRingNM(point, previousRing));
    let current: Point[] = [];
    for (let index = 1; index < part.length; index += 1) {
      if (Math.max(advance[index - 1], advance[index]) >= limit) {
        if (current.length === 0) current.push(part[index - 1]);
        current.push(part[index]);
      } else if (current.length > 0) {
        trimmed.push(current);
        current = [];
      }
    }
    if (current.length > 1) trimmed.push(current);
  });
  return trimmed;
}

// Garde l'arc de l'isochrone tourné vers la destination : les côtés dont la
// normale extérieure fait au plus `maxAngle` degrés avec la direction. À 90°,
// les flancs de l'enveloppe disparaissent et le front paraît troué.
export function forwardContour(ring: Point[], direction: { x: number; y: number }, maxAngle = 90): Point[] {
  const threshold = Math.cos((maxAngle * Math.PI) / 180);
  const directionLength = Math.hypot(direction.x, direction.y);
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
    const edgeLength = Math.hypot(next.latitude - point.latitude, next.longitude - point.longitude);
    if (edgeLength === 0) return false;
    // Cosinus de l'angle entre la normale extérieure du côté et la direction.
    const facing =
      (Math.sign(area) * ((next.latitude - point.latitude) * direction.x - (next.longitude - point.longitude) * direction.y)) /
      (edgeLength * directionLength);
    return facing > threshold;
  });
  const start = visible.findIndex((value, index) => value && !visible[(index + visible.length - 1) % visible.length]);
  if (start < 0) return [];
  const arc = [vertices[start]];
  for (let offset = 0; offset < vertices.length && visible[(start + offset) % vertices.length]; offset++) {
    arc.push(vertices[(start + offset + 1) % vertices.length]);
  }
  return arc;
}

/** Cercle de rayon donné (en milles nautiques) autour d'un point, fermé, pour le dessiner sur la carte. */
export function circlePolygon(center: Point, radiusNM: number, steps = 64): Point[] {
  const latitudeRadians = center.latitude * (Math.PI / 180);
  const degreesLatitude = radiusNM / 60;
  const degreesLongitude = radiusNM / (60 * Math.max(0.01, Math.cos(latitudeRadians)));
  const points: Point[] = [];
  for (let step = 0; step <= steps; step += 1) {
    const angle = (step / steps) * 2 * Math.PI;
    points.push({
      latitude: center.latitude + degreesLatitude * Math.cos(angle),
      longitude: center.longitude + degreesLongitude * Math.sin(angle),
    });
  }
  return points;
}

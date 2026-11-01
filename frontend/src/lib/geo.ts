import type { Point } from "./types.ts";

const POSITION_PATTERN =
  /^(\d{1,3})°\s*([NS])\s*(\d{1,2})['’]\s*(\d+(?:\.\d+)?)"\s*-\s*(\d{1,3})°\s*([EW])\s*(\d{1,2})['’]\s*(\d+(?:\.\d+)?)"$/i;

export function parseVirtualRegattaPosition(value: string): Point | null {
  const match = value.trim().match(POSITION_PATTERN);
  if (!match) return null;
  const [, latitudeDegrees, latitudeHemisphere, latitudeMinutes, latitudeSeconds, longitudeDegrees, longitudeHemisphere, longitudeMinutes, longitudeSeconds] = match;
  const latitude = Number(latitudeDegrees) + Number(latitudeMinutes) / 60 + Number(latitudeSeconds) / 3600;
  const longitude = Number(longitudeDegrees) + Number(longitudeMinutes) / 60 + Number(longitudeSeconds) / 3600;
  return {
    latitude: latitudeHemisphere.toUpperCase() === "S" ? -latitude : latitude,
    longitude: longitudeHemisphere.toUpperCase() === "W" ? -longitude : longitude,
  };
}

function formatVirtualRegattaCoordinate(value: number, isLatitude: boolean): string {
  const absolute = Math.abs(Number(value));
  let degrees = Math.floor(absolute);
  const minutesValue = (absolute - degrees) * 60;
  let minutes = Math.floor(minutesValue);
  let seconds = Math.round((minutesValue - minutes) * 60);
  if (seconds >= 60) {
    seconds = 0;
    minutes += 1;
  }
  if (minutes >= 60) {
    minutes = 0;
    degrees += 1;
  }
  const hemisphere = isLatitude ? (Number(value) < 0 ? "S" : "N") : Number(value) < 0 ? "W" : "E";
  const degreeText = isLatitude ? String(degrees) : String(degrees).padStart(3, "0");
  return `${degreeText}°${hemisphere} ${String(minutes).padStart(2, "0")}'${String(seconds).padStart(2, "0")}"`;
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

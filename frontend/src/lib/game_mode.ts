import type { GameOrder } from "./game_orders.ts";
import type { ReportRow } from "./route_details.ts";
import type { Point, RoutePoint } from "./types.ts";

/**
 * Mode jeu : la feuille de route suivie en direct. Les ordres sont situés par
 * rapport à l'heure actuelle, et la position du bateau dans le jeu est
 * comparée à la route prévue.
 */

/** Où en est un ordre à l'heure actuelle. */
export type OrderStatus = "past" | "current" | "next" | "later";

export type TimedOrder = {
  order: GameOrder;
  start: number;
  /** Fin de validité (début de l'ordre suivant ou fin de la route), ou NaN. */
  end: number;
  status: OrderStatus;
};

/**
 * Situe chaque ordre par rapport à `now` : passé, en cours, prochain (le
 * premier à venir) ou plus tard.
 */
export function timeOrders(orders: GameOrder[], now: number): TimedOrder[] {
  let nextFound = false;
  return orders.map((order) => {
    const start = Date.parse(order.instant);
    const end = order.durationMinutes === null ? Number.NaN : start + order.durationMinutes * 60_000;
    let status: OrderStatus;
    if (start > now) {
      status = nextFound ? "later" : "next";
      nextFound = true;
    } else if (Number.isFinite(end) && end <= now) {
      status = "past";
    } else {
      status = "current";
    }
    return { order, start, end, status };
  });
}

/** Compte à rebours « 00:42:10 », ou « 1 j 03:12:00 » au-delà d'un jour. */
export function formatCountdown(milliseconds: number): string {
  const total = Math.max(0, Math.floor(milliseconds / 1000));
  const days = Math.floor(total / 86_400);
  const hours = Math.floor((total % 86_400) / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const seconds = total % 60;
  const clock = [hours, minutes, seconds].map((value) => String(value).padStart(2, "0")).join(":");
  return days > 0 ? `${days} j ${clock}` : clock;
}

/** « dans 42 min », « dans 5 h 22 », « dans 2 j 3 h ». */
export function formatTimeUntil(milliseconds: number): string {
  const minutes = Math.max(0, Math.round(milliseconds / 60_000));
  if (minutes < 60) return `dans ${minutes} min`;
  const days = Math.floor(minutes / 1440);
  const hours = Math.floor((minutes % 1440) / 60);
  if (days > 0) return hours ? `dans ${days} j ${hours} h` : `dans ${days} j`;
  const rest = minutes % 60;
  return rest ? `dans ${hours} h ${String(rest).padStart(2, "0")}` : `dans ${hours} h`;
}

/** « 1 h 25 », « 6 min » : durée d'un retard ou d'une avance, sans signe. */
export function formatGap(minutes: number): string {
  const total = Math.round(Math.abs(minutes));
  if (total < 60) return `${total} min`;
  const hours = Math.floor(total / 60);
  const rest = total % 60;
  return rest ? `${hours} h ${String(rest).padStart(2, "0")}` : `${hours} h`;
}

/**
 * Stamina dépensée par la manœuvre qui ouvre un ordre : la baisse entre le
 * point où l'ordre commence et le point suivant. Null sans baisse mesurable.
 */
export function maneuverCost(rows: ReportRow[], index: number): number | null {
  const before = rows[index]?.stamina;
  const after = rows[index + 1]?.stamina;
  if (before === null || before === undefined || after === null || after === undefined) return null;
  const cost = before - after;
  return cost > 0.05 ? cost : null;
}

/** Position du bateau comparée à la route prévue. */
export type RouteGap = {
  /** Distance la plus courte entre le bateau et la route, en milles. */
  crossTrackNM: number;
  /** Heure à laquelle la route prévoyait de passer au point le plus proche (ms). */
  plannedTime: number;
};

/**
 * Point de la route le plus proche du bateau, et l'heure prévue pour y passer.
 * Les longitudes sont ramenées autour du bateau (passage de l'antiméridien) et
 * projetées localement : l'erreur reste faible à l'échelle d'un pas de route.
 */
export function routeGap(route: RoutePoint[], position: Point): RouteGap | null {
  if (route.length === 0) return null;
  const cosine = Math.cos((position.latitude * Math.PI) / 180);
  const project = (point: Point) => {
    let longitude = point.longitude - position.longitude;
    if (longitude > 180) longitude -= 360;
    if (longitude < -180) longitude += 360;
    return { x: longitude * cosine * 60, y: (point.latitude - position.latitude) * 60 };
  };
  let best: RouteGap | null = null;
  const consider = (distance: number, time: number) => {
    if (Number.isFinite(time) && (!best || distance < best.crossTrackNM)) best = { crossTrackNM: distance, plannedTime: time };
  };
  if (route.length === 1) {
    const only = project(route[0].position);
    consider(Math.hypot(only.x, only.y), Date.parse(route[0].instant));
    return best;
  }
  for (let index = 0; index < route.length - 1; index += 1) {
    const a = project(route[index].position);
    const b = project(route[index + 1].position);
    const dx = b.x - a.x;
    const dy = b.y - a.y;
    const length = dx * dx + dy * dy;
    const fraction = length > 0 ? Math.min(1, Math.max(0, -(a.x * dx + a.y * dy) / length)) : 0;
    const x = a.x + dx * fraction;
    const y = a.y + dy * fraction;
    const startTime = Date.parse(route[index].instant);
    const endTime = Date.parse(route[index + 1].instant);
    consider(Math.hypot(x, y), startTime + (endTime - startTime) * fraction);
  }
  return best;
}

/** Écart entre deux caps, de 0 à 180°. */
export function headingGap(a: number, b: number): number {
  const gap = Math.abs(a - b) % 360;
  return gap > 180 ? 360 - gap : gap;
}

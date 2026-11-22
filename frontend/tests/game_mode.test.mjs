import assert from "node:assert/strict";
import test from "node:test";

import {
  formatCountdown,
  formatGap,
  formatTimeUntil,
  headingGap,
  maneuverCost,
  routeGap,
  timeOrders,
} from "../src/lib/game_mode.ts";

const at = (minutes) => new Date(Date.UTC(2026, 10, 22, 12, minutes)).toISOString();
const order = (minutes, duration) => ({ index: minutes, instant: at(minutes), durationMinutes: duration });

test("timeOrders situe chaque ordre par rapport à l'heure actuelle", () => {
  const orders = [order(0, 20), order(20, 20), order(40, 20), order(60, 20)];
  const now = Date.parse(at(25));
  assert.deepEqual(
    timeOrders(orders, now).map((item) => item.status),
    ["past", "current", "next", "later"],
  );
  // Avant le départ : le premier ordre est le prochain.
  assert.equal(timeOrders(orders, Date.parse(at(0)) - 1)[0].status, "next");
  // Après la fin : tout est passé, aucun prochain ordre.
  assert.ok(timeOrders(orders, Date.parse(at(200))).every((item) => item.status === "past"));
});

test("formatCountdown, formatTimeUntil et formatGap", () => {
  assert.equal(formatCountdown(42 * 60_000 + 10_000), "00:42:10");
  assert.equal(formatCountdown(-5), "00:00:00");
  assert.equal(formatCountdown(27 * 3_600_000), "1 j 03:00:00");
  assert.equal(formatTimeUntil(42 * 60_000), "dans 42 min");
  assert.equal(formatTimeUntil(322 * 60_000), "dans 5 h 22");
  assert.equal(formatTimeUntil(51 * 3_600_000), "dans 2 j 3 h");
  assert.equal(formatGap(-6), "6 min");
  assert.equal(formatGap(85), "1 h 25");
});

test("maneuverCost lit la baisse de stamina du pas qui suit l'ordre", () => {
  const rows = [{ stamina: 100 }, { stamina: 83 }, { stamina: 84 }, { stamina: null }];
  assert.equal(maneuverCost(rows, 0), 17);
  assert.equal(maneuverCost(rows, 1), null);
  assert.equal(maneuverCost(rows, 2), null);
});

test("routeGap mesure l'écart à la route et l'heure prévue au point le plus proche", () => {
  const route = [
    { position: { latitude: 0, longitude: 0 }, instant: at(0) },
    { position: { latitude: 0, longitude: 1 }, instant: at(60) },
  ];
  const gap = routeGap(route, { latitude: 0.1, longitude: 0.5 });
  assert.ok(Math.abs(gap.crossTrackNM - 6) < 0.01);
  assert.equal(gap.plannedTime, Date.parse(at(30)));
  assert.equal(routeGap([], { latitude: 0, longitude: 0 }), null);
});

test("routeGap passe l'antiméridien", () => {
  const route = [
    { position: { latitude: 0, longitude: 179.5 }, instant: at(0) },
    { position: { latitude: 0, longitude: -179.5 }, instant: at(60) },
  ];
  const gap = routeGap(route, { latitude: 0, longitude: 180 });
  assert.ok(gap.crossTrackNM < 0.01);
  assert.equal(gap.plannedTime, Date.parse(at(30)));
});

test("headingGap", () => {
  assert.equal(headingGap(350, 10), 20);
  assert.equal(headingGap(90, 270), 180);
});

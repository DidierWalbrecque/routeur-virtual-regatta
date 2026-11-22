import assert from "node:assert/strict";
import test from "node:test";

import {
  formatDuration,
  formatLongInstant,
  groupOrdersByDay,
  maneuverFractions,
  routeHeadline,
  staminaPath,
  staminaSeries,
} from "../src/lib/route_brief.ts";

const local = (day, hour, minute = 0) => new Date(2026, 10, day, hour, minute).toISOString();
const row = (hour, stamina, maneuver = "") => ({ instant: local(22, hour), stamina, maneuver });

test("formatLongInstant et formatDuration", () => {
  assert.equal(formatLongInstant(local(23, 13, 4)), "lun. 23/11 · 13:04");
  assert.equal(formatLongInstant(""), "—");
  assert.equal(formatDuration(1440), "1 j");
  assert.equal(formatDuration(1620), "1 j 3 h");
  assert.equal(formatDuration(320), "5 h 20");
  assert.equal(formatDuration(40), "40 min");
  assert.equal(formatDuration(null), "—");
});

test("routeHeadline résume la route en une phrase", () => {
  const summary = { endInstant: "", durationMinutes: 1440, distanceNM: 694.6, maneuverCount: 3, finalStamina: 65 };
  assert.equal(routeHeadline(summary, "06Z du 22/11"), "1 j de route · 695 NM · 3 manœuvres · GFS 06Z du 22/11");
  assert.equal(routeHeadline({ ...summary, distanceNM: 12.34, maneuverCount: 1 }, ""), "1 j de route · 12,3 NM · 1 manœuvre");
});

test("staminaSeries et maneuverFractions placent la route dans le temps", () => {
  const rows = [row(10, 100), row(11, 83, "tack"), row(12, null), row(14, 65)];
  assert.deepEqual(staminaSeries(rows), [
    { fraction: 0, value: 100 },
    { fraction: 0.25, value: 83 },
    { fraction: 1, value: 65 },
  ]);
  assert.deepEqual(maneuverFractions(rows), [0.25]);
  assert.deepEqual(staminaSeries([row(10, 100)]), []);
});

test("staminaPath garde la chute visible sous la plus basse valeur", () => {
  const { path, floor } = staminaPath([{ fraction: 0, value: 100 }, { fraction: 1, value: 65 }], 100, 50);
  assert.equal(floor, 60);
  assert.equal(path, "M0.0 0.0 L100.0 43.8");
  assert.deepEqual(staminaPath([], 100, 50), { path: "", floor: 0 });
});

test("groupOrdersByDay regroupe les ordres par jour local", () => {
  const orders = [{ instant: local(22, 13) }, { instant: local(22, 23) }, { instant: local(23, 1) }];
  const days = groupOrdersByDay(orders);
  assert.deepEqual(days.map((day) => [day.label, day.orders.length]), [["Dimanche 22/11", 2], ["Lundi 23/11", 1]]);
});

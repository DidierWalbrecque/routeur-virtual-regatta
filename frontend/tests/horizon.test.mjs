import assert from "node:assert/strict";
import test from "node:test";

import {
  HORIZON_OPTIONS,
  MAX_HORIZON_MINUTES,
  forecastHoursFor,
  formatDuration,
  horizonMinutes,
  isHorizonChoice,
} from "../src/lib/horizon.ts";

const start = Date.UTC(2026, 10, 1, 16, 50);
const hours = (count) => start + count * 3600000;

test("les options proposées sont celles de la liste", () => {
  assert.deepEqual(
    HORIZON_OPTIONS.map((option) => option.label),
    ["Jusqu'à l'arrivée", "4 heures", "24 heures", "2 jours", "3 jours", "5 jours"],
  );
  assert.ok(isHorizonChoice("5d"));
  assert.ok(!isHorizonChoice("7d"));
});

test("un horizon fixe est conservé quand les prévisions le couvrent", () => {
  const result = horizonMinutes("2d", start, hours(100));
  assert.deepEqual(result, { ok: true, minutes: 2880, limited: false, requestedMinutes: 2880 });
});

test("un horizon plus long que les prévisions est raccourci et signalé", () => {
  const result = horizonMinutes("5d", start, hours(67));
  assert.equal(result.ok, true);
  assert.equal(result.minutes, 67 * 60);
  assert.equal(result.limited, true);
  assert.equal(result.requestedMinutes, 7200);
});

test("jusqu'à l'arrivée prend toute la couverture, plafonnée à 5 jours", () => {
  const partial = horizonMinutes("arrival", start, hours(67));
  assert.equal(partial.minutes, 67 * 60);
  assert.equal(partial.limited, false);
  const long = horizonMinutes("arrival", start, hours(300));
  assert.equal(long.minutes, MAX_HORIZON_MINUTES);
  assert.equal(long.limited, false);
});

test("un départ hors des prévisions est refusé", () => {
  assert.equal(horizonMinutes("24h", start, start).ok, false);
  assert.equal(horizonMinutes("24h", start, start - 3600000).ok, false);
  assert.equal(horizonMinutes("24h", start, Number.NaN).ok, false);
});

test("les prévisions à télécharger couvrent l'horizon", () => {
  // 3 jours après un départ jusqu'à 12 h après l'émission du run : 84 h.
  assert.equal(forecastHoursFor("3d").at(-1), 84);
  assert.equal(forecastHoursFor("5d").at(-1), 132);
  assert.equal(forecastHoursFor("5d").length, 45);
  for (const choice of ["arrival", "4h", "24h", "2d"]) {
    assert.equal(forecastHoursFor(choice).at(-1), 72, choice);
  }
  assert.deepEqual(forecastHoursFor("4h").slice(0, 3), [0, 3, 6]);
});

test("formatDuration écrit des jours et des heures", () => {
  assert.equal(formatDuration(240), "4 h");
  assert.equal(formatDuration(2880), "2 j");
  assert.equal(formatDuration(4020), "2 j 19 h");
});

import { MAX_MINUTES_BY_MODE, horizonAllowedIn, horizonOptionsFor } from "../src/lib/horizon.ts";

test("le mode précis s'arrête à 3 jours, le rapide va jusqu'à 5", () => {
  assert.deepEqual(
    horizonOptionsFor("precise").map((option) => option.value),
    ["arrival", "4h", "24h", "2d", "3d"],
  );
  assert.deepEqual(
    horizonOptionsFor("quick").map((option) => option.value),
    ["arrival", "4h", "24h", "2d", "3d", "5d"],
  );
  assert.equal(horizonAllowedIn("5d", "precise"), "3d");
  assert.equal(horizonAllowedIn("5d", "quick"), "5d");
  assert.equal(horizonAllowedIn("arrival", "precise"), "arrival");
});

test("jusqu'à l'arrivée est plafonné selon le mode", () => {
  const precise = horizonMinutes("arrival", start, hours(300), MAX_MINUTES_BY_MODE.precise);
  assert.equal(precise.minutes, 3 * 1440);
  assert.equal(precise.limited, false);
  const quick = horizonMinutes("arrival", start, hours(300), MAX_MINUTES_BY_MODE.quick);
  assert.equal(quick.minutes, 5 * 1440);
  const fixedTooLong = horizonMinutes("5d", start, hours(300), MAX_MINUTES_BY_MODE.precise);
  assert.equal(fixedTooLong.minutes, 3 * 1440);
  assert.equal(fixedTooLong.limited, true);
});

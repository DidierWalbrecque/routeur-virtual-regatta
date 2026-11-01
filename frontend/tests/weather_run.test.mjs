import assert from "node:assert/strict";
import test from "node:test";

import { buildWeatherRunModel } from "../src/lib/weather_run.ts";

const files = [
  "../data/weather/gfs.2026110106.f000.grb2",
  "../data/weather/gfs.2026110106.f036.grb2",
  "../data/weather/gfs.2026110106.f072.grb2",
];

test("buildWeatherRunModel décrit le run, l'horizon et l'âge", () => {
  const now = Date.UTC(2026, 10, 1, 18, 30);
  const model = buildWeatherRunModel("2026110106", files, now);
  assert.equal(model.title, "GFS 0.25");
  assert.equal(model.runText, "01/11/2026 06Z");
  assert.equal(model.horizonHours, 72);
  assert.equal(model.ageHours, 12);
  assert.equal(model.elapsedPercent, 17);
  assert.equal(model.coveredUntil, "2026-11-04T06:00:00.000Z");
  assert.equal(model.fileCount, 3);
});

test("buildWeatherRunModel borne la part écoulée à l'horizon", () => {
  const model = buildWeatherRunModel("2026110106", files, Date.UTC(2026, 10, 10));
  assert.equal(model.elapsedPercent, 100);
});

test("buildWeatherRunModel refuse un run invalide", () => {
  assert.equal(buildWeatherRunModel("n'importe quoi", files), null);
  assert.equal(buildWeatherRunModel("", files), null);
});

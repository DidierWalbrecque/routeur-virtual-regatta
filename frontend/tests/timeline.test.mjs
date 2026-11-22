import assert from "node:assert/strict";
import test from "node:test";

import {
  activeAt,
  legAt,
  floorHour,
  formatOffset,
  fractionOf,
  freeRange,
  interpolateLongitude,
  isochroneIndexAt,
  maneuverMarks,
  positionAt,
  routeRange,
  stepIndexAt,
  windBands,
  windGradient,
} from "../src/lib/timeline.ts";

const at = (minutes) => new Date(Date.UTC(2026, 10, 22, 12, minutes)).toISOString();
// Comme dans le tableau de route : le vent, la voile et la manœuvre d'un pas
// sont portés par le point où il se termine.
const route = [
  { position: { latitude: 16, longitude: -61 }, instant: at(0) },
  { position: { latitude: 16.2, longitude: -61.2 }, instant: at(10), tws: 12, twa: 140, sail: "Code 0" },
  { position: { latitude: 16.4, longitude: -61.4 }, instant: at(20), tws: 12, twa: 150, maneuver: "gybe" },
  { position: { latitude: 16.6, longitude: -61.6 }, instant: at(30), tws: 25, twa: 150, terminal: true },
];

test("routeRange va du premier au dernier point de la route", () => {
  const range = routeRange(route);
  assert.equal(range.start, Date.parse(at(0)));
  assert.equal(range.end, Date.parse(at(30)));
  assert.equal(routeRange([route[0]]), null);
  assert.equal(routeRange([]), null);
});

test("freeRange part de l'heure en cours et couvre 240 h", () => {
  const now = Date.parse("2026-11-22T11:31:48Z");
  const range = freeRange(now);
  assert.equal(range.start, Date.parse("2026-11-22T11:00:00Z"));
  assert.equal(range.end - range.start, 240 * 3_600_000);
  assert.equal(floorHour(now), range.start);
});

test("stepIndexAt donne le dernier point atteint", () => {
  assert.equal(stepIndexAt(route, Date.parse(at(0)) - 1), 0);
  assert.equal(stepIndexAt(route, Date.parse(at(10))), 1);
  assert.equal(stepIndexAt(route, Date.parse(at(19))), 1);
  assert.equal(stepIndexAt(route, Date.parse(at(90))), 3);
  assert.equal(stepIndexAt([], 0), -1);
});

test("positionAt interpole entre deux points de route", () => {
  const middle = positionAt(route, Date.parse(at(5)));
  assert.ok(Math.abs(middle.latitude - 16.1) < 1e-9);
  assert.ok(Math.abs(middle.longitude + 61.1) < 1e-9);
  assert.deepEqual(positionAt(route, Date.parse(at(60))), route[3].position);
  assert.equal(positionAt([], 0), null);
});

test("l'interpolation passe l'antiméridien par le plus court chemin", () => {
  assert.ok(Math.abs(interpolateLongitude(179, -179, 0.5) - 180) < 1e-9 || Math.abs(interpolateLongitude(179, -179, 0.5) + 180) < 1e-9);
  assert.ok(Math.abs(interpolateLongitude(179, -179, 0.25) - 179.5) < 1e-9);
  assert.ok(Math.abs(interpolateLongitude(-179, 179, 0.25) + 179.5) < 1e-9);
});

test("legAt donne le pas parcouru depuis un point", () => {
  assert.equal(legAt(route, 0), route[1]);
  assert.equal(legAt(route, 2), route[3]);
  assert.equal(legAt(route, 3), route[3]);
  assert.equal(legAt([], 0), null);
});

test("maneuverMarks place les manœuvres au début du pas qui les fait", () => {
  const marks = maneuverMarks(route, routeRange(route));
  assert.equal(marks.length, 1);
  assert.equal(marks[0].index, 1);
  assert.ok(Math.abs(marks[0].fraction - 1 / 3) < 1e-9);
});

test("windBands regroupe les points de même force Beaufort", () => {
  const bands = windBands(route, routeRange(route));
  assert.equal(bands.length, 2);
  assert.equal(bands[0].force, 4);
  assert.ok(Math.abs(bands[0].to - 2 / 3) < 1e-9);
  assert.equal(bands[1].force, 6);
  assert.equal(bands[1].to, 1);
});

test("isochroneIndexAt et activeAt suivent le curseur", () => {
  const isochrones = [{ instant: at(10), contour: [] }, { instant: at(20), contour: [] }];
  assert.equal(isochroneIndexAt(isochrones, Date.parse(at(5))), -1);
  assert.equal(isochroneIndexAt(isochrones, Date.parse(at(15))), 0);
  assert.equal(isochroneIndexAt(isochrones, Date.parse(at(25))), 1);
  const orders = [{ index: 0 }, { index: 2 }];
  assert.equal(activeAt(orders, 1), orders[0]);
  assert.equal(activeAt(orders, 2), orders[1]);
  assert.equal(activeAt([], 3), null);
});

test("fractionOf et formatOffset", () => {
  const range = { start: 0, end: 100 };
  assert.equal(fractionOf(range, -10), 0);
  assert.equal(fractionOf(range, 50), 0.5);
  assert.equal(fractionOf(range, 200), 1);
  assert.equal(formatOffset(5), "+5 min");
  assert.equal(formatOffset(60), "+1 h");
  assert.equal(formatOffset(860), "+14 h 20");
  assert.equal(formatOffset(3060), "+2 j 3 h");
});

test("windGradient place la couleur de chaque tronçon en son milieu", () => {
  assert.equal(windGradient([]), "none");
  assert.equal(windGradient([{ from: 0, to: 1, force: 3, color: "#96f7b4" }]), "linear-gradient(#96f7b4, #96f7b4)");
  assert.equal(
    windGradient([
      { from: 0, to: 0.5, force: 3, color: "#96f7b4" },
      { from: 0.5, to: 1, force: 5, color: "#73ed12" },
    ]),
    "linear-gradient(to right, #96f7b4 25.00%, #73ed12 75.00%)",
  );
});

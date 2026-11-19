import assert from "node:assert/strict";
import test from "node:test";

import { buildRouteDetailsModel } from "../src/lib/route_details.ts";

test("buildRouteDetailsModel décrit une route complète", () => {
  const model = buildRouteDetailsModel({
    destinationReached: true,
    route: [
      { position: { latitude: 13, longitude: 121 }, instant: "2026-09-25T10:00:00Z" },
      { position: { latitude: 13.1, longitude: 121.1 }, heading: 45, sail: "Jib", instant: "2026-09-25T10:15:00Z" },
      { position: { latitude: 13.2, longitude: 121.2 }, instant: "2026-09-25T10:30:00Z", terminal: true },
    ],
  });

  assert.equal(model.destinationReached, true);
  assert.equal(model.statusText, "Destination atteinte.");
  assert.equal(model.pointCount, 3);
  assert.equal(model.durationMinutes, 30);
  assert.deepEqual(model.steps.map((step) => step.label), ["Départ", "Point 1", "Arrivée"]);
  assert.equal(model.steps[1].elapsedMinutes, 15);
  assert.equal(model.steps[1].heading, 45);
  assert.equal(model.steps[2].terminal, true);
});

test("buildRouteDetailsModel conserve une route partielle", () => {
  const model = buildRouteDetailsModel({
    destinationReached: false,
    route: [
      { position: { latitude: 13, longitude: 121 }, instant: "2026-09-25T10:00:00Z" },
      { position: { latitude: 13.1, longitude: 121.1 }, heading: 90, sail: "Code 0", instant: "2026-09-25T10:20:00Z" },
    ],
  });

  assert.equal(model.destinationReached, false);
  assert.match(model.statusText, /meilleure route disponible/);
  assert.equal(model.pointCount, 2);
  assert.equal(model.durationMinutes, 20);
  assert.equal(model.steps[1].label, "Point 1");
  assert.equal(model.steps[1].terminal, false);
});

import { formatDms, formatElapsed, formatPosition } from "../src/lib/route_details.ts";

test("buildRouteDetailsModel associe à chaque départ de pas le détail du pas suivant", () => {
  const model = buildRouteDetailsModel({
    destinationReached: true,
    route: [
      { position: { latitude: 49.07, longitude: -4.04 }, instant: "2026-11-01T16:36:00Z", stamina: 100 },
      {
        position: { latitude: 49.08, longitude: -4.15 }, instant: "2026-11-01T16:46:00Z", stamina: 90,
        heading: 271, sail: "Light Gennaker", twa: 134, twd: 44, tws: 14.1, tack: "starboard",
        speedKnots: 26.55, foilPercent: 62, speedCoefficient: 0.9, maneuver: "tack",
      },
      { position: { latitude: 49.1, longitude: -4.3 }, instant: "2026-11-01T16:56:00Z", terminal: true },
    ],
  });

  const first = model.rows[0];
  assert.equal(first.heading, 271);
  assert.equal(first.windAngle, 134);
  assert.equal(first.tack, "T");
  assert.equal(first.windDirection, 44);
  assert.equal(first.windKnots, 14.1);
  assert.equal(first.speedKnots, 26.55);
  assert.equal(first.foilPercent, 62);
  assert.equal(first.speedCoefficient, 0.9);
  assert.equal(first.sail, "Light Gennaker");
  assert.equal(first.maneuver, "tack");
  assert.equal(model.maneuverCount, 1);
  // Arrivée nue (backend ancien, cap absent) : pas de détail pour le dernier bord.
  assert.equal(model.rows[1].heading, null);
  assert.equal(model.rows[1].tack, "—");
  assert.equal(model.rows[2].terminal, true);
});

test("buildRouteDetailsModel affiche le dernier bord quand l'arrivée le décrit", () => {
  const route = [
    { position: { latitude: 16.43, longitude: -61.8 }, instant: "2026-11-12T19:23:00Z", stamina: 100 },
    {
      position: { latitude: 16.4311, longitude: -61.8004 }, instant: "2026-11-12T19:33:00Z", stamina: 100,
      heading: 190, sail: "Jib", twa: 60, twd: 130, tws: 9, tack: "port", speedKnots: 8,
    },
    {
      position: { latitude: 16.367, longitude: -61.78 }, instant: "2026-11-12T19:43:00Z", stamina: 100, terminal: true,
      heading: 196, sail: "Jib", twa: 66, twd: 130, tws: 9.2, tack: "port", speedKnots: 8.4,
    },
  ];
  const model = buildRouteDetailsModel({ destinationReached: true, route });
  assert.equal(model.rows[1].heading, 196);
  assert.equal(model.rows[1].windAngle, 66);
  assert.equal(model.rows[1].sail, "Jib");
  assert.equal(model.rows[1].speedKnots, 8.4);
  // Après l'arrivée, il n'y a plus de bord.
  assert.equal(model.rows[2].heading, null);
  // Une arrivée nue avec un cap à 0 ne remplit pas la ligne précédente.
  const bare = { ...route[2], heading: 0, twa: undefined, twd: undefined, tws: undefined, sail: "", speedKnots: undefined, tack: "" };
  assert.equal(buildRouteDetailsModel({ route: [route[0], route[1], bare] }).rows[1].heading, null);
});

test("formatPosition et formatElapsed reproduisent le format du jeu", () => {
  assert.equal(formatDms(49.0724222, "N", "S"), `49°04'20.72"N`);
  assert.equal(formatDms(-4.04624, "E", "W"), `4°02'46.46"W`);
  assert.equal(formatDms(10.99999999, "N", "S"), `11°00'00.00"N`);
  assert.equal(formatPosition(49.0724222, -4.04624), `49°04'20.72"N 4°02'46.46"W`);
  assert.equal(formatPosition(NaN, 1), "—");
  assert.equal(formatElapsed(0), "+0:00");
  assert.equal(formatElapsed(130), "+2:10");
  assert.equal(formatElapsed(null), "—");
});

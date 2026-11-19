import test from "node:test";
import assert from "node:assert/strict";
import { bestDepartureIndex, departureBetter, departureGapNM, departureSummary } from "../src/lib/departure_window.ts";

const outcome = (fields) => ({
  index: 0,
  departure: "2026-11-20T00:00:00Z",
  reached: false,
  elapsedHours: 72,
  gatesCrossed: 0,
  nextTarget: "Biscay",
  distanceNM: 0,
  position: { latitude: 0, longitude: 0 },
  ...fields,
});

test("le classement suit l'arrivée, puis les portes, puis la distance", () => {
  const arrived = outcome({ reached: true, elapsedHours: 60 });
  const twoGates = outcome({ gatesCrossed: 2, distanceNM: 900 });
  const oneGateNear = outcome({ gatesCrossed: 1, distanceNM: 10 });
  const oneGateFar = outcome({ gatesCrossed: 1, distanceNM: 400 });
  assert.equal(departureBetter(arrived, twoGates), true);
  assert.equal(departureBetter(twoGates, oneGateNear), true);
  assert.equal(departureBetter(oneGateNear, oneGateFar), true);
  assert.equal(departureBetter(oneGateFar, oneGateNear), false);
  assert.equal(bestDepartureIndex([oneGateFar, oneGateNear, twoGates]), 2);
  assert.equal(bestDepartureIndex([]), -1);
});

test("à égalité, le départ le plus tôt l'emporte", () => {
  const early = outcome({ gatesCrossed: 1, distanceNM: 100 });
  const late = outcome({ departure: "2026-11-20T06:00:00Z", gatesCrossed: 1, distanceNM: 100 });
  assert.equal(departureBetter(early, late), true);
  assert.equal(departureBetter(late, early), false);
});

test("le résumé d'un départ dit où en est la route", () => {
  assert.equal(departureSummary(outcome({ gatesCrossed: 2, distanceNM: 412.6, nextTarget: "Equator" }), 13), "2/13 portes · 413 NM de Equator");
  assert.equal(
    departureSummary(outcome({ gatesCrossed: 1, distanceNM: 235, nextTarget: "Checkpoint 2.8" }), 13, ["Biscay", "tribord"]),
    "1/13 portes · 235 NM de la porte 2 (tribord)",
  );
  assert.equal(departureSummary(outcome({ gatesCrossed: 13, distanceNM: 80, nextTarget: "arrivée" }), 13), "13/13 portes · 80 NM de l'arrivée");
  assert.equal(departureSummary(outcome({ reached: true, elapsedHours: 52 }), 13), "arrivée en 2 j 4 h");
});

test("l'écart au meilleur n'a de sens que vers la même porte", () => {
  const best = outcome({ gatesCrossed: 2, distanceNM: 100 });
  assert.equal(departureGapNM(outcome({ gatesCrossed: 2, distanceNM: 160 }), best), 60);
  assert.equal(departureGapNM(outcome({ gatesCrossed: 1, distanceNM: 10 }), best), null);
});

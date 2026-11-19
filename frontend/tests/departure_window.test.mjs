import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { bestDepartureIndex, departureBetter, departureGapNM, departureSummary, departureTarget } from "../src/lib/departure_window.ts";
import { checkpointName } from "../src/lib/vr.ts";

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

test("les portes sans nom de la capture du Jules Verne sont nommées par leur sens", () => {
  const leg = JSON.parse(readFileSync(new URL("../../data/data-record.json", import.meta.url), "utf8")).res.leg;
  const names = leg.checkpoints.map((checkpoint) => checkpointName(checkpoint));
  assert.equal(names[0], "Biscay");
  // Porte 2.8 : nom vide, display « none ».
  assert.equal(names[1], "tribord");
  assert.equal(
    departureSummary(outcome({ gatesCrossed: 1, distanceNM: 235, nextTarget: "Checkpoint 2.8" }), names.length, names),
    "1/13 portes · 235 NM de la porte 2 (tribord)",
  );
});

test("la porte visée s'affiche par son numéro et son nom", () => {
  const names = ["Biscay", "tribord"];
  assert.equal(departureTarget(outcome({ gatesCrossed: 1, nextTarget: "Checkpoint 2.8" }), 13, names), "porte 2 (tribord)");
  assert.equal(departureTarget(outcome({ gatesCrossed: 13, nextTarget: "arrivée" }), 13, names), "arrivée");
  assert.equal(departureTarget(outcome({ gatesCrossed: 5, nextTarget: "Cape Agulhas" }), 13, names), "Cape Agulhas");
});

test("les départs se répartissent par pages de 10", async () => {
  const { pageCount, pageOf, clampPage } = await import("../src/lib/departure_window.ts");
  assert.equal(pageCount(0), 1);
  assert.equal(pageCount(10), 1);
  assert.equal(pageCount(11), 2);
  assert.equal(pageCount(25), 3);
  assert.equal(pageOf(0), 0);
  assert.equal(pageOf(9), 0);
  assert.equal(pageOf(10), 1);
  assert.equal(pageOf(-1), 0);
  assert.equal(clampPage(5, 25), 2);
  assert.equal(clampPage(-1, 25), 0);
});

test("les barres de la fenêtre de départ suivent le retard sur le meilleur", async () => {
  const { departureBars } = await import("../src/lib/departure_window.ts");
  // Retards : 100, 0, 700 (isolé), 200, 300, 400 ; l'échelle s'arrête au
  // 80e centile, 400 NM, et le départ isolé reste au plancher.
  const bars = departureBars([
    outcome({ gatesCrossed: 1, distanceNM: 300 }),
    outcome({ gatesCrossed: 1, distanceNM: 200 }),
    outcome({ gatesCrossed: 1, distanceNM: 900 }),
    outcome({ gatesCrossed: 1, distanceNM: 400 }),
    outcome({ gatesCrossed: 1, distanceNM: 500 }),
    outcome({ gatesCrossed: 1, distanceNM: 600 }),
    outcome({ gatesCrossed: 0, distanceNM: 50 }),
  ]);
  assert.equal(bars[1].value, 1);
  assert.ok(Math.abs(bars[0].value - (1 - 0.85 * (100 / 400))) < 1e-9);
  assert.ok(Math.abs(bars[5].value - 0.15) < 1e-9);
  assert.ok(Math.abs(bars[2].value - 0.15) < 1e-9);
  assert.ok(bars[0].value > bars[3].value && bars[3].value > bars[4].value);
  assert.equal(bars[6].comparable, false);
  assert.deepEqual(departureBars([]), []);
  const arrived = departureBars([outcome({ reached: true, elapsedHours: 50 }), outcome({ reached: true, elapsedHours: 60 }), outcome({ gatesCrossed: 9 })]);
  assert.equal(arrived[0].value, 1);
  assert.ok(Math.abs(arrived[1].value - 0.15) < 1e-9);
  assert.equal(arrived[2].comparable, false);
});

test("le retard hors milles se dit en portes, ou en arrivée manquée", async () => {
  const { departureLagLabel } = await import("../src/lib/departure_window.ts");
  const best = outcome({ gatesCrossed: 3, distanceNM: 100 });
  assert.equal(departureLagLabel(outcome({ gatesCrossed: 2 }), best), "1 porte de retard");
  assert.equal(departureLagLabel(outcome({ gatesCrossed: 1 }), best), "2 portes de retard");
  assert.equal(departureLagLabel(outcome({ gatesCrossed: 3, distanceNM: 150 }), best), null);
  assert.equal(departureLagLabel(outcome({ gatesCrossed: 13 }), outcome({ reached: true, elapsedHours: 60 })), "n'arrive pas");
});

test("deux départs identiques sont tous deux meilleurs", async () => {
  const { bestDepartureIndices, departuresTied } = await import("../src/lib/departure_window.ts");
  const list = [
    outcome({ departure: "2026-11-20T00:00:00Z", gatesCrossed: 1, distanceNM: 300 }),
    outcome({ departure: "2026-11-20T12:00:00Z", gatesCrossed: 1, distanceNM: 235.2 }),
    outcome({ departure: "2026-11-21T00:00:00Z", gatesCrossed: 1, distanceNM: 234.8 }),
    outcome({ departure: "2026-11-21T12:00:00Z", gatesCrossed: 1, distanceNM: 236 }),
  ];
  assert.deepEqual(bestDepartureIndices(list), [1, 2]);
  assert.deepEqual(bestDepartureIndices([]), []);
  assert.equal(departuresTied(outcome({ gatesCrossed: 2, distanceNM: 100 }), outcome({ gatesCrossed: 1, distanceNM: 100 })), false);
  assert.equal(departuresTied(outcome({ reached: true, elapsedHours: 50 }), outcome({ reached: true, elapsedHours: 50.001 })), true);
});

test("la phrase des réglages nomme la polaire et les options utilisées", async () => {
  const { departureSettingsText } = await import("../src/lib/departure_window.ts");
  assert.equal(
    departureSettingsText({ polar: "Ultim BP 11", options: ["Light", "Heavy", "Foils"] }),
    "Calculé avec la polaire Ultim BP 11 et les options Light, Heavy et Foils.",
  );
  assert.equal(departureSettingsText({ polar: "Imoca 60 Foils", options: ["Code 0"] }), "Calculé avec la polaire Imoca 60 Foils et l'option Code 0.");
  assert.equal(departureSettingsText({ polar: "Class 40", options: [] }), "Calculé avec la polaire Class 40, sans option.");
});

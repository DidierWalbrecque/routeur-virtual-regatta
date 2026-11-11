import assert from "node:assert/strict";
import test from "node:test";

import { cutContourAtBreaks, forwardContour, formatVirtualRegattaPosition, parseLocalDate, parseVirtualRegattaPosition } from "../src/lib/geo.ts";
import { checkpointFeatures, checkpointLabel, equipmentFromVrOptions, meaningfulName, usableCheckpoints } from "../src/lib/vr.ts";

test("parseVirtualRegattaPosition et formatVirtualRegattaPosition sont réciproques", () => {
  const position = parseVirtualRegattaPosition(`47°26'11.95"N - 24°54'55.32"W`);
  assert.ok(position);
  assert.ok(Math.abs(position.latitude - (47 + 26 / 60 + 11.95 / 3600)) < 1e-9);
  assert.ok(Math.abs(position.longitude - -(24 + 54 / 60 + 55.32 / 3600)) < 1e-9);
  assert.equal(formatVirtualRegattaPosition(position), `47°26'11.95"N - 24°54'55.32"W`);
  // L'ancien format reste accepté en entrée (rétro-compat).
  const legacy = parseVirtualRegattaPosition(`13°N 07'16" - 121°E 35'12"`);
  assert.ok(legacy);
  assert.ok(Math.abs(legacy.latitude - (13 + 7 / 60 + 16 / 3600)) < 1e-9);
  const west = parseVirtualRegattaPosition(`49°04'21.00"N - 4°02'46.00"W`);
  assert.ok(west && west.longitude < 0);
  assert.equal(formatVirtualRegattaPosition(west), `49°04'21.00"N - 4°02'46.00"W`);
  assert.equal(parseVirtualRegattaPosition("n'importe quoi"), null);
});

test("parseLocalDate refuse un format invalide", () => {
  assert.throws(() => parseLocalDate("2026-11-01"), /JJ\/MM\/AAAA/);
  assert.match(parseLocalDate("01/11/2026 12:30"), /^2026-11-0[12]T/);
});

test("equipmentFromVrOptions reconnaît les alias du jeu", () => {
  const owned = equipmentFromVrOptions(["Foil", "reach", "hull_polish", "WINCHES"]);
  assert.deepEqual([...owned].sort(), ["code0", "foils", "hull_polish", "pro_winches"]);
  assert.equal(equipmentFromVrOptions(undefined).size, 0);
});

test("les portes passées ou à terre ne sont pas utilisables", () => {
  const point = { latitude: 1, longitude: 2 };
  const state = {
    gateGroupCounters: [true, false, false],
    checkpoints: [
      { group: 1, id: 1, start: point, end: point },
      { group: 2, id: 1, start: point, end: point },
      { group: 3, id: 1, start: point, end: point, onLand: true },
    ],
  };
  assert.deepEqual(usableCheckpoints(state).map((checkpoint) => checkpoint.group), [2]);
  const features = checkpointFeatures(state);
  // Une seule porte utilisable : un point, sur sa marque de départ.
  assert.equal(features.length, 1);
  assert.equal(features[0].geometry.type, "Point");
});

test("une porte nommée « none » par le jeu affiche son sens de passage", () => {
  const point = { latitude: 1, longitude: 2 };
  assert.equal(checkpointLabel({ group: 4, id: 4, name: "none", side: "port", start: point, end: point }, 0), "P4.4 — bâbord");
  assert.equal(checkpointLabel({ group: 4, id: 5, name: " None ", display: "", start: point, end: point }, 1), "P4.5 — sens libre");
  assert.equal(checkpointLabel({ group: 1, id: 1, name: "none", display: "Bouée de Marie-Galante", start: point, end: point }, 0), "P1.1 — Bouée de Marie-Galante");
  assert.equal(checkpointLabel({ group: 2, id: 1, name: "Cap Horn", rhumb: true, start: point, end: point }, 0), "P2.1 — Cap Horn");

  const [label] = checkpointFeatures({ checkpoints: [{ group: 4, id: 4, name: "none", side: "stbd", start: point, end: point }] });
  assert.equal(label.properties.name, "tribord");
  assert.equal(label.properties.label, "P4.4 — tribord");

  assert.equal(meaningfulName("none"), null);
  assert.equal(meaningfulName("buoy"), null);
  assert.equal(checkpointLabel({ group: 2, id: 2, name: "buoy", side: "stbd", start: point, end: point }, 0), "P2.2 — tribord");
  assert.equal(meaningfulName(undefined), null);
  assert.equal(meaningfulName(" Pointe-à-Pitre "), "Pointe-à-Pitre");
});

test("forwardContour garde l'arc tourné vers la destination", () => {
  const ring = [
    { latitude: 0, longitude: 0 },
    { latitude: 1, longitude: 1 },
    { latitude: 0, longitude: 2 },
    { latitude: -1, longitude: 1 },
  ];
  assert.deepEqual(forwardContour(ring, { x: 0, y: 0 }), []);
  assert.ok(forwardContour(ring, { x: 1, y: 0 }).length >= 2);
});

test("cutContourAtBreaks ne coupe que les côtés signalés, même longs", () => {
  const ring = [
    { latitude: 0, longitude: 0 },
    { latitude: 0, longitude: 0.01 },
    // Côté de 30 milles : au près, c'est un vrai bord du front.
    { latitude: 0, longitude: 0.51 },
    { latitude: 0, longitude: 0.52 },
    { latitude: 0, longitude: 0.53 },
  ];
  assert.deepEqual(cutContourAtBreaks(ring, ring, []).map((part) => part.length), [5]);
  // Le côté 2 -> 3 traverse une île : il est coupé, dans un sens comme dans l'autre.
  assert.deepEqual(cutContourAtBreaks(ring, ring, [2]).map((part) => part.length), [3, 2]);
  assert.deepEqual(cutContourAtBreaks([...ring].reverse(), ring, [2]).map((part) => part.length), [2, 3]);
});

test("forwardContour garde les flancs de l'enveloppe quand l'angle s'élargit", () => {
  // Carré orienté nord-sud ; la destination est plein sud.
  const square = [
    { latitude: 0, longitude: 0 },
    { latitude: 0, longitude: 1 },
    { latitude: 1, longitude: 1 },
    { latitude: 1, longitude: 0 },
    { latitude: 0, longitude: 0 },
  ];
  const south = { x: 0, y: -1 };
  // À 90°, seul le côté sud reste : les flancs est et ouest disparaissent.
  assert.equal(forwardContour(square, south).length, 2);
  // À 110°, les flancs, perpendiculaires à la direction, sont gardés.
  assert.equal(forwardContour(square, south, 110).length, 4);
  // Le côté nord, tourné à l'opposé, n'est jamais tracé.
  assert.ok(forwardContour(square, south, 179).length < 5);
});

import { EQUIPMENT, EQUIPMENT_LABELS } from "../src/lib/vr.ts";

test("chaque équipement a un libellé lisible", () => {
  for (const item of EQUIPMENT) {
    assert.ok(EQUIPMENT_LABELS[item], `libellé manquant pour ${item}`);
    assert.ok(!EQUIPMENT_LABELS[item].includes("_"), `libellé technique pour ${item}`);
  }
});

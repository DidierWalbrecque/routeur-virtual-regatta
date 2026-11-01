import assert from "node:assert/strict";
import test from "node:test";

import { forwardContour, formatVirtualRegattaPosition, parseLocalDate, parseVirtualRegattaPosition } from "../src/lib/geo.ts";
import { checkpointFeatures, equipmentFromVrOptions, usableCheckpoints } from "../src/lib/vr.ts";

test("parseVirtualRegattaPosition et formatVirtualRegattaPosition sont réciproques", () => {
  const position = parseVirtualRegattaPosition(`13°N 07'16" - 121°E 35'12"`);
  assert.ok(position);
  assert.ok(Math.abs(position.latitude - (13 + 7 / 60 + 16 / 3600)) < 1e-9);
  assert.equal(formatVirtualRegattaPosition(position), `13°N 07'16" - 121°E 35'12"`);
  const west = parseVirtualRegattaPosition(`49°N 04'21" - 004°W 02'46"`);
  assert.ok(west && west.longitude < 0);
  assert.equal(formatVirtualRegattaPosition(west), `49°N 04'21" - 004°W 02'46"`);
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
  assert.equal(features.length, 4);
  assert.equal(features[0].geometry.type, "LineString");
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

import { EQUIPMENT, EQUIPMENT_LABELS } from "../src/lib/vr.ts";

test("chaque équipement a un libellé lisible", () => {
  for (const item of EQUIPMENT) {
    assert.ok(EQUIPMENT_LABELS[item], `libellé manquant pour ${item}`);
    assert.ok(!EQUIPMENT_LABELS[item].includes("_"), `libellé technique pour ${item}`);
  }
});

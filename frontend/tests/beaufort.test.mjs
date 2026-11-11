import assert from "node:assert/strict";
import test from "node:test";

import { BEAUFORT, beaufortForce, windFieldPixels } from "../src/lib/beaufort.ts";

test("beaufortForce suit les bornes de l'échelle", () => {
  assert.equal(beaufortForce(0), 0);
  assert.equal(beaufortForce(0.9), 0);
  assert.equal(beaufortForce(1), 1);
  assert.equal(beaufortForce(10.9), 3);
  assert.equal(beaufortForce(11), 4);
  assert.equal(beaufortForce(33.9), 7);
  assert.equal(beaufortForce(64), 12);
  assert.equal(beaufortForce(120), 12);
});

test("windFieldPixels colore chaque pixel selon la force interpolée", () => {
  // Deux colonnes : 0 nœud à l'ouest, 20 nœuds à l'est ; un point inconnu au sud-est.
  const field = { time: "", south: 0, north: 1, west: 0, east: 1, cols: 2, rows: 2, speeds: [0, 20, 0, -1] };
  const pixels = windFieldPixels(field, 3, 2, 200);
  const rgba = (x, y) => [...pixels.slice((y * 3 + x) * 4, (y * 3 + x) * 4 + 4)];
  const hex = (color) => [1, 3, 5].map((index) => parseInt(color.slice(index, index + 2), 16));
  // Ligne nord : 0 nœud (F0), 10 nœuds au milieu (F3), 20 nœuds (F5).
  assert.deepEqual(rgba(0, 0), [...hex(BEAUFORT[0].color), 200]);
  assert.deepEqual(rgba(1, 0), [...hex(BEAUFORT[3].color), 200]);
  assert.deepEqual(rgba(2, 0), [...hex(BEAUFORT[5].color), 200]);
  // Ligne sud : le point inconnu est ignoré ; sur lui, le pixel reste transparent.
  assert.deepEqual(rgba(1, 1), [...hex(BEAUFORT[0].color), 200]);
  assert.equal(rgba(2, 1)[3], 0);
});

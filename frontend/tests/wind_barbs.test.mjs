import assert from "node:assert/strict";
import test from "node:test";

import { barbFeathers, barbPoints, barbShape } from "../src/lib/wind_barbs.ts";

test("barbFeathers arrondit aux 5 kn et décompose en fanions, traits et demi-trait", () => {
  assert.deepEqual(barbFeathers(1.2), { knots: 0, pennants: 0, full: 0, half: 0 });
  assert.deepEqual(barbFeathers(4), { knots: 5, pennants: 0, full: 0, half: 1 });
  assert.deepEqual(barbFeathers(11), { knots: 10, pennants: 0, full: 1, half: 0 });
  assert.deepEqual(barbFeathers(23.5), { knots: 25, pennants: 0, full: 2, half: 1 });
  assert.deepEqual(barbFeathers(66), { knots: 65, pennants: 1, full: 1, half: 1 });
});

test("barbShape : cercle au calme, traits du côté de l'hémisphère", () => {
  assert.equal(barbShape(1, 32, 32, 26, true).calm, true);
  const north = barbShape(15, 32, 32, 26, false);
  const south = barbShape(15, 32, 32, 26, true);
  // Hampe, un trait et un demi-trait.
  assert.equal(north.lines.length, 3);
  assert.deepEqual(north.lines[0], [32, 32, 32, 6]);
  assert.ok(north.lines[1][2] > 32, "hémisphère nord : traits à droite de la hampe qui monte");
  assert.ok(south.lines[1][2] < 32, "hémisphère sud : traits à gauche");
  assert.equal(barbShape(55, 32, 32, 26, true).pennants.length, 1);
});

test("barbPoints prend un point sur deux en quinconce et ignore les directions inconnues", () => {
  const cols = 8;
  const rows = 4;
  const speeds = Array.from({ length: cols * rows }, () => 12);
  const directions = Array.from({ length: cols * rows }, () => 270);
  directions[1 * cols + 1] = -1;
  const field = { time: "", south: -10, north: 10, west: 0, east: 70, cols, rows, speeds, directions };
  const points = barbPoints(field, 2);
  // Lignes 1 et 3 ; colonnes 1 et 5 puis 3 et 7 ; le point (1, 1) est inconnu.
  assert.equal(points.length, 3);
  assert.deepEqual(points.map((point) => point.longitude), [50, 30, 70]);
  assert.equal(points[0].direction, 270);
  assert.deepEqual(barbPoints({ ...field, directions: undefined }, 2), []);
});

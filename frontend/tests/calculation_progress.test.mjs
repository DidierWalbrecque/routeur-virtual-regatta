import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import { formatClock, progressFraction, remainingEstimate } from "../src/lib/calculation_progress.ts";

test("progressFraction reste entre 0 et 1, même sans total connu", () => {
  assert.equal(progressFraction(null), 0);
  assert.equal(progressFraction({ completed: 5, total: 0 }), 0);
  assert.equal(progressFraction({ completed: 33, total: 432 }), 33 / 432);
  assert.equal(progressFraction({ completed: 500, total: 432 }), 1);
});

test("formatClock affiche m:ss puis h:mm:ss", () => {
  assert.equal(formatClock(0), "0:00");
  assert.equal(formatClock(84_900), "1:24");
  assert.equal(formatClock(3_725_000), "1:02:05");
});

test("remainingEstimate attend quelques fronts avant d'estimer", () => {
  assert.equal(remainingEstimate(null, 60_000), null);
  assert.equal(remainingEstimate({ completed: 2, total: 100 }, 60_000), null);
  assert.equal(remainingEstimate({ completed: 10, total: 100 }, 3_000), null);
  assert.equal(remainingEstimate({ completed: 100, total: 100 }, 60_000), null);
  // 33 fronts en 33 s : 399 fronts restants, soit environ 7 min.
  assert.equal(remainingEstimate({ completed: 33, total: 432 }, 33_000), "environ 7 min");
  assert.equal(remainingEstimate({ completed: 90, total: 100 }, 90_000), "moins d'une minute");
  assert.equal(remainingEstimate({ completed: 10, total: 100 }, 900_000), "environ 2 h 15");
});

test("l'annulation du routage n'est pas présentée comme une erreur", () => {
  const source = readFileSync(new URL("../src/App.tsx", import.meta.url), "utf8");
  assert.match(source, /cancelledController\.current = controller;\s*controller\.abort\(\);/);
  assert.ok(source.includes("Routage annulé."));
});

import assert from "node:assert/strict";
import test from "node:test";

import { approximateDistanceNM, circlePolygon } from "../src/lib/geo.ts";
import { checkpointFeatures } from "../src/lib/vr.ts";

test("circlePolygon trace un cercle fermé au rayon demandé", () => {
  const center = { latitude: 16.2, longitude: -61.5 };
  const ring = circlePolygon(center, 1.5);
  assert.equal(ring.length, 65);
  assert.deepEqual(ring[0], ring.at(-1));
  for (const point of ring) {
    assert.ok(Math.abs(approximateDistanceNM(center, point) - 1.5) < 0.01);
  }
});

test("checkpointFeatures numérote les portes restantes dans l'ordre de passage", () => {
  const gate = (group, side) => ({
    group,
    id: 1,
    name: `Porte ${group}`,
    side,
    start: { latitude: 16, longitude: -61 - group / 10 },
    end: { latitude: 16.1, longitude: -61 - group / 10 },
  });
  const features = checkpointFeatures({
    checkpoints: [gate(1, "port"), gate(2, "stbd"), gate(3, "port")],
    // La première porte est déjà passée : la suivante devient la porte 1.
    gateGroupCounters: [true, false, false],
  });
  const badges = features.filter((feature) => feature.properties.kind === "label");
  assert.deepEqual(
    badges.map((feature) => [feature.properties.checkpoint, feature.properties.order]),
    [
      ["2.1", 1],
      ["3.1", 2],
    ],
  );
  // Les extrémités n'ont plus de lettre D/A, réservées aux épingles du routage.
  const marks = features.filter((feature) => feature.properties.kind === "mark");
  assert.equal(marks.length, 4);
  assert.ok(marks.every((feature) => !("markLabel" in feature.properties)));
});

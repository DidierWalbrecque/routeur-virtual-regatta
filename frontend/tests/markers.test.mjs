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
  assert.deepEqual(
    features.map((feature) => [feature.properties.checkpoint, feature.properties.order]),
    [
      ["2.1", 1],
      ["3.1", 2],
    ],
  );
  // Chaque porte est placée sur sa marque de départ ; la seconde extrémité n'est plus dessinée.
  assert.deepEqual(features[0].geometry, { type: "Point", coordinates: [-61.2, 16] });
});

test("gateRounding traduit le sens de passage en sens de contournement", async () => {
  const { gateRounding, gateRingSvg } = await import("../src/lib/gate_marker.ts");
  assert.equal(gateRounding("stbd", false), "clockwise");
  assert.equal(gateRounding("port", false), "counterclockwise");
  assert.equal(gateRounding("stbd", true), "free");
  assert.equal(gateRounding("", false), "free");
  // Le sens antihoraire est le miroir du sens horaire ; l'anneau libre est pointillé.
  assert.doesNotMatch(gateRingSvg("clockwise"), /scale\(-1 1\)/);
  assert.match(gateRingSvg("counterclockwise"), /scale\(-1 1\)/);
  assert.match(gateRingSvg("free"), /stroke-dasharray/);
});

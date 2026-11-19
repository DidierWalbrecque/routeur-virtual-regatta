import assert from "node:assert/strict";
import test from "node:test";

import { approximateDistanceNM, circlePolygon } from "../src/lib/geo.ts";
import { checkpointFeatures, gateMarks } from "../src/lib/vr.ts";

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

test("gateMarks place un repère sur la marque de départ et sur la marque d'arrivée", () => {
  const marks = gateMarks({
    checkpoints: [
      { group: 1, id: 7, name: "Biscay", side: "port", start: { latitude: 43, longitude: -9 }, end: { latitude: 42, longitude: -71 } },
      { group: 2, id: 1, name: "Bouée", side: "stbd", start: { latitude: 10, longitude: 20 }, end: { latitude: 10, longitude: 20 } },
      { group: 3, id: 2, name: "Agulhas", side: "port", start: { latitude: -34.8, longitude: 20 }, end: { latitude: -70.9, longitude: 20 } },
    ],
    // La porte 1 est passée : Agulhas devient la deuxième porte à passer.
    gateGroupCounters: [true, false, false],
  });
  assert.deepEqual(
    marks.map((mark) => [mark.checkpoint, mark.order, mark.end, mark.position.latitude]),
    [
      // Marque d'arrivée confondue avec le départ : un seul repère.
      ["2.1", 1, false, 10],
      ["3.2", 2, false, -34.8],
      ["3.2", 2, true, -70.9],
    ],
  );
});

test("la marque d'arrivée d'une porte se contourne dans le sens inverse", async () => {
  const { endMarkRounding } = await import("../src/lib/gate_marker.ts");
  assert.equal(endMarkRounding("clockwise"), "counterclockwise");
  assert.equal(endMarkRounding("counterclockwise"), "clockwise");
  assert.equal(endMarkRounding("free"), "free");
});

test("les portes nommées « none » ne sont pas dessinées mais gardent la numérotation", () => {
  const point = (latitude) => ({ latitude, longitude: 145 });
  const checkpoints = [
    { group: 1, id: 1, name: "gate", side: "stbd", start: point(-39), end: point(-40) },
    { group: 2, id: 2, name: "none", side: "port", start: point(-38), end: point(-37) },
    { group: 2, id: 3, name: "buoy", side: "port", start: point(-36), end: point(-36) },
  ];
  assert.deepEqual(
    checkpointFeatures({ checkpoints }).map((feature) => [feature.properties.checkpoint, feature.properties.order]),
    [["1.1", 1], ["2.3", 3]],
  );
  assert.deepEqual(
    gateMarks({ checkpoints }).map((mark) => [mark.checkpoint, mark.order, mark.end]),
    [["1.1", 1, false], ["1.1", 1, true], ["2.3", 3, false]],
  );
});

import assert from "node:assert/strict";
import test from "node:test";

import {
  createZone,
  draftFeatureCollection,
  nextZoneName,
  parseStoredZones,
  zonesFeatureCollection,
  zonesForRequest,
} from "../src/lib/zones.ts";

const square = [
  { latitude: 0, longitude: 0 },
  { latitude: 0, longitude: 1 },
  { latitude: 1, longitude: 1 },
  { latitude: 1, longitude: 0 },
];

test("createZone refuse un tracé de moins de trois points", () => {
  assert.equal(createZone([], square.slice(0, 2), "a"), null);
  const zone = createZone([], square, "a");
  assert.equal(zone?.name, "Zone 1");
  assert.equal(zone?.enabled, true);
  assert.notEqual(zone?.points[0], square[0]);
});

test("nextZoneName évite les noms déjà pris", () => {
  const zones = [{ id: "1", name: "Zone 2", enabled: true, points: square }];
  assert.equal(nextZoneName(zones), "Zone 3");
  assert.equal(nextZoneName([]), "Zone 1");
});

test("zonesForRequest n'envoie que les zones actives", () => {
  const zones = [
    { id: "1", name: "Île", enabled: true, points: square },
    { id: "2", name: "Désactivée", enabled: false, points: square },
  ];
  assert.deepEqual(zonesForRequest(zones), [{ name: "Île", points: square }]);
});

test("parseStoredZones ignore les données invalides", () => {
  assert.deepEqual(parseStoredZones(null), []);
  assert.deepEqual(parseStoredZones("pas du json"), []);
  assert.deepEqual(parseStoredZones('{"a":1}'), []);
  const raw = JSON.stringify([
    { id: "ok", name: "Bonne", enabled: false, points: square },
    { id: "court", name: "Courte", points: square.slice(0, 2) },
    { name: "Sans id", points: square },
    { id: "hors", name: "Hors limites", points: [...square.slice(0, 2), { latitude: 120, longitude: 0 }] },
  ]);
  const zones = parseStoredZones(raw);
  assert.equal(zones.length, 1);
  assert.equal(zones[0].id, "ok");
  assert.equal(zones[0].enabled, false);
});

test("les polygones affichés sont fermés", () => {
  const collection = zonesFeatureCollection([{ id: "1", name: "Z", enabled: true, points: square }]);
  const ring = collection.features[0].geometry.coordinates[0];
  assert.equal(ring.length, 5);
  assert.deepEqual(ring[0], ring[4]);
});

test("le tracé en cours se referme à partir de trois points", () => {
  assert.equal(draftFeatureCollection([]).features.length, 0);
  assert.equal(draftFeatureCollection(square.slice(0, 1)).features.length, 1);
  const two = draftFeatureCollection(square.slice(0, 2));
  assert.equal(two.features.filter((feature) => feature.geometry.type === "LineString").length, 1);
  const three = draftFeatureCollection(square.slice(0, 3));
  const line = three.features.find((feature) => feature.geometry.type === "LineString");
  assert.equal(line.geometry.coordinates.length, 4);
  assert.equal(three.features.filter((feature) => feature.properties.first).length, 1);
});

import { gameZoneId, gameZonesFrom, parseDisabledIds } from "../src/lib/zones.ts";

test("gameZonesFrom donne des identifiants stables et respecte les zones décochées", () => {
  const state = {
    zones: [
      { name: "Cornouailles", points: square },
      { name: "Cornouailles", points: square },
      { name: "", points: square },
      { name: "Trop courte", points: square.slice(0, 2) },
    ],
  };
  const first = gameZonesFrom(state, new Set());
  assert.equal(first.length, 3);
  assert.ok(first.every((zone) => zone.source === "game" && zone.enabled));
  assert.equal(new Set(first.map((zone) => zone.id)).size, 3);
  assert.equal(first[2].name, "Zone de la course 3");
  assert.equal(first[0].id, gameZoneId({ name: "Cornouailles", points: square }));

  const disabled = gameZonesFrom(state, new Set([first[0].id]));
  assert.equal(disabled[0].enabled, false);
  assert.equal(disabled[1].enabled, true);
  assert.deepEqual(gameZonesFrom(undefined, new Set()), []);
});

test("parseDisabledIds tolère un stockage invalide", () => {
  assert.deepEqual([...parseDisabledIds('["a","b",3]')], ["a", "b"]);
  assert.equal(parseDisabledIds("pas du json").size, 0);
  assert.equal(parseDisabledIds(null).size, 0);
});

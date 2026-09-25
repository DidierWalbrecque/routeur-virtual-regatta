import assert from "node:assert/strict";
import test from "node:test";

import { buildRouteDetailsModel } from "../static/route_details.js";

test("buildRouteDetailsModel décrit une route complète", () => {
  const model = buildRouteDetailsModel({
    destinationReached: true,
    route: [
      { position: { latitude: 13, longitude: 121 }, instant: "2026-09-25T10:00:00Z" },
      { position: { latitude: 13.1, longitude: 121.1 }, heading: 45, sail: "Jib", instant: "2026-09-25T10:15:00Z" },
      { position: { latitude: 13.2, longitude: 121.2 }, instant: "2026-09-25T10:30:00Z", terminal: true },
    ],
  });

  assert.equal(model.destinationReached, true);
  assert.equal(model.statusText, "Destination atteinte.");
  assert.equal(model.pointCount, 3);
  assert.equal(model.durationMinutes, 30);
  assert.deepEqual(model.steps.map((step) => step.label), ["Départ", "Point 1", "Arrivée"]);
  assert.equal(model.steps[1].elapsedMinutes, 15);
  assert.equal(model.steps[1].heading, 45);
  assert.equal(model.steps[2].terminal, true);
});

test("buildRouteDetailsModel conserve une route partielle", () => {
  const model = buildRouteDetailsModel({
    destinationReached: false,
    route: [
      { position: { latitude: 13, longitude: 121 }, instant: "2026-09-25T10:00:00Z" },
      { position: { latitude: 13.1, longitude: 121.1 }, heading: 90, sail: "Code 0", instant: "2026-09-25T10:20:00Z" },
    ],
  });

  assert.equal(model.destinationReached, false);
  assert.match(model.statusText, /meilleure route disponible/);
  assert.equal(model.pointCount, 2);
  assert.equal(model.durationMinutes, 20);
  assert.equal(model.steps[1].label, "Point 1");
  assert.equal(model.steps[1].terminal, false);
});

import assert from "node:assert/strict";
import test from "node:test";

import { buildGameOrders, gameOrdersToText, orderSetting, summarizeRoute } from "../src/lib/game_orders.ts";
import { buildRouteDetailsModel } from "../src/lib/route_details.ts";

// Route de 10 pas de 10 minutes : TWA qui dérive lentement, un empannage sur
// le pas qui part de +0:30, un changement de voile sur celui qui part de +1:10.
function route() {
  const start = Date.parse("2026-11-09T20:00:00Z");
  const steps = [
    { twa: 132, heading: 260, tack: "starboard", sail: "Spi" },
    { twa: 131, heading: 262, tack: "starboard", sail: "Spi" },
    { twa: 130, heading: 263, tack: "starboard", sail: "Spi" },
    { twa: -130, heading: 170, tack: "port", sail: "Spi", maneuver: "gybe" },
    { twa: -129, heading: 172, tack: "port", sail: "Spi" },
    { twa: -127, heading: 175, tack: "port", sail: "Spi" },
    { twa: -124, heading: 179, tack: "port", sail: "Spi" },
    { twa: -110, heading: 190, tack: "port", sail: "Code 0", maneuver: "sail" },
    { twa: -109, heading: 191, tack: "port", sail: "Code 0" },
    { twa: -108, heading: 192, tack: "port", sail: "Code 0" },
  ];
  const points = [{ position: { latitude: 23, longitude: -60 }, instant: new Date(start).toISOString(), stamina: 100 }];
  steps.forEach((step, index) => {
    points.push({
      position: { latitude: 23 - (index + 1) * 0.02, longitude: -60 - (index + 1) * 0.01 },
      instant: new Date(start + (index + 1) * 600000).toISOString(),
      stamina: index < 3 ? 100 : index < 7 ? 87 : 70,
      heading: step.heading,
      twa: step.twa,
      tack: step.tack,
      sail: step.sail,
      maneuver: step.maneuver,
    });
  });
  points.push({ position: { latitude: 22.7, longitude: -60.12 }, instant: new Date(start + 11 * 600000).toISOString(), terminal: true });
  return buildRouteDetailsModel({ destinationReached: true, route: points });
}

test("buildGameOrders ne crée un ordre qu'aux changements utiles (TWA)", () => {
  // Sans durée minimale : départ, empannage, dérive de la TWA à 6° de l'ordre
  // en cours, changement de voile.
  const orders = buildGameOrders(route().rows, "twa", 5, 0);
  assert.deepEqual(
    orders.map((order) => [order.elapsedMinutes, orderSetting(order, "twa"), order.sail, order.maneuver]),
    [
      [0, "TWA 132° T", "Spi", ""],
      [30, "TWA 130° B", "Spi", "gybe"],
      [60, "TWA 124° B", "Spi", ""],
      [70, "TWA 110° B", "Code 0", "sail"],
    ],
  );
  // Chaque ordre dure jusqu'au suivant ; le dernier jusqu'à l'arrivée (+1:50).
  assert.deepEqual(orders.map((order) => order.durationMinutes), [30, 30, 10, 40]);
});

test("buildGameOrders fond une dérive trop courte dans l'ordre précédent", () => {
  // La dérive à +1:00 ne dure que 10 min avant le changement de voile.
  const orders = buildGameOrders(route().rows, "twa");
  assert.deepEqual(
    orders.map((order) => [order.elapsedMinutes, orderSetting(order, "twa"), order.durationMinutes]),
    [
      [0, "TWA 132° T", 30],
      [30, "TWA 130° B", 40],
      [70, "TWA 110° B", 40],
    ],
  );
});

test("buildGameOrders suit le cap quand on le demande", () => {
  const orders = buildGameOrders(route().rows, "heading");
  assert.deepEqual(
    orders.map((order) => [order.elapsedMinutes, orderSetting(order, "heading")]),
    [
      [0, "Cap 260°"],
      [30, "Cap 170°"],
      [70, "Cap 190°"],
    ],
  );
});

test("gameOrdersToText donne une ligne par ordre", () => {
  const text = gameOrdersToText(buildGameOrders(route().rows, "twa"), "twa");
  const lines = text.split("\n");
  assert.equal(lines.length, 3);
  assert.match(lines[1], / · TWA 130° B · Spi · Empannage$/);
  assert.match(lines[2], / · TWA 110° B · Code 0 · Changement de voile$/);
});

test("summarizeRoute donne arrivée, distance et stamina finale", () => {
  const model = route();
  const summary = summarizeRoute(model.rows, model.maneuverCount, model.durationMinutes);
  assert.equal(summary.endInstant, "2026-11-09T21:50:00.000Z");
  assert.equal(summary.durationMinutes, 110);
  assert.equal(summary.maneuverCount, 2);
  assert.equal(summary.finalStamina, 70);
  // 10 pas d'environ 1,3 NM, plus le dernier pas vers l'arrivée.
  assert.ok(summary.distanceNM > 13 && summary.distanceNM < 20, String(summary.distanceNM));
});

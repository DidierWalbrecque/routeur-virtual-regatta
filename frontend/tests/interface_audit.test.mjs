import assert from "node:assert/strict";
import test from "node:test";

import { splitContour } from "../src/lib/geo.ts";
import { buildRouteDetailsModel, constantColumns, formatAngle, formatNumber } from "../src/lib/route_details.ts";
import { friendlyRouteError } from "../src/lib/route_errors.ts";
import { exportFileName, routeRowsToTsv, routeToGpx } from "../src/lib/route_export.ts";
import { shortRunLabel } from "../src/lib/weather_run.ts";

test("splitContour coupe le contour au saut qui traverse une île", () => {
  const points = [
    { latitude: 16, longitude: -61.8 },
    { latitude: 16.01, longitude: -61.8 },
    { latitude: 16.02, longitude: -61.8 },
    { latitude: 16.03, longitude: -61.8 },
    // saut de 30 milles : le front contourne la terre
    { latitude: 16.53, longitude: -61.8 },
    { latitude: 16.54, longitude: -61.8 },
    { latitude: 16.55, longitude: -61.8 },
  ];
  const parts = splitContour(points);
  assert.equal(parts.length, 2);
  assert.equal(parts[0].length, 4);
  assert.equal(parts[1].length, 3);
  // Un contour régulier reste d'un seul tenant.
  assert.equal(splitContour(points.slice(0, 4)).length, 1);
  assert.deepEqual(splitContour([]), []);
});

test("friendlyRouteError explique un départ bloqué par la côte", () => {
  const message = friendlyRouteError(
    "calcul des isochrones impossible: routing: aucune trajectoire autorisée à la couche 1",
    { coastalMarginNM: 2, enabledZoneCount: 3 },
  );
  assert.match(message, /^Aucun cap possible au départ/);
  assert.match(message, /marge de 2 NM/);
  assert.match(message, /réduisez la marge/);
  assert.match(message, /décochez la zone/);
  assert.doesNotMatch(message, /couche|routing/);

  const noMargin = friendlyRouteError("routing: aucune trajectoire autorisée à la couche 1", { coastalMarginNM: 0, enabledZoneCount: 0 });
  assert.equal(noMargin, "Aucun cap possible au départ : toutes les directions sont bloquées par la côte. Pour calculer, déplacez le départ.");

  assert.match(friendlyRouteError("routing: aucune trajectoire autorisée à la couche 7", { coastalMarginNM: 1.5, enabledZoneCount: 0 }), /après 6 pas.*1,5 NM/);
  assert.match(friendlyRouteError("le point de départ se trouve dans une zone interdite", { coastalMarginNM: 2, enabledZoneCount: 1 }), /^Le départ est dans une zone interdite/);
  assert.equal(friendlyRouteError("autre panne", { coastalMarginNM: 2, enabledZoneCount: 0 }), "Autre panne");
});

const route = {
  destinationReached: true,
  route: [
    { position: { latitude: 16, longitude: -61.7 }, instant: "2026-11-08T12:00:00Z", stamina: 82.8 },
    {
      position: { latitude: 16.1, longitude: -61.8 }, instant: "2026-11-08T12:10:00Z", stamina: 47,
      heading: 105.1, twa: 113.04, twd: 70, tws: 9.72, tack: "port", speedKnots: 18.62, foilPercent: 0, speedCoefficient: 1,
      sail: "Code 0", maneuver: "gybe",
    },
    { position: { latitude: 16.2, longitude: -61.9 }, instant: "2026-11-08T12:20:00Z", terminal: true },
  ],
};

test("les nombres du tableau sont à la française, avec une décimale", () => {
  assert.equal(formatNumber(18.62, 1), "18,6");
  assert.equal(formatAngle(113.04), "113");
  assert.equal(formatAngle(105.14), "105,1");
  assert.equal(formatNumber(null, 1), "—");
});

test("constantColumns repère les colonnes Foils et Coeff sans variation", () => {
  const model = buildRouteDetailsModel(route);
  assert.deepEqual([...constantColumns(model.rows)].sort(), ["coeff", "foils"]);
});

test("routeToGpx exporte la trace et les manœuvres", () => {
  const gpx = routeToGpx(route);
  assert.match(gpx, /<gpx version="1.1"/);
  assert.equal((gpx.match(/<trkpt /g) || []).length, 3);
  assert.match(gpx, /<wpt lat="16.000000" lon="-61.700000"><time>2026-11-08T12:00:00.000Z<\/time><name>Empannage<\/name><\/wpt>/);
  assert.match(gpx, /<desc>cap 105° · Code 0<\/desc>/);
});

test("routeRowsToTsv produit un tableau à coller dans un tableur", () => {
  const lines = routeRowsToTsv(buildRouteDetailsModel(route).rows).split("\n");
  assert.equal(lines.length, 4);
  assert.match(lines[0], /^Date\tTemps écoulé\tCap/);
  const first = lines[1].split("\t");
  assert.equal(first[2], "105");
  assert.equal(first[8], "18,6");
  assert.equal(first[12], "82,8");
  assert.equal(first[13], "Empannage");
});

test("exportFileName et shortRunLabel", () => {
  assert.match(exportFileName("2026-11-08T12:00:00Z", "gpx"), /^route-2026-11-08-\d{4}\.gpx$/);
  assert.equal(shortRunLabel("2026110718"), "18Z du 07/11");
  assert.equal(shortRunLabel("n'importe"), "n'importe");
});

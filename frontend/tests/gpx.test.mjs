import assert from "node:assert/strict";
import test from "node:test";

import { parseGpx, gpxFeatureCollection, gpxPointCount } from "../src/lib/gpx.ts";

const TRACK = `<?xml version="1.0" encoding="UTF-8"?>
<gpx version="1.1" xmlns="http://www.topografix.com/GPX/1/1">
  <wpt lat="13.12" lon="121.58"><name>Départ &amp; cap</name></wpt>
  <wpt lat="13.17" lon="121.33"/>
  <trk><name>Course</name>
    <trkseg>
      <trkpt lat="13.121" lon="121.586"><ele>0</ele></trkpt>
      <trkpt lat="13.140" lon="121.500"></trkpt>
      <trkpt lat="13.171" lon="121.336"/>
    </trkseg>
  </trk>
  <rte>
    <rtept lon="121.5" lat="13.0"/>
    <rtept lat="13.2" lon="121.2"/>
  </rte>
</gpx>`;

test("parseGpx lit segments de trace, routes et points de passage", () => {
  const geo = parseGpx(TRACK);
  assert.equal(geo.lines.length, 2, "un trkseg + une rte");
  assert.deepEqual(geo.lines[0][0], { latitude: 13.121, longitude: 121.586 });
  assert.equal(geo.lines[0].length, 3);
  assert.equal(geo.lines[1].length, 2, "route à deux points");
  assert.equal(geo.waypoints.length, 2);
  assert.equal(geo.waypoints[0].name, "Départ & cap", "entités XML décodées");
  assert.equal(geo.waypoints[1].name, undefined, "point sans nom accepté");
  assert.equal(gpxPointCount(geo), 7);
});

test("gpxFeatureCollection produit des LineString et des Point en lon,lat", () => {
  const collection = gpxFeatureCollection(parseGpx(TRACK));
  const lines = collection.features.filter((f) => f.geometry.type === "LineString");
  const points = collection.features.filter((f) => f.geometry.type === "Point");
  assert.equal(lines.length, 2);
  assert.equal(points.length, 2);
  assert.deepEqual(lines[0].geometry.coordinates[0], [121.586, 13.121], "ordre GeoJSON lon,lat");
  assert.equal(points[0].properties.name, "Départ & cap");
});

test("parseGpx ignore les coordonnées invalides et les segments trop courts", () => {
  const geo = parseGpx(`<gpx>
    <trk><trkseg>
      <trkpt lat="99" lon="0"/>
      <trkpt lat="abc" lon="10"/>
      <trkpt lat="1" lon="1"/>
    </trkseg></trk>
  </gpx>`);
  assert.equal(geo.lines.length, 0, "un seul point valide : pas de ligne");
});

test("parseGpx tolère un fichier vide ou sans trace", () => {
  assert.deepEqual(parseGpx("<gpx></gpx>"), { lines: [], waypoints: [] });
  assert.deepEqual(parseGpx(""), { lines: [], waypoints: [] });
});

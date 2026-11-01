import assert from "node:assert/strict";
import test from "node:test";

import { matchPolar } from "../src/lib/polar_match.ts";

const polars = [
  { label: "class_40.json", value: "../data/polars/class_40.json", id: 3, boat: "mono/class40" },
  { label: "imoca_60_foils.json", value: "../data/polars/imoca_60_foils.json", id: 4, boat: "mono/imoca_60_foils" },
  { label: "super_maxi_100.json", value: "../data/polars/super_maxi_100.json", id: 14, boat: "mono/super_maxi_100" },
  { label: "ultim_bp_11.json", value: "../data/polars/ultim_bp_11.json", id: 21, boat: "Ultim_BP_XI" },
];

test("une clé polaire numérique est rapprochée de l'identifiant du fichier", () => {
  const match = matchPolar(polars, [{ path: "res.leg.boat.polar_id", value: "14" }]);
  assert.equal(match?.polar.label, "super_maxi_100.json");
});

test("un nom de bateau est rapproché malgré la casse et la ponctuation", () => {
  assert.equal(matchPolar(polars, [{ path: "res.bs.boatName", value: "IMOCA 60 Foils" }])?.polar.id, 4);
  assert.equal(matchPolar(polars, [{ path: "res.bs.boat", value: "Class 40" }])?.polar.id, 3);
  assert.equal(matchPolar(polars, [{ path: "res.leg.boatLabel", value: "mono/ultim_bp_11" }])?.polar.id, 21);
});

test("l'identifiant de polaire prime sur un nom contradictoire", () => {
  const match = matchPolar(polars, [
    { path: "res.bs.boatName", value: "Class 40" },
    { path: "res.leg.polarId", value: "4" },
  ]);
  assert.equal(match?.polar.id, 4);
});

test("sans indice exploitable, aucune polaire n'est choisie", () => {
  assert.equal(matchPolar(polars, undefined), null);
  assert.equal(matchPolar(polars, []), null);
  assert.equal(matchPolar(polars, [{ path: "res.leg.boatClass", value: "9" }]), null);
  assert.equal(matchPolar(polars, [{ path: "res.bs.polarId", value: "999" }]), null);
  assert.equal(matchPolar([], [{ path: "res.bs.polarId", value: "4" }]), null);
});

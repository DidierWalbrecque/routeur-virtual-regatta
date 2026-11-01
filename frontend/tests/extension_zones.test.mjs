import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import vm from "node:vm";

const source = readFileSync(new URL("../../vr-state-extension/page-hook.js", import.meta.url), "utf8");

// Exécute le script de l'extension dans un faux navigateur, lui fait « recevoir »
// les réponses données et renvoie l'état publié après chacune.
async function publishedStates(bodies) {
  const messages = [];
  let current = null;
  const window = {
    fetch: () => Promise.resolve({ ok: true, clone: () => ({ json: async () => current }) }),
    postMessage: (message) => messages.push(message),
  };
  function XMLHttpRequest() {}
  XMLHttpRequest.prototype.open = () => {};
  XMLHttpRequest.prototype.send = () => {};
  vm.runInContext(source, vm.createContext({ window, XMLHttpRequest }));

  const states = [];
  for (const body of bodies) {
    current = body;
    await window.fetch("https://vro-api-client.prod.virtualregatta.com/getboatinfos");
    await new Promise((resolve) => setTimeout(resolve, 5));
    // Les objets viennent d'un autre contexte : on les recopie pour comparer.
    states.push(JSON.parse(JSON.stringify(messages.at(-1)?.state)));
  }
  return states;
}

const boat = { pos: { lat: 49.1, lon: -4.2 }, heading: 284, speed: 20.9, sail: 2, options: ["foil"], _id: { race_id: 845, leg_num: 1 } };
const triangle = [
  { lat: 50, lon: -5.5 },
  { lat: 50.1, lon: -5.3 },
  { lat: 49.9, lon: -5.3 },
];

test("les zones à sommets {lat, lon} sont relevées avec leur nom", async () => {
  const [state] = await publishedStates([
    { res: { bs: boat, leg: { restrictedZones: [{ id: 1, name: "Cornouailles", vertices: triangle }] } } },
  ]);
  assert.equal(state.zones.length, 1);
  assert.equal(state.zones[0].name, "Cornouailles");
  assert.deepEqual(state.zones[0].points[0], { latitude: 50, longitude: -5.5 });
  assert.equal(state.zones[0].points.length, 3);
});

test("un polygone GeoJSON est lu dans l'ordre longitude, latitude et refermé", async () => {
  const [state] = await publishedStates([
    {
      res: {
        bs: boat,
        leg: {
          exclusionZones: [
            {
              type: "Feature",
              properties: { name: "Banc" },
              geometry: { type: "Polygon", coordinates: [[[-5, 50], [-4, 50], [-4, 51], [-5, 50]]] },
            },
          ],
        },
      },
    },
  ]);
  assert.equal(state.zones[0].name, "Banc");
  assert.equal(state.zones[0].points.length, 3);
  assert.deepEqual(state.zones[0].points[0], { latitude: 50, longitude: -5 });
});

test("les listes de paires sont lues comme [latitude, longitude]", async () => {
  const [state] = await publishedStates([
    { res: { bs: boat, leg: { forbiddenAreas: [[[50, -5], [50, -4], [51, -4]]] } } },
  ]);
  assert.equal(state.zones.length, 1);
  assert.deepEqual(state.zones[0].points[1], { latitude: 50, longitude: -4 });
});

test("portes, limites des glaces et formes incomplètes ne sont pas des zones", async () => {
  const [state] = await publishedStates([
    {
      res: {
        bs: boat,
        leg: {
          checkpoints: [{ group: 1, id: 1, start: { lat: 1, lon: 1 }, end: { lat: 2, lon: 2 }, vertices: triangle }],
          ice_limits: [{ lat: 60, lon: -10 }, { lat: 60, lon: 0 }, { lat: 61, lon: 0 }],
          restrictedZones: [{ name: "trop courte", vertices: triangle.slice(0, 2) }, { name: "texte", vertices: "abc" }],
        },
      },
    },
  ]);
  assert.deepEqual(state.zones, []);
  assert.ok(state.checkpoints.length >= 1);
});

test("les zones sont conservées quand une réponse n'a pas de manche, et la structure est résumée", async () => {
  const [first, second] = await publishedStates([
    { res: { bs: boat, leg: { restrictedZones: [{ name: "Z", vertices: triangle }], other: { a: 1 } } } },
    { res: { bs: boat } },
  ]);
  assert.equal(first.zones.length, 1);
  assert.equal(second.zones.length, 1);
  const shape = first.legShape.map((entry) => entry.path);
  assert.ok(shape.includes("res.leg.restrictedZones"));
  assert.ok(first.legShape.find((entry) => entry.path === "res.leg.restrictedZones").value.startsWith("tableau[1]"));
});

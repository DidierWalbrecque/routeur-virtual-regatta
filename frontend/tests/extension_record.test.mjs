import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import vm from "node:vm";

import { vrStatusText } from "../src/lib/vr.ts";

const source = readFileSync(new URL("../../vr-state-extension/page-hook.js", import.meta.url), "utf8");
// Réponse getboatinfos du Trophée Jules Verne avant le départ : bs est nul.
const record = JSON.parse(readFileSync(new URL("../../data/data-record.json", import.meta.url), "utf8"));

// Exécute le script de l'extension dans un faux navigateur et renvoie, pour
// chaque réponse reçue, l'état publié (null si rien ne l'a été).
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
    const before = messages.length;
    current = body;
    await window.fetch("https://vro-api-client.prod.virtualregatta.com/getboatinfos");
    await new Promise((resolve) => setTimeout(resolve, 5));
    states.push(messages.length > before ? JSON.parse(JSON.stringify(messages.at(-1).state)) : null);
  }
  return states;
}

const classicLeg = { _id: { race_id: 700, num: 1 }, race: { type: "leg" }, start: { lat: 1, lon: 2 } };
const classicBoat = { _id: { race_id: 700, leg_num: 1 }, pos: { lat: 10, lon: 20 }, gateGroupCounters: [1, 1, 1] };

test("un record avant le départ est publié depuis la ligne de départ", async () => {
  const [state] = await publishedStates([record]);
  assert.ok(state);
  assert.equal(state.preStart, true);
  assert.equal(state.raceType, "record");
  assert.equal(state.raceId, "817");
  assert.equal(state.legNum, "1");
  assert.deepEqual(state.position, { latitude: 48.52034, longitude: -5.143132 });
  assert.equal(state.legStart.name, "Ouessant");
  assert.equal(state.arrival.name, "Brest");
  assert.deepEqual(state.startWindow, { open: 1794142804188, close: 1806620340000 });
  assert.equal(state.endDate, 1806015540000);
  assert.equal(state.stamina, null);
  assert.deepEqual(state.options, []);
  assert.ok(state.checkpoints.length > 0);
  assert.ok(state.boatHints.some((hint) => hint.path === "res.leg.boat.polar_id" && hint.value === "21"));
});

test("les portes franchies d'une autre course ne s'appliquent pas au record", async () => {
  const [, state] = await publishedStates([{ res: { bs: classicBoat, leg: classicLeg } }, record]);
  assert.deepEqual(state.gateGroupCounters, []);
});

test("une course classique sans état de bateau ne publie rien", async () => {
  const [state] = await publishedStates([{ res: { bs: null, leg: classicLeg } }]);
  assert.equal(state, null);
});

test("après le départ du record, l'état live garde le type et les portes du leg", async () => {
  const [, state] = await publishedStates([
    record,
    { res: { bs: { _id: { raceId: 817, legNum: 1 }, pos: { lat: 47, lon: -7 }, gateGroupCounters: [1] } } },
  ]);
  assert.equal(state.preStart, undefined);
  assert.equal(state.raceType, "record");
  assert.deepEqual(state.position, { latitude: 47, longitude: -7 });
  assert.ok(state.checkpoints.length > 0);
  assert.deepEqual(state.gateGroupCounters, [true]);
});

test("le départ et l'arrivée restent connus quand une réponse n'a ni manche ni numéro d'étape", async () => {
  const leg = { _id: { race_id: 845, num: 1 }, race: { type: "leg" }, start: { lat: 46, lon: -1.8, name: "Les Sables" }, end: { lat: 16.2, lon: -61.5, name: "Pointe-à-Pitre" } };
  const states = await publishedStates([
    { res: { bs: { _id: { race_id: 845 }, pos: { lat: 45, lon: -5 } }, leg } },
    { res: { bs: { _id: { race_id: 845 }, pos: { lat: 44.9, lon: -5.1 } } } },
  ]);
  for (const state of states) {
    assert.equal(state.legStart?.name, "Les Sables");
    assert.equal(state.arrival?.name, "Pointe-à-Pitre");
  }
});

test("chaque course garde son départ, son arrivée et ses portes franchies", async () => {
  const states = await publishedStates([
    { res: { bs: classicBoat, leg: { ...classicLeg, end: { lat: 3, lon: 4, name: "Arrivée 700" } } } },
    record,
    { res: { bs: { _id: { race_id: 700, leg_num: 1 }, pos: { lat: 11, lon: 21 } } } },
  ]);
  assert.equal(states[1].arrival.name, "Brest");
  assert.deepEqual(states[1].gateGroupCounters, []);
  assert.equal(states[2].arrival.name, "Arrivée 700");
  assert.deepEqual(states[2].legStart, { latitude: 1, longitude: 2, name: "" });
  assert.deepEqual(states[2].gateGroupCounters, [true, true, true]);
});

test("une manche reçue sans identifiant de course n'est pas oubliée", async () => {
  const [, state] = await publishedStates([
    { res: { bs: { pos: { lat: 45, lon: -5 } }, leg: { start: { lat: 46, lon: -1.8, name: "Les Sables" } } } },
    { res: { bs: { _id: { race_id: 845, leg_num: 1 }, pos: { lat: 44.9, lon: -5.1 } } } },
  ]);
  assert.equal(state.legStart?.name, "Les Sables");
});

test("un changement d'étape connu oublie les données de l'étape précédente", async () => {
  const [, state] = await publishedStates([
    { res: { bs: { _id: { race_id: 845, leg_num: 1 }, pos: { lat: 45, lon: -5 }, gateGroupCounters: [1] }, leg: { start: { lat: 46, lon: -1.8 } } } },
    { res: { bs: { _id: { race_id: 845, leg_num: 2 }, pos: { lat: 16, lon: -61 } } } },
  ]);
  assert.equal(state.legStart, null);
  assert.deepEqual(state.gateGroupCounters, []);
});

test("le statut annonce un record dont le départ n'est pas pris", () => {
  const text = vrStatusText({ preStart: true, raceType: "record", startWindow: { open: 1, close: Date.UTC(2027, 3, 1, 20, 59) } });
  assert.match(text, /^Record, départ non pris · fermeture le 0[12]\/04\/2027/);
  assert.match(vrStatusText({ raceType: "record", stamina: 87.25 }), /^Record en cours · .*stamina 87\.3 %/);
  assert.match(vrStatusText({ stamina: null }), /^Virtual Regatta connecté · stamina inconnue · 0 porte/);
});

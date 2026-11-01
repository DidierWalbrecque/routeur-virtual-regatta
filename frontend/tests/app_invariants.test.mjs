import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const source = readFileSync(new URL("../src/App.tsx", import.meta.url), "utf8");

function functionBody(name) {
  const start = source.indexOf(`const ${name} = `);
  assert.ok(start >= 0, `${name} introuvable dans App.tsx`);
  const next = source.indexOf("\n  // ---", start);
  return source.slice(start, next > start ? next : undefined);
}

test("la lecture de l'état Virtual Regatta n'efface pas la route", () => {
  const body = functionBody("refreshVrState");
  assert.ok(!body.includes("resetRoute"));
  assert.ok(!body.includes("updateEndpoints"));
  assert.ok(!body.includes("setFrame"));
});

test("l'actualisation des GRIB ne change pas l'heure de départ", () => {
  const body = functionBody("refreshWeather");
  assert.ok(!body.includes("instant"));
  assert.ok(!body.includes("resetRoute"));
});

test("l'actualisation des GRIB demande les prévisions de l'horizon choisi", () => {
  const body = functionBody("refreshWeather");
  assert.match(body, /forecastHours: forecastHoursFor\(form\.horizon\)/);
});

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
  assert.ok(body.includes("forecastHours: GRIB_FORECAST_HOURS"));
});

test("la lecture de l'état VR n'écrase le départ que si la position vient de VR", () => {
  const body = functionBody("refreshVrState");
  assert.ok(body.includes('previous.positionSource === "vr"'));
  assert.ok(!body.includes("previous.useVrState"));
});

test("le calcul envoie les trois réglages découplés et plus l'ancien drapeau unique", () => {
  const body = functionBody("submit");
  assert.ok(body.includes("positionSource: form.positionSource"));
  assert.ok(body.includes("staminaSource: form.staminaSource"));
  assert.ok(body.includes("useCourseGates: form.useCourseGates"));
  assert.match(body, /manualStamina: form\.staminaSource === "manual" \? Number\(form\.staminaManual\) : null/);
  assert.ok(!body.includes("useVirtualRegattaState"));
});

test("loadForm migre l'ancien réglage unique vers les trois réglages découplés", () => {
  assert.ok(source.includes("typeof saved.useVrState"));
  assert.match(source, /merged\.positionSource = vr \? "vr" : "manual"/);
  assert.match(source, /merged\.useCourseGates = vr/);
});

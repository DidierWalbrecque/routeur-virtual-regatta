import assert from "node:assert/strict";
import test from "node:test";

import { formatRunLabel, weatherSwitchMessage } from "../src/lib/weather_switch.ts";

const partial = {
  latestRun: "2026110718",
  fallbackRun: "2026110712",
  downloaded: 5,
  expected: 129,
  switchAt: "2026-11-08T06:00:00Z",
  affectsRoute: true,
};

test("formatRunLabel rend le run lisible", () => {
  assert.equal(formatRunLabel("2026110718"), "18Z du 07/11");
  assert.equal(formatRunLabel("inconnu"), "inconnu");
});

test("weatherSwitchMessage signale la bascule vers le run précédent", () => {
  assert.equal(
    weatherSwitchMessage(partial),
    "Vent du run 18Z du 07/11 (5/129 échéances téléchargées) jusqu'au 08/11 06:00 UTC, puis du run 12Z du 07/11, plus ancien, pour la suite de la route.",
  );
  assert.equal(
    weatherSwitchMessage({ ...partial, affectsRoute: false }),
    "Vent du run 18Z du 07/11 (5/129 échéances téléchargées), qui couvre toute la route.",
  );
});

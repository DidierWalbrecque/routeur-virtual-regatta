import assert from "node:assert/strict";
import test from "node:test";

import { parseSSEBlock, readSSEStream } from "../static/route_stream.js";

test("parseSSEBlock decodes event name and JSON data", () => {
  assert.deepEqual(
    parseSSEBlock('event: frontier\ndata: {"completedLayers":2}'),
    { event: "frontier", data: { completedLayers: 2 } },
  );
});

test("readSSEStream handles fragmented events in order", async () => {
  const encoder = new TextEncoder();
  const chunks = [
    encoder.encode('event: start\ndata: {"totalLayers":3}\n\n'),
    encoder.encode('event: front'),
    encoder.encode('ier\ndata: {"completedLayers":1}\n\n'),
    encoder.encode('event: complete\ndata: {"destinationReached":true}\n\n'),
  ];
  const events = [];
  const response = new Response(
    new ReadableStream({
      pull(controller) {
        const chunk = chunks.shift();
        if (chunk) controller.enqueue(chunk);
        else controller.close();
      },
    }),
    { status: 200 },
  );

  await readSSEStream(response, (event, data) => events.push({ event, data }));

  assert.deepEqual(events, [
    { event: "start", data: { totalLayers: 3 } },
    { event: "frontier", data: { completedLayers: 1 } },
    { event: "complete", data: { destinationReached: true } },
  ]);
});

test("readSSEStream exposes server errors", async () => {
  await assert.rejects(
    readSSEStream(
      new Response(JSON.stringify({ error: "calcul impossible" }), {
        status: 400,
        headers: { "Content-Type": "application/json" },
      }),
      () => {},
    ),
    /calcul impossible/,
  );
});

test("readSSEStream propagates an aborted reader", async () => {
  const response = {
    ok: true,
    body: {
      getReader() {
        return { read: async () => { throw new DOMException("aborted", "AbortError"); } };
      },
    },
  };
  await assert.rejects(readSSEStream(response, () => {}), { name: "AbortError" });
});

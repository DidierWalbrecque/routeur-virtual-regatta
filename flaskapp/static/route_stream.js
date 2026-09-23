export function parseSSEBlock(block) {
  let event = "message";
  const dataLines = [];
  for (const line of block.split(/\r?\n/)) {
    if (line.startsWith("event:")) event = line.slice(6).trim();
    if (line.startsWith("data:")) dataLines.push(line.slice(5).trimStart());
  }
  if (dataLines.length === 0) return null;
  return { event, data: JSON.parse(dataLines.join("\n")) };
}

export async function readSSEStream(response, onEvent) {
  if (!response.ok) {
    let message = `HTTP ${response.status}`;
    try {
      const body = await response.json();
      message = body.error || message;
    } catch {
      // Le serveur peut renvoyer un corps d'erreur non JSON.
    }
    throw new Error(message);
  }
  if (!response.body || typeof response.body.getReader !== "function") {
    throw new Error("Le navigateur ne prend pas en charge la lecture du flux SSE.");
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  while (true) {
    const { value, done } = await reader.read();
    buffer += decoder.decode(value || new Uint8Array(), { stream: !done });
    const blocks = buffer.split(/\r?\n\r?\n/);
    buffer = blocks.pop() || "";
    for (const block of blocks) {
      const parsed = parseSSEBlock(block.trim());
      if (parsed) onEvent(parsed.event, parsed.data);
    }
    if (done) break;
  }
  const parsed = parseSSEBlock(buffer.trim());
  if (parsed) onEvent(parsed.event, parsed.data);
}

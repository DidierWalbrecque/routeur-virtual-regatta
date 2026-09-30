const flaskStateEndpoint = "http://localhost:5000/api/vr/state";

async function sendStateToFlask(state) {
  try {
    await fetch(flaskStateEndpoint, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(state),
    });
  } catch {
    // Le routeur Go peut être arrêté : la popup continue de fonctionner.
  }
}

chrome.runtime.onMessage.addListener((message, sender) => {
  if (!message || message.type !== "vr-state" || !message.state) return;

  const latestState = {
    ...message.state,
    receivedAt: Date.now(),
    frameId: sender.frameId,
    pageUrl: sender.url || "",
  };
  void chrome.storage.local.set({ latestState });
  void sendStateToFlask(latestState);
});

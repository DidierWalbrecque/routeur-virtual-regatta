(() => {
  const script = document.createElement("script");
  script.src = chrome.runtime.getURL("page-hook.js");
  script.onload = () => script.remove();
  (document.head || document.documentElement).appendChild(script);

  window.addEventListener("message", (event) => {
    if (event.source !== window) return;
    const message = event.data;
    if (!message || message.source !== "vr-state-lite" || message.type !== "boat-state") return;

    chrome.runtime.sendMessage({ type: "vr-state", state: message.state });
  });
})();

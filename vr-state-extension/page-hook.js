(() => {
  if (window.__vrStateLiteInstalled) return;
  window.__vrStateLiteInstalled = true;

  const boatInfoPattern = /\/getboatinfos(?:[/?]|$)/i;
  const sailDefinitions = [
    { id: 1, name: "Jib", option: null },
    { id: 2, name: "Spi", option: null },
    { id: 3, name: "Staysail", option: "heavy" },
    { id: 4, name: "Light Jib", option: "light" },
    { id: 5, name: "Code 0", option: "reach" },
    { id: 6, name: "Heavy Gennaker", option: "heavy" },
    { id: 7, name: "Light Gennaker", option: "light" },
  ];
  let latestCheckpoints = [];
  let latestGateGroupCounters = [];

  function availableSails(options, fullOptions) {
    const normalizedOptions = new Set(options.map((option) => String(option).toLowerCase()));
    return sailDefinitions
      .filter((sail) => fullOptions || !sail.option || normalizedOptions.has(sail.option))
      .map(({ id, name }) => ({ id, name }));
  }

  function sailName(sailId) {
    const normalizedId = ((Number(sailId) % 10) + 10) % 10;
    return sailDefinitions.find((sail) => sail.id === normalizedId)?.name || String(sailId ?? "—");
  }

  function text(value) {
    return value == null ? "" : String(value);
  }

  function checkpointPosition(value) {
    const latitude = Number(value?.lat ?? value?.latitude);
    const longitude = Number(value?.lon ?? value?.lng ?? value?.longitude);
    if (!Number.isFinite(latitude) || !Number.isFinite(longitude)) return null;
    if (latitude < -90 || latitude > 90 || longitude < -180 || longitude > 180) return null;
    return { latitude, longitude };
  }

  function checkpointsFromLeg(leg) {
    if (!Array.isArray(leg?.checkpoints)) return [];
    return leg.checkpoints.flatMap((checkpoint, index) => {
      const start = checkpointPosition(checkpoint?.start);
      const end = checkpointPosition(checkpoint?.end);
      if (!start || !end || (start.latitude === end.latitude && start.longitude === end.longitude)) return [];
      const group = Number(checkpoint?.group);
      const id = Number(checkpoint?.id);
      return [{
        group: Number.isFinite(group) ? group : index + 1,
        id: Number.isFinite(id) ? id : index + 1,
        name: text(checkpoint?.name || checkpoint?.display || `Checkpoint ${index + 1}`),
        display: text(checkpoint?.display),
        engine: text(checkpoint?.engine),
        rhumb: checkpoint?.rhumb === true || String(checkpoint?.rhumb).toLowerCase() === "true",
        side: text(checkpoint?.side).toLowerCase(),
        start,
        end,
      }];
    });
  }

  function gateCounters(value) {
    if (!Array.isArray(value)) return null;
    return value.map((counter) => counter === true || Number(counter) > 0);
  }

  function publish(body, url) {
    const boat = body?.res?.bs || body?.bs || body?.res?.boat;
    if (!boat || !boat.pos || !Number.isFinite(Number(boat.pos.lat)) || !Number.isFinite(Number(boat.pos.lon))) return;

    const id = boat._id || {};
    const options = Array.isArray(boat.options)
      ? boat.options.slice()
      : boat.options == null
        ? []
        : [boat.options];

    const fullOptions = boat.fullOptions === true;
    const currentSail = boat.sail ?? null;
    const leg = body?.res?.leg || body?.leg || null;
    if (Array.isArray(leg?.checkpoints)) {
      latestCheckpoints = checkpointsFromLeg(leg);
    }
    const counters = gateCounters(boat.gateGroupCounters);
    if (counters) latestGateGroupCounters = counters;

    window.postMessage({
      source: "vr-state-lite",
      type: "boat-state",
      state: {
        raceId: text(id.raceId ?? id.race_id),
        legNum: text(id.legNum ?? id.leg_num),
        userId: text(id.user_id ?? id.userId),
        displayName: boat.displayName || "",
        position: {
          latitude: Number(boat.pos.lat),
          longitude: Number(boat.pos.lon),
        },
        stamina: Number.isFinite(Number(boat.stamina)) ? Number(boat.stamina) : null,
        options,
        fullOptions,
        sails: availableSails(options, fullOptions),
        currentSail: currentSail == null ? null : {
          id: Number(currentSail) % 10,
          name: sailName(currentSail),
          automatic: Number(currentSail) >= 10,
        },
        heading: Number.isFinite(Number(boat.heading)) ? Number(boat.heading) : null,
        speed: Number.isFinite(Number(boat.speed)) ? Number(boat.speed) : null,
        state: boat.state || "",
        updatedAt: boat.lastCalcDate ?? null,
        checkpoints: latestCheckpoints,
        gateGroupCounters: latestGateGroupCounters,
        sourceUrl: url,
      },
    }, "*");
  }

  async function inspectResponse(response, url) {
    try {
      if (!response.ok) return;
      publish(await response.clone().json(), url);
    } catch {
      // The game can return non-JSON responses during loading.
    }
  }

  const nativeFetch = window.fetch;
  window.fetch = function (...args) {
    const request = args[0];
    const url = typeof request === "string" ? request : request?.url || String(request);
    const result = Reflect.apply(nativeFetch, this, args);
    if (boatInfoPattern.test(url)) result.then((response) => inspectResponse(response, url));
    return result;
  };

  const nativeOpen = XMLHttpRequest.prototype.open;
  const nativeSend = XMLHttpRequest.prototype.send;
  XMLHttpRequest.prototype.open = function (method, url, ...rest) {
    this.__vrStateLiteUrl = String(url);
    return Reflect.apply(nativeOpen, this, [method, url, ...rest]);
  };
  XMLHttpRequest.prototype.send = function (...args) {
    if (boatInfoPattern.test(this.__vrStateLiteUrl || "")) {
      this.addEventListener("load", () => {
        try {
          publish(JSON.parse(this.responseText), this.__vrStateLiteUrl);
        } catch {
          // Ignore incomplete or non-JSON responses.
        }
      });
    }
    return Reflect.apply(nativeSend, this, args);
  };
})();

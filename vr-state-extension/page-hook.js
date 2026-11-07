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
  let latestArrival = null;

  // Arrivée officielle du leg : fin du leg (lat, lon, rayon, nom).
  function legEndOf(leg) {
    const end = leg?.end;
    const latitude = Number(end?.lat);
    const longitude = Number(end?.lon);
    if (!Number.isFinite(latitude) || !Number.isFinite(longitude)) return null;
    const radius = Number(end?.radius);
    return {
      latitude,
      longitude,
      radius: Number.isFinite(radius) && radius > 0 ? radius : null,
      name: text(end?.name),
    };
  }

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

  // Repère les valeurs de la réponse du jeu qui peuvent identifier le bateau
  // (identifiant de polaire, nom du bateau…). Elles sont transmises telles
  // quelles ; l'application choisit la polaire correspondante.
  const boatHintKeys = /polar|boat|hull|yacht|class/i;
  const boatHintIgnoredKeys = /^(displayName|checkpoints|options|sails|pos|_id)$/i;

  function collectBoatHints(roots) {
    const hints = [];
    const seen = new Set();
    function visit(value, path, depth) {
      if (hints.length >= 12 || depth > 4 || value == null || typeof value !== "object" || Array.isArray(value)) return;
      if (seen.has(value)) return;
      seen.add(value);
      for (const [key, child] of Object.entries(value)) {
        if (boatHintIgnoredKeys.test(key)) continue;
        const childPath = path + "." + key;
        if (typeof child === "string" || typeof child === "number") {
          if (boatHintKeys.test(key) && String(child) !== "") {
            hints.push({ path: childPath, value: String(child).slice(0, 80) });
          }
        } else {
          visit(child, childPath, depth + 1);
        }
        if (hints.length >= 12) return;
      }
    }
    roots.forEach(([path, value]) => visit(value, path, 0));
    return hints;
  }

  // --- Zones interdites de la course -----------------------------------------
  // Le jeu décrit les zones à ne pas traverser sous forme de polygones. Le nom
  // exact du champ n'étant pas documenté, on cherche, dans la description de la
  // manche, les listes dont le nom évoque une zone et dont les éléments portent
  // au moins trois sommets. Les portes (checkpoints) et les limites des glaces
  // (lignes, pas des polygones) sont écartées.
  const zoneKeyPattern = /zone|restrict|forbid|exclu|polygon|danger|obstacle/i;
  const zoneIgnoredKeyPattern = /checkpoint|ice|limit/i;
  const zoneVertexKeys = ["vertices", "points", "coords", "coordinates", "polygon", "path", "shape", "latlons", "boundary", "positions"];
  let latestZones = [];
  let latestLegShape = [];

  function samePosition(first, second) {
    return first.latitude === second.latitude && first.longitude === second.longitude;
  }

  function ringFromList(list, geojson) {
    if (!Array.isArray(list) || list.length === 0) return null;
    // Polygone GeoJSON : [ [ [lon, lat], ... ] ]
    if (Array.isArray(list[0]) && list[0].length > 0 && Array.isArray(list[0][0])) {
      return ringFromList(list[0], true);
    }
    const ring = [];
    for (const item of list) {
      const position = Array.isArray(item)
        ? checkpointPosition(geojson ? { lon: item[0], lat: item[1] } : { lat: item[0], lon: item[1] })
        : checkpointPosition(item);
      if (!position) return null;
      ring.push(position);
    }
    if (ring.length >= 2 && samePosition(ring[0], ring[ring.length - 1])) ring.pop();
    return ring.length >= 3 ? ring.slice(0, 500) : null;
  }

  function zoneFromItem(item, key, index) {
    let list = Array.isArray(item) ? item : null;
    let geojson = false;
    if (!list && item && typeof item === "object") {
      const geometry = item.geometry && typeof item.geometry === "object" ? item.geometry : item;
      for (const vertexKey of zoneVertexKeys) {
        if (Array.isArray(geometry[vertexKey])) {
          list = geometry[vertexKey];
          geojson = vertexKey === "coordinates";
          break;
        }
      }
    }
    const ring = ringFromList(list, geojson);
    if (!ring) return null;
    const label = text(item?.name || item?.display || item?.label || item?.properties?.name || item?.id);
    return { name: (label || key + " " + (index + 1)).slice(0, 60), points: ring };
  }

  function describeShape(path, value) {
    if (Array.isArray(value)) return path + " : tableau[" + value.length + "]";
    if (value && typeof value === "object") return path + " : objet{" + Object.keys(value).slice(0, 6).join(",") + "}";
    return path + " : " + typeof value;
  }

  function collectZones(roots) {
    const zones = [];
    const seen = new Set();
    function visit(value, path, depth) {
      if (depth > 4 || value == null || typeof value !== "object" || seen.has(value)) return;
      seen.add(value);
      for (const [key, child] of Object.entries(value)) {
        if (zoneIgnoredKeyPattern.test(key)) continue;
        const childPath = path + "." + key;
        if (child && typeof child === "object" && zoneKeyPattern.test(key)) {
          const items = Array.isArray(child) ? child : [child].concat(Object.values(child));
          const single = Array.isArray(child) ? null : zoneFromItem(child, key, 0);
          const found = single ? [single] : items.map((item, index) => zoneFromItem(item, key, index)).filter(Boolean);
          found.forEach((zone) => {
            if (zones.length < 50) zones.push(zone);
          });
        } else if (child && typeof child === "object" && !Array.isArray(child)) {
          visit(child, childPath, depth + 1);
        }
      }
    }
    roots.forEach(([path, value]) => visit(value, path, 0));
    return zones;
  }

  // Résumé de la structure de la manche, pour retrouver le bon champ si les
  // zones ne sont pas reconnues.
  function legShapeOf(leg) {
    if (!leg || typeof leg !== "object") return [];
    return Object.entries(leg)
      .filter(([, value]) => value !== null && typeof value === "object")
      .slice(0, 40)
      .map(([key, value]) => ({ path: "res.leg." + key, value: describeShape("", value).replace(/^ : /, "") }));
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
    const arrival = legEndOf(leg);
    if (arrival) latestArrival = arrival;
    if (leg && typeof leg === "object") {
      latestZones = collectZones([["res.leg", leg]]);
      latestLegShape = legShapeOf(leg);
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
        boatHints: collectBoatHints([["res.bs", boat], ["res.leg", leg], ["res", body?.res], ["body", body]]),
        zones: latestZones,
        legShape: latestLegShape,
        checkpoints: latestCheckpoints,
        arrival: latestArrival,
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

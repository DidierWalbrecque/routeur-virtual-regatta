import { useEffect, useRef } from "react";

type Point = { latitude: number; longitude: number };
type Isochrone = { instant: string; positions: Point[]; contour: Point[] };

// Round only a small part of each corner, inside the convex display envelope.
function roundedContour(vertices: { x: number; y: number }[]) {
  const corners = vertices.map((p, i) => {
    const previous = vertices[Math.max(0, i - 1)];
    const next = vertices[Math.min(vertices.length - 1, i + 1)];
    const before = Math.hypot(previous.x - p.x, previous.y - p.y);
    const after = Math.hypot(next.x - p.x, next.y - p.y);
    const radius = Math.min(12, before * 0.15, after * 0.15);
    const towards = (q: { x: number; y: number }, length: number) => ({ x: p.x + (q.x - p.x) * radius / (length || 1), y: p.y + (q.y - p.y) * radius / (length || 1) });
    return { p, entry: towards(previous, before), exit: towards(next, after) };
  });
  return corners.map(({ p, entry, exit }, i) => `${i ? "L" : "M"} ${entry.x},${entry.y} Q ${p.x},${p.y} ${exit.x},${exit.y}`).join(" ");
}

// Keep the connected forward-facing edges of the convex display envelope.
function forwardArc(ring: { x: number; y: number }[], direction: { x: number; y: number }) {
  const vertices = ring.slice(0, -1);
  if (Math.hypot(direction.x, direction.y) < 0.001) return [];
  const area = vertices.reduce((sum, p, i) => { const q = vertices[(i + 1) % vertices.length]; return sum + p.x * q.y - q.x * p.y; }, 0);
  const visible = vertices.map((p, i) => {
    const q = vertices[(i + 1) % vertices.length];
    return Math.sign(area) * ((q.y - p.y) * direction.x - (q.x - p.x) * direction.y) > 0;
  });
  const start = visible.findIndex((v, i) => v && !visible[(i + visible.length - 1) % visible.length]);
  if (start < 0) return [];
  const arc = [vertices[start]];
  for (let offset = 0; offset < vertices.length && visible[(start + offset) % vertices.length]; offset++) {
    arc.push(vertices[(start + offset + 1) % vertices.length]);
  }
  return arc;
}

export function PracticalMap({ position, destination, isochrones, route, startedAt }: { position: Point; destination: Point; isochrones: Isochrone[]; route: Point[]; startedAt?: string }) {
  const root = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!root.current) return;
    let disposed = false;
    let map: import("maplibre-gl").Map | undefined;

    async function initializeMap() {
      const maplibregl = await import("maplibre-gl");
      await import("maplibre-gl/dist/maplibre-gl.css");
      if (!root.current || disposed) return;

      const mapInstance = new maplibregl.Map({
        container: root.current,
        style: {
          version: 8,
          sources: {
            openstreetmap: {
              type: "raster",
              tiles: ["https://tile.openstreetmap.org/{z}/{x}/{y}.png"],
              tileSize: 256,
              attribution: "© OpenStreetMap contributors",
            },
          },
          layers: [{ id: "openstreetmap", type: "raster", source: "openstreetmap" }],
        },
        center: [position.longitude, position.latitude],
        zoom: 4,
      });
      map = mapInstance;
      mapInstance.on("load", () => {
      const bounds = [position, destination, ...route, ...isochrones.flatMap((isochrone) => isochrone.positions)].reduce(
        (value, point) => value.extend([point.longitude, point.latitude]),
        new maplibregl.LngLatBounds([position.longitude, position.latitude], [position.longitude, position.latitude]),
      );
      if (route.length > 1) {
        mapInstance.addSource("best-route", {
          type: "geojson",
          data: {
            type: "Feature", properties: {}, geometry: { type: "LineString", coordinates: route.map((point) => [point.longitude, point.latitude]) },
          },
        });
        mapInstance.addLayer({
          id: "best-route",
          type: "line",
          source: "best-route", paint: { "line-color": "#f59f00", "line-width": 5, "line-opacity": 0.9 },
        });
        route.forEach((point, index) => new maplibregl.Marker({ color: "#f59f00", scale: index === 0 ? 0.7 : 0.85 }).setLngLat([point.longitude, point.latitude]).setPopup(new maplibregl.Popup().setText(`Route optimale — étape ${index}`)).addTo(mapInstance));
      }
      const overlay = document.createElementNS("http://www.w3.org/2000/svg", "svg");
      overlay.setAttribute("aria-label", "Contours des isochrones");
      overlay.style.cssText = "position:absolute;inset:0;pointer-events:none;z-index:2";
      mapInstance.getCanvasContainer().appendChild(overlay);
      const drawIsochrones = () => {
        const { clientWidth: width, clientHeight: height } = mapInstance.getContainer();
        overlay.setAttribute("viewBox", `0 0 ${width} ${height}`);
        overlay.setAttribute("width", String(width));
        overlay.setAttribute("height", String(height));
        overlay.replaceChildren();
        const labelBoxes: { x: number; y: number }[] = [];
        isochrones.forEach((isochrone) => {
          const minutes = startedAt ? Math.round((Date.parse(isochrone.instant) - Date.parse(startedAt)) / 60000) : NaN;
          const major = Number.isFinite(minutes) && minutes % 30 === 0;
          if (!isochrone.contour || isochrone.contour.length < 4) return;
          const origin = mapInstance.project([position.longitude, position.latitude]);
          const target = mapInstance.project([destination.longitude, destination.latitude]);
          const points = forwardArc(isochrone.contour.map((point) => mapInstance.project([point.longitude, point.latitude])), { x: target.x - origin.x, y: target.y - origin.y });
          if (points.length < 2) return;
          const path = document.createElementNS("http://www.w3.org/2000/svg", "path");
          path.setAttribute("d", roundedContour(points));
          path.setAttribute("fill", "none");
          path.setAttribute("stroke", major ? "#145cff" : "#72adff");
          path.setAttribute("stroke-width", major ? "1.4" : "0.8");
          path.setAttribute("stroke-opacity", major ? "0.95" : "0.75");
          path.setAttribute("stroke-linejoin", "round");
          overlay.appendChild(path);
          const anchor = [...points].sort((a, b) => a.y - b.y).find(p => p.x > 45 && p.x < width - 45 && p.y > 20 && p.y < height - 20 && labelBoxes.every(b => Math.abs(b.x - p.x) > 85 || Math.abs(b.y - p.y) > 25));
          if (anchor && major) {
            labelBoxes.push(anchor);
            const label = document.createElementNS("http://www.w3.org/2000/svg", "text");
            label.textContent = Number.isFinite(minutes) ? `+${minutes} min` : new Date(isochrone.instant).toLocaleTimeString("fr-FR", { hour: "2-digit", minute: "2-digit" });
            label.setAttribute("x", String(anchor.x));
            label.setAttribute("y", String(anchor.y - 7));
            label.setAttribute("text-anchor", "middle");
            label.setAttribute("style", "font:500 12px system-ui;fill:#145cff;stroke:#fff;stroke-width:3px;paint-order:stroke;stroke-linejoin:round");
            overlay.appendChild(label);
          }
        });
        if (route.length > 1) {
          const routePoints = route.map((point) => mapInstance.project([point.longitude, point.latitude]));
          const routeLine = document.createElementNS("http://www.w3.org/2000/svg", "polyline");
          routeLine.setAttribute("points", routePoints.map((point) => `${point.x},${point.y}`).join(" "));
          routeLine.setAttribute("fill", "none");
          routeLine.setAttribute("stroke", "#f59f00");
          routeLine.setAttribute("stroke-width", "4");
          routeLine.setAttribute("stroke-linecap", "round");
          routeLine.setAttribute("stroke-linejoin", "round");
          overlay.appendChild(routeLine);
        }
      };
      mapInstance.on("move", drawIsochrones);
      mapInstance.on("resize", drawIsochrones);
      drawIsochrones();
      new maplibregl.Marker({ color: "#1971c2" }).setLngLat([position.longitude, position.latitude]).setPopup(new maplibregl.Popup().setText("Départ")).addTo(mapInstance);
      new maplibregl.Marker({ color: "#6741d9" }).setLngLat([destination.longitude, destination.latitude]).setPopup(new maplibregl.Popup().setText("Destination")).addTo(mapInstance);
      mapInstance.fitBounds(bounds, { padding: 50, maxZoom: 10 });
      });
    }

    void initializeMap();
    return () => {
      disposed = true;
      map?.remove();
    };
  }, [position, destination, isochrones, route, startedAt]);
  return <section className="map-section"><h2>Carte des caps pratiques</h2><p>{isochrones.length} front{isochrones.length > 1 ? "s" : ""} d’isochrone calculé{isochrones.length > 1 ? "s" : ""}</p><div className="map" ref={root} /></section>;
}

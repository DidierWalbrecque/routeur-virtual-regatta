import { useEffect, useRef } from "react";
import * as maplibregl from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";

type Point = { latitude: number; longitude: number };
type Isochrone = { instant: string; positions: Point[] };

export function PracticalMap({ position, destination, isochrones, route }: { position: Point; destination: Point; isochrones: Isochrone[]; route: Point[] }) {
  const root = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!root.current) return;
    const map = new maplibregl.Map({
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
    map.on("load", () => {
      const bounds = [position, destination, ...route, ...isochrones.flatMap((isochrone) => isochrone.positions)].reduce(
        (value, point) => value.extend([point.longitude, point.latitude]),
        new maplibregl.LngLatBounds([position.longitude, position.latitude], [position.longitude, position.latitude]),
      );
      if (route.length > 1) {
        map.addSource("best-route", {
          type: "geojson",
          data: {
            type: "Feature", properties: {}, geometry: { type: "LineString", coordinates: route.map((point) => [point.longitude, point.latitude]) },
          },
        });
        map.addLayer({
          id: "best-route",
          type: "line",
          source: "best-route", paint: { "line-color": "#f59f00", "line-width": 5, "line-opacity": 0.9 },
        });
        route.forEach((point, index) => new maplibregl.Marker({ color: "#f59f00", scale: index === 0 ? 0.7 : 0.85 }).setLngLat([point.longitude, point.latitude]).setPopup(new maplibregl.Popup().setText(`Route optimale — étape ${index}`)).addTo(map));
      }
      const overlay = document.createElementNS("http://www.w3.org/2000/svg", "svg");
      overlay.setAttribute("aria-label", "Contours des isochrones");
      overlay.style.cssText = "position:absolute;inset:0;pointer-events:none;z-index:2";
      map.getCanvasContainer().appendChild(overlay);
      const drawIsochrones = () => {
        const { clientWidth: width, clientHeight: height } = map.getContainer();
        overlay.setAttribute("viewBox", `0 0 ${width} ${height}`);
        overlay.replaceChildren();
        if (route.length > 1) {
          const routePoints = route.map((point) => map.project([point.longitude, point.latitude]));
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
      map.on("move", drawIsochrones);
      map.on("resize", drawIsochrones);
      drawIsochrones();
      new maplibregl.Marker({ color: "#1971c2" }).setLngLat([position.longitude, position.latitude]).setPopup(new maplibregl.Popup().setText("Départ")).addTo(map);
      new maplibregl.Marker({ color: "#6741d9" }).setLngLat([destination.longitude, destination.latitude]).setPopup(new maplibregl.Popup().setText("Destination")).addTo(map);
      map.fitBounds(bounds, { padding: 50, maxZoom: 10 });
    });
    return () => map.remove();
  }, [position, destination, isochrones, route]);
  return <section className="map-section"><h2>Carte des caps pratiques</h2><p>{isochrones.length} front{isochrones.length > 1 ? "s" : ""} d’isochrone calculé{isochrones.length > 1 ? "s" : ""}</p><div className="map" ref={root} /></section>;
}

import { useState, type FormEvent } from "react";
import { createRoot } from "react-dom/client";
import "./style.css";
import { PracticalMap } from "./PracticalMap";

type Candidate = { heading: number; progressNM: number; remainingDistanceNM: number; stabilityScore: number; worstCaseProgressNM: number; scenarioProgressLossNM: number; robust: boolean; nextPosition: { latitude: number; longitude: number }; nextInstant: string; sail: string; speedKnots: number };
type Point = { latitude: number; longitude: number };
type Isochrone = { instant: string; positions: Point[]; contour: Point[] };
type RoutePoint = { position: Point; heading: number; sail: string; instant: string };
type Result = { position: Point; destination: Point; candidates: Candidate[]; isochrones: Isochrone[]; route: RoutePoint[] };
const equipment = ["light", "heavy", "code0", "foils", "hull_polish", "pro_winches"];

function formatLocalDate(value: Date) {
  return new Intl.DateTimeFormat("fr-FR", {
    day: "2-digit", month: "2-digit", year: "numeric", hour: "2-digit", minute: "2-digit", hour12: false,
  }).format(value).replace(",", "");
}

function parseLocalDate(value: string) {
  const match = value.trim().match(/^(\d{2})\/(\d{2})\/(\d{4})\s+(\d{2}):(\d{2})$/);
  if (!match) throw new Error("Date invalide : utilisez JJ/MM/AAAA HH:MM.");
  const [, day, month, year, hour, minute] = match;
  const date = new Date(Number(year), Number(month) - 1, Number(day), Number(hour), Number(minute));
  if (Number.isNaN(date.getTime())) throw new Error("Date invalide : utilisez JJ/MM/AAAA HH:MM.");
  return date.toISOString();
}

function parseVirtualRegattaPosition(value: string): Point | null {
  const match = value.trim().match(/^(\d{1,2})°\s*([NS])\s*(\d{1,2})['’]\s*(\d{1,2}(?:\.\d+)?)\"\s*-\s*(\d{1,3})°\s*([EW])\s*(\d{1,2})['’]\s*(\d{1,2}(?:\.\d+)?)\"$/i);
  if (!match) return null;
  const [, latitudeDegrees, latitudeHemisphere, latitudeMinutes, latitudeSeconds, longitudeDegrees, longitudeHemisphere, longitudeMinutes, longitudeSeconds] = match;
  const latitude = Number(latitudeDegrees) + Number(latitudeMinutes) / 60 + Number(latitudeSeconds) / 3600;
  const longitude = Number(longitudeDegrees) + Number(longitudeMinutes) / 60 + Number(longitudeSeconds) / 3600;
  return {
    latitude: latitudeHemisphere.toUpperCase() === "S" ? -latitude : latitude,
    longitude: longitudeHemisphere.toUpperCase() === "W" ? -longitude : longitude,
  };
}

function formatVirtualRegattaPosition(point: Point) {
  const format = (value: number, positive: string, negative: string) => {
    const absolute = Math.abs(value); const degrees = Math.floor(absolute); const totalSeconds = Math.round((absolute - degrees) * 3600); const minutes = Math.floor(totalSeconds / 60); const seconds = totalSeconds % 60;
    return `${degrees}°${value >= 0 ? positive : negative} ${String(minutes).padStart(2, "0")}'${String(seconds).padStart(2, "0")}"`;
  };
  return `${format(point.latitude, "N", "S")} - ${format(point.longitude, "E", "W")}`;
}

function App() {
  const [weatherFiles, setWeatherFiles] = useState("C:\\Users\\dwalb\\Documents\\dev\\routeur-virtual-regatta\\data\\weather\\gfs.2026090506.f000.grb2\nC:\\Users\\dwalb\\Documents\\dev\\routeur-virtual-regatta\\data\\weather\\gfs.2026090506.f003.grb2\nC:\\Users\\dwalb\\Documents\\dev\\routeur-virtual-regatta\\data\\weather\\gfs.2026090506.f006.grb2");
  const [polarPath, setPolarPath] = useState("../data/polars/ultim_bp_11.json");
  const [start, setStart] = useState(`13°N 07'16" - 121°E 35'12"`);
  const [destination, setDestination] = useState(`13°N 07'16" - 121°E 45'12"`);
  const [instant, setInstant] = useState("05/09/2026 09:00");
  const [headings, setHeadings] = useState("0, 15, 30, 45, 60, 75, 90, 105, 120, 135, 150, 165, 180, 195, 210, 225, 240, 255, 270, 285, 300, 315, 330, 345");
  const [owned, setOwned] = useState<string[]>([]);
  const [duration, setDuration] = useState("240");
  const [step, setStep] = useState("10");
  const [fixedSailID, setFixedSailID] = useState("0");
  const [maxLoss, setMaxLoss] = useState("0");
  const [robustOnly, setRobustOnly] = useState(false);
  const [candidates, setCandidates] = useState<Candidate[]>([]);
  const [route, setRoute] = useState<Result | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const mapPosition = route?.position ?? parseVirtualRegattaPosition(start);
  const mapDestination = route?.destination ?? parseVirtualRegattaPosition(destination);

  const toggleEquipment = (item: string) => setOwned((current) => current.includes(item) ? current.filter((value) => value !== item) : [...current, item]);
  async function submit(event: FormEvent) {
    event.preventDefault(); setLoading(true); setError(""); setCandidates([]); setRoute(null);
    let instantISO: string;
    try { instantISO = parseLocalDate(instant); }
    catch (reason) { setError(reason instanceof Error ? reason.message : "Date invalide"); setLoading(false); return; }
    const payload = {
      weatherFiles: weatherFiles.split(/\r?\n/).map((value) => value.trim()).filter(Boolean), polarPath,
      position: { virtualRegatta: start }, destination: { virtualRegatta: destination },
      instant: instantISO, durationMinutes: Number(duration), stepMinutes: Number(step),
      headings: headings.split(",").map((value) => Number(value.trim())).filter(Number.isFinite), owned,
      fixedSailID: Number(fixedSailID), maxScenarioProgressLossNM: Number(maxLoss), requireRobustScenarios: robustOnly,
    };
    try {
      const response = await fetch("/api/practical-headings", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(payload) });
      const body = await response.json();
      if (!response.ok) throw new Error(body.error ?? "Calcul refusé");
      setCandidates(body.candidates); setRoute(body);
    } catch (reason) { setError(reason instanceof Error ? reason.message : "Erreur réseau"); }
    finally { setLoading(false); }
  }
  return <main>
    <header><p className="eyebrow">Routeur Virtual Regatta</p><h1>Caps pratiques</h1><p>Comparez les caps fixes sur quatre heures, avec la météo et les équipements réellement disponibles.</p></header>
    <form onSubmit={submit}>
      <section><h2>Données de calcul</h2><label>Fichiers GRIB <textarea required value={weatherFiles} onChange={(event) => setWeatherFiles(event.target.value)} placeholder="Un chemin par ligne" /></label><label>Polaire <input required value={polarPath} onChange={(event) => setPolarPath(event.target.value)} /></label><label>Position actuelle Virtual Regatta <input required value={start} onChange={(event) => setStart(event.target.value)} placeholder={'13°N 07\'16" - 121°E 35\'12"'} /></label><label>Destination Virtual Regatta <input required value={destination} onChange={(event) => setDestination(event.target.value)} placeholder={'16°N 20\'00" - 61°W 30\'00"'} /></label><label>Départ (heure locale, JJ/MM/AAAA HH:MM) <input required value={instant} onChange={(event) => setInstant(event.target.value)} placeholder="29/04/2026 11:00" /></label></section>
      <section><h2>Stratégie pratique</h2><div className="grid"><label>Durée (min) <input type="number" min="1" value={duration} onChange={(event) => setDuration(event.target.value)} /></label><label>Pas (min) <input type="number" min="1" value={step} onChange={(event) => setStep(event.target.value)} /></label><label>Voile fixe (0 = auto) <input type="number" min="0" value={fixedSailID} onChange={(event) => setFixedSailID(event.target.value)} /></label><label>Perte max. (NM) <input type="number" min="0" step="any" value={maxLoss} onChange={(event) => setMaxLoss(event.target.value)} /></label></div><label>Caps (degrés, séparés par des virgules) <input required value={headings} onChange={(event) => setHeadings(event.target.value)} /></label><div className="checks">{equipment.map((item) => <label key={item}><input type="checkbox" checked={owned.includes(item)} onChange={() => toggleEquipment(item)} /> {item}</label>)}</div><label className="checks"><input type="checkbox" checked={robustOnly} onChange={(event) => setRobustOnly(event.target.checked)} /> Ne conserver que les caps robustes</label></section>
      <button disabled={loading}>{loading ? "Calcul en cours…" : "Calculer les caps"}</button>{error && <p className="error">{error}</p>}
    </form>
    {mapPosition && mapDestination ? <PracticalMap position={mapPosition} destination={mapDestination} isochrones={route?.isochrones ?? []} route={route?.route.map((point) => point.position) ?? []} startedAt={route?.route[0]?.instant} /> : <p className="map-hint">Renseignez le départ et la destination au format Virtual Regatta pour afficher la carte.</p>}
    {candidates.length > 0 && <section className="results"><h2>Résultats classés</h2><div className="table-wrap"><table><thead><tr><th>Cap</th><th>Progression</th><th>Voile</th><th>Vitesse</th><th>Stabilité</th><th>Pire scénario</th><th>Robuste</th></tr></thead><tbody>{candidates.map((candidate) => <tr key={candidate.heading}><td>{candidate.heading.toFixed(0)}°</td><td>{candidate.progressNM.toFixed(2)} NM</td><td>{candidate.sail}</td><td>{candidate.speedKnots.toFixed(2)} nd</td><td>{(candidate.stabilityScore * 100).toFixed(0)} %</td><td>{candidate.worstCaseProgressNM.toFixed(2)} NM <small>(−{candidate.scenarioProgressLossNM.toFixed(2)})</small></td><td>{candidate.robust ? "Oui" : "Non"}</td></tr>)}</tbody></table></div></section>}
    {route && route.route.length > 0 && <section className="results"><h2>Points de la route optimale</h2><div className="table-wrap"><table><thead><tr><th>Étape</th><th>Coordonnées Virtual Regatta</th><th>Cap</th><th>Voile</th></tr></thead><tbody>{route.route.map((point, index) => <tr key={`${point.position.latitude}-${point.position.longitude}`}><td>{index === 0 ? "Départ" : `Point ${index}`}</td><td>{formatVirtualRegattaPosition(point.position)}</td><td>{index === 0 ? "—" : `${point.heading.toFixed(0)}°`}</td><td>{index === 0 ? "—" : point.sail}</td></tr>)}</tbody></table></div></section>}
  </main>;
}
createRoot(document.getElementById("root")!).render(<App />);

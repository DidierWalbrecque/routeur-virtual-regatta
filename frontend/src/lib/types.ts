import type { WeatherSwitch } from "./weather_switch.ts";

export type Point = { latitude: number; longitude: number };

export type RoutePoint = {
  position: Point;
  heading?: number;
  sail?: string;
  stamina?: number;
  instant: string;
  terminal?: boolean;
  twd?: number;
  tws?: number;
  twa?: number;
  tack?: string;
  speedKnots?: number;
  foilPercent?: number;
  speedCoefficient?: number;
  maneuver?: string;
};

export type Isochrone = {
  instant: string;
  positions?: Point[];
  contour: Point[];
  completedLayers?: number;
  totalLayers?: number;
};

export type RouteResult = {
  /** Run récent encore partiel, complété par le précédent. */
  weather?: WeatherSwitch;
  position?: Point;
  destination?: Point;
  destinationReached?: boolean;
  route: RoutePoint[];
  isochrones?: Isochrone[];
};

export type RouteStart = {
  position?: Point;
  destination?: Point;
  startedAt?: string;
  totalLayers?: number;
  weather?: WeatherSwitch;
};

export type Checkpoint = {
  group?: number;
  id?: number;
  name?: string;
  display?: string;
  side?: string;
  rhumb?: boolean;
  onLand?: boolean;
  start: Point;
  end: Point;
};

export type BoatHint = { path: string; value: string };

export type VrState = {
  position?: Point;
  arrival?: { latitude: number; longitude: number; radius?: number | null; name?: string } | null;
  legStart?: { latitude: number; longitude: number; name?: string } | null;
  state?: string;
  stamina?: number | null;
  options?: string[];
  checkpoints?: Checkpoint[];
  gateGroupCounters?: boolean[];
  updatedAt?: number;
  boatHints?: BoatHint[];
  /** Zones interdites de la course, reçues du jeu. */
  zones?: { name: string; points: Point[] }[];
  /** Structure de la manche reçue, pour retrouver le champ des zones. */
  legShape?: BoatHint[];
};

export type PolarOption = { label: string; value: string; id?: number; boat?: string };

import { formatVirtualRegattaPosition } from "./geo.ts";
import type { Checkpoint, VrState } from "./types.ts";

export const EQUIPMENT = ["light", "heavy", "code0", "foils", "hull_polish", "pro_winches"] as const;
export type Equipment = (typeof EQUIPMENT)[number];

const OPTION_ALIASES: Record<Equipment, string[]> = {
  light: ["light"],
  heavy: ["heavy"],
  code0: ["code0", "codezero", "reach"],
  foils: ["foil", "foils"],
  hull_polish: ["hull", "hullpolish"],
  pro_winches: ["winch", "winches", "prowinches"],
};

// Déduit les équipements possédés des options reçues de Virtual Regatta.
export function equipmentFromVrOptions(options: string[] | undefined): Set<Equipment> {
  const normalized = new Set((options || []).map((option) => String(option).toLowerCase().replace(/[_ -]/g, "")));
  return new Set(EQUIPMENT.filter((item) => OPTION_ALIASES[item].some((alias) => normalized.has(alias))));
}

export function isCheckpointPassed(state: VrState | undefined, checkpoint: Checkpoint): boolean {
  const group = Number(checkpoint?.group);
  return Number.isInteger(group) && group > 0 && state?.gateGroupCounters?.[group - 1] === true;
}

export function isCheckpointUsable(state: VrState | undefined, checkpoint: Checkpoint): boolean {
  return !isCheckpointPassed(state, checkpoint) && checkpoint?.onLand !== true;
}

export function usableCheckpoints(state: VrState | undefined): Checkpoint[] {
  return (state?.checkpoints || []).filter((checkpoint) => isCheckpointUsable(state, checkpoint));
}

/**
 * Porte à dessiner sur la carte : le jeu nomme « none » les portes qu'il ne
 * montre pas. Elles restent imposées au calcul de route, seul l'affichage les
 * écarte.
 */
export function isCheckpointShown(checkpoint: Checkpoint): boolean {
  return String(checkpoint?.name ?? "").trim().toLowerCase() !== "none";
}

export function checkpointKey(checkpoint: Checkpoint, index: number): string {
  return `${checkpoint.group ?? index + 1}.${checkpoint.id ?? index + 1}`;
}

// Valeurs que le jeu envoie à la place d'un vrai nom.
// « buoy » (bouée) n'est qu'un type de marque, pas un nom : la porte est alors
// désignée par son sens de passage, comme une porte sans nom.
const PLACEHOLDER_NAMES = new Set(["", "none", "null", "undefined", "-", "—", "buoy", "bouée", "bouee"]);

/** Nom lisible, ou null quand le jeu n'envoie qu'un nom vide, « none » ou « buoy ». */
export function meaningfulName(value: string | null | undefined): string | null {
  const name = String(value ?? "").trim();
  return PLACEHOLDER_NAMES.has(name.toLowerCase()) ? null : name;
}

export function checkpointSideLabel(checkpoint: Checkpoint): string {
  if (checkpoint.rhumb) return "dans les deux sens";
  const side = String(checkpoint.side || "").toLowerCase();
  return side === "stbd" ? "tribord" : side === "port" ? "bâbord" : "sens libre";
}

/** Nom de la porte, ou son sens de passage quand le jeu ne la nomme pas. */
export function checkpointName(checkpoint: Checkpoint): string {
  return meaningfulName(checkpoint.name) ?? meaningfulName(checkpoint.display) ?? checkpointSideLabel(checkpoint);
}

export function checkpointLabel(checkpoint: Checkpoint, index: number): string {
  return `P${checkpointKey(checkpoint, index)} — ${checkpointName(checkpoint)}`;
}

export function checkpointTitle(checkpoint: Checkpoint): string {
  return `Départ : ${formatVirtualRegattaPosition(checkpoint.start)}\nArrivée : ${formatVirtualRegattaPosition(checkpoint.end)}`;
}

export type MapFeature = {
  type: "Feature";
  properties: Record<string, unknown>;
  geometry: { type: "LineString"; coordinates: number[][] } | { type: "Point"; coordinates: number[] };
};

/**
 * Portes encore à passer, pour la carte : un point par porte, sur sa marque de
 * départ, avec son nom, son sens de passage et son numéro d'ordre de passage
 * (1 = prochaine porte). Les noms ne sont placés qu'à côté de la marque de
 * départ ; les repères des deux marques viennent de gateMarks.
 */
export function checkpointFeatures(state: VrState | undefined): MapFeature[] {
  return usableCheckpoints(state).flatMap((checkpoint, index): MapFeature[] => {
    if (!isCheckpointShown(checkpoint)) return [];
    const { start } = checkpoint;
    if (!start || !Number.isFinite(Number(start.latitude)) || !Number.isFinite(Number(start.longitude))) return [];
    const group = checkpoint.group ?? index + 1;
    const id = checkpoint.id ?? index + 1;
    const name = checkpointName(checkpoint);
    return [
      {
        type: "Feature",
        properties: {
          kind: "label",
          checkpoint: `${group}.${id}`,
          label: `P${group}.${id} — ${name}`,
          name,
          side: checkpoint.rhumb ? "any" : String(checkpoint.side || "").toLowerCase(),
          sideLabel: checkpointSideLabel(checkpoint),
          rhumb: checkpoint.rhumb === true,
          order: index + 1,
        },
        geometry: { type: "Point", coordinates: [Number(start.longitude), Number(start.latitude)] },
      },
    ];
  });
}

/** Marque d'une porte à dessiner sur la carte. */
export type GateMark = {
  /** Identifiant de la porte (groupe.id). */
  checkpoint: string;
  /** Numéro d'ordre de passage (1 = prochaine porte). */
  order: number;
  name: string;
  /** Marque d'arrivée (seconde extrémité) plutôt que de départ. */
  end: boolean;
  side: string;
  rhumb: boolean;
  position: { latitude: number; longitude: number };
};

function validMark(point: { latitude: number; longitude: number } | undefined): { latitude: number; longitude: number } | null {
  if (!point) return null;
  const latitude = Number(point.latitude);
  const longitude = Number(point.longitude);
  return Number.isFinite(latitude) && Number.isFinite(longitude) ? { latitude, longitude } : null;
}

/**
 * Marques des portes encore à passer : la marque de départ puis la marque
 * d'arrivée de chaque porte, numérotées dans l'ordre de passage comme
 * checkpointFeatures. Une porte sans marque de départ valide est ignorée ; une
 * marque d'arrivée confondue avec celle de départ n'est pas répétée.
 */
export function gateMarks(state: VrState | undefined): GateMark[] {
  return usableCheckpoints(state).flatMap((checkpoint, index): GateMark[] => {
    if (!isCheckpointShown(checkpoint)) return [];
    const start = validMark(checkpoint.start);
    if (!start) return [];
    const base = {
      checkpoint: `${checkpoint.group ?? index + 1}.${checkpoint.id ?? index + 1}`,
      order: index + 1,
      name: checkpointName(checkpoint),
      side: checkpoint.rhumb ? "any" : String(checkpoint.side || "").toLowerCase(),
      rhumb: checkpoint.rhumb === true,
    };
    const marks: GateMark[] = [{ ...base, end: false, position: start }];
    const end = validMark(checkpoint.end);
    if (end && (end.latitude !== start.latitude || end.longitude !== start.longitude)) {
      marks.push({ ...base, end: true, position: end });
    }
    return marks;
  });
}

export const EQUIPMENT_LABELS: Record<Equipment, string> = {
  light: "Light",
  heavy: "Heavy",
  code0: "Code 0",
  foils: "Foils",
  hull_polish: "Carène polie",
  pro_winches: "Winchs pro",
};

function formatGameDate(value: number | null | undefined): string | null {
  if (value == null || !Number.isFinite(value)) return null;
  return new Date(value).toLocaleString("fr-FR", { day: "2-digit", month: "2-digit", year: "numeric", hour: "2-digit", minute: "2-digit" });
}

/** Texte de statut de la connexion Virtual Regatta. */
export function vrStatusText(state: VrState): string {
  const remaining = usableCheckpoints(state).length;
  const gates = `${remaining} porte(s) restante(s)`;
  if (state.preStart) {
    const close = formatGameDate(state.startWindow?.close);
    const windowText = close ? ` · fermeture le ${close}` : "";
    return `Record, départ non pris${windowText} · ${gates}`;
  }
  const stamina = state.stamina == null ? "inconnue" : `${Number(state.stamina).toFixed(1)} %`;
  const record = state.raceType === "record" ? "Record en cours · " : "";
  return `${record}Virtual Regatta connecté · stamina ${stamina} · ${gates}`;
}

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
 * (1 = prochaine porte). La seconde extrémité, souvent un point lointain qui ne
 * sert qu'à prolonger la ligne, n'est pas dessinée.
 */
export function checkpointFeatures(state: VrState | undefined): MapFeature[] {
  return usableCheckpoints(state).flatMap((checkpoint, index): MapFeature[] => {
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

export const EQUIPMENT_LABELS: Record<Equipment, string> = {
  light: "Light",
  heavy: "Heavy",
  code0: "Code 0",
  foils: "Foils",
  hull_polish: "Carène polie",
  pro_winches: "Winchs pro",
};

import type { BoatHint, PolarOption } from "./types.ts";

export type PolarMatch = { polar: PolarOption; hint: BoatHint };

function normalize(value: string): string {
  return value
    .toLowerCase()
    .replace(/^mono\//, "")
    .replace(/\.json$/, "")
    .replace(/[^a-z0-9]/g, "");
}

function namesMatch(candidate: string, wanted: string): boolean {
  if (!candidate || !wanted) return false;
  if (candidate === wanted) return true;
  if (wanted.length >= 5 && candidate.includes(wanted)) return true;
  return candidate.length >= 6 && wanted.includes(candidate);
}

// Trouve la polaire du bateau à partir des indices reçus de Virtual Regatta :
// d'abord un identifiant numérique de polaire, sinon un nom de bateau.
export function matchPolar(polars: PolarOption[], hints: BoatHint[] | undefined): PolarMatch | null {
  if (!hints?.length || !polars.length) return null;

  for (const hint of hints) {
    if (!/polar/i.test(hint.path)) continue;
    const id = Number(hint.value);
    if (!Number.isInteger(id)) continue;
    const polar = polars.find((candidate) => candidate.id === id);
    if (polar) return { polar, hint };
  }

  for (const hint of hints) {
    const wanted = normalize(hint.value);
    if (wanted.length < 4 || /^\d+$/.test(wanted)) continue;
    const polar = polars.find((candidate) =>
      [candidate.boat, candidate.label, candidate.value.split("/").pop()].some((name) => namesMatch(normalize(name || ""), wanted)),
    );
    if (polar) return { polar, hint };
  }
  return null;
}

/**
 * Repère d'une porte : un anneau autour de sa marque de départ, avec le numéro
 * d'ordre de passage au centre.
 *
 * Une porte « tribord » se franchit de la gauche vers la droite de la ligne
 * orientée marque de départ → seconde marque : le bateau laisse la marque à
 * tribord et tourne autour d'elle dans le sens des aiguilles d'une montre.
 * Une porte « bâbord » se contourne dans l'autre sens. Une porte sans sens
 * imposé a un anneau pointillé, immobile.
 */
export type GateRounding = "clockwise" | "counterclockwise" | "free";

export function gateRounding(side: string | undefined, rhumb: boolean | undefined): GateRounding {
  if (rhumb) return "free";
  const value = String(side || "").toLowerCase();
  if (value === "stbd" || value === "starboard") return "clockwise";
  if (value === "port") return "counterclockwise";
  return "free";
}

export function gateRoundingLabel(rounding: GateRounding): string {
  if (rounding === "clockwise") return "marque à laisser à tribord, à contourner dans le sens horaire";
  if (rounding === "counterclockwise") return "marque à laisser à bâbord, à contourner dans le sens antihoraire";
  return "porte à franchir dans les deux sens";
}

/**
 * Anneau en SVG (cadre de 40 × 40) : un arc de 270° terminé par une pointe de
 * flèche, dessiné dans le sens horaire puis retourné pour le sens antihoraire.
 */
export function gateRingSvg(rounding: GateRounding, color = "#ffffff"): string {
  if (rounding === "free") {
    return `<svg viewBox="0 0 40 40" aria-hidden="true"><circle cx="20" cy="20" r="16" fill="none" stroke="${color}" stroke-width="2.5" stroke-dasharray="4 4" stroke-opacity="0.8"/></svg>`;
  }
  const mirror = rounding === "counterclockwise" ? ' transform="translate(40 0) scale(-1 1)"' : "";
  return `<svg viewBox="0 0 40 40" aria-hidden="true"><g${mirror}>
    <path d="M 6.14 12 A 16 16 0 1 1 12 33.86" fill="none" stroke="${color}" stroke-width="3" stroke-linecap="round"/>
    <path d="M 7.67 31.36 L 11.55 37.64 L 15.05 31.58 Z" fill="${color}"/>
  </g></svg>`;
}

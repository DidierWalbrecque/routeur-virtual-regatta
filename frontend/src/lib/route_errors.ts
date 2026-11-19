/** Contexte du calcul utile pour expliquer une erreur du moteur. */
export type RouteErrorContext = { coastalMarginNM: number; enabledZoneCount: number };

// Le serveur préfixe ses erreurs par l'étape qui a échoué : inutile pour l'utilisateur.
const TECHNICAL_PREFIXES = [/^calcul des isochrones impossible\s*:\s*/i, /^routing\s*:\s*/i];

function blockers(context: RouteErrorContext): string {
  const parts = ["la côte"];
  if (context.coastalMarginNM > 0) parts.push(`sa marge de ${String(context.coastalMarginNM).replace(".", ",")} NM`);
  if (context.enabledZoneCount > 0) parts.push("une zone interdite");
  return parts.length === 1 ? parts[0] : `${parts.slice(0, -1).join(", ")} ou ${parts.at(-1)}`;
}

function remedies(context: RouteErrorContext): string {
  const parts: string[] = [];
  if (context.coastalMarginNM > 0) parts.push("réduisez la marge à la côte (Réglages avancés)");
  if (context.enabledZoneCount > 0) parts.push("décochez la zone voisine dans « Zones interdites »");
  parts.push("déplacez le départ");
  return parts.length === 1 ? parts[0] : `${parts.slice(0, -1).join(", ")} ou ${parts.at(-1)}`;
}

/** Traduit une erreur du moteur de routage en phrase compréhensible, avec la marche à suivre. */
export function friendlyRouteError(message: string, context: RouteErrorContext): string {
  let text = message.trim();
  for (const prefix of TECHNICAL_PREFIXES) text = text.replace(prefix, "");

  const blocked = /aucune trajectoire autorisée à la couche (\d+)/i.exec(text);
  if (blocked) {
    const layer = Number(blocked[1]);
    if (layer === 1) {
      return `Aucun cap possible au départ : toutes les directions sont bloquées par ${blockers(context)}. Pour calculer, ${remedies(context)}.`;
    }
    return `Le calcul s'arrête après ${layer - 1} pas : le bateau est bloqué par ${blockers(context)}. Pour continuer, ${remedies(context)}.`;
  }
  const inZone = /le point de (départ|destination) se trouve dans une zone interdite/i.exec(text);
  if (inZone) {
    return `${inZone[1] === "départ" ? "Le départ" : "La destination"} est dans une zone interdite : décochez cette zone dans « Zones interdites » ou déplacez le point.`;
  }
  const onLand = /le point de (départ|destination) se trouve sur une terre ou la côte/i.exec(text);
  if (onLand) {
    return `${onLand[1] === "départ" ? "Le départ" : "La destination"} est sur la terre : vérifiez la position saisie.`;
  }
  return text ? text.charAt(0).toUpperCase() + text.slice(1) : "Le calcul a échoué.";
}

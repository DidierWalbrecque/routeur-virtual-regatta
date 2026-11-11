# Améliorations urgentes du calcul de route

## Analyse directe

Le routeur utilise une recherche par isochrones : à chaque pas de temps, il simule une grille de caps pour chaque position conservée, puis réduit les candidats par cellules spatiales et garde au plus `MaxCandidates`.

La structure est claire et déjà équipée de garde-fous utiles : annulation par contexte, contraintes de trajet, prise en compte des portes, amures, voiles, pénalités et stamina. Les principales limites sont dans la qualité de l’exploration et dans la façon de réduire les trajectoires.

## Améliorations prioritaires

### 1. Éviter de supprimer trop tôt une bonne trajectoire

Dans `backend/internal/routing/multi_layer_isochrone.go`, l’élagage regroupe les états par cellule, cap, voile, amure, pénalité et prochaine porte. Deux positions proches peuvent donc être fusionnées alors que l’une est mieux placée pour le vent ou les obstacles à l’étape suivante. La limite dure peut aussi finir par couper des états qui ne sont pas comparables.

**À améliorer :** conserver une petite diversité par cellule et état, et ne comparer que les états réellement équivalents, notamment en stamina et en progression vers la cible.

### 2. Rendre le classement plus pertinent pour la suite du parcours

Le tri favorise la progression vers la prochaine cible, puis vers la destination et la stamina. Il n’estime pas le temps restant ni la qualité météo attendue plus loin. Une route qui avance vite vers la porte immédiate peut ainsi éliminer une route plus longue à court terme, mais plus rapide au total.

**À améliorer :** ajouter une estimation du coût restant — par exemple un temps minimal estimé — au classement et à l’élagage. C’est l’amélioration qui peut le plus changer la qualité des routes proposées.

### 3. Adapter la grille de caps au lieu d’utiliser partout le même pas

`backend/internal/routing/heading_grid.go` génère une grille uniforme, et chaque état conservé réévalue tous ces caps. Un pas fin améliore la précision, mais augmente fortement le travail à chaque couche.

**À améliorer :** explorer grossièrement d’abord, puis raffiner autour des meilleurs caps, des changements d’amure et des bords de portes. Cela permettrait d’améliorer la précision sans multiplier uniformément les calculs.

### 4. Limiter les caps manifestement inutiles

La grille couvre toujours 360°, même lorsque certains caps mènent immédiatement à terre ou s’éloignent sans possibilité réaliste de revenir. Les contraintes de terrain éliminent ensuite ces trajectoires, après leur simulation.

**À améliorer :** filtrer les caps impossibles avant l’évaluation et ajouter un contrôle anticipé de collision avec les terres. Le gain dépendra de la part des routes proches des côtes ou des zones d’exclusion.

### 5. Mesurer les coûts avant d’optimiser davantage

`backend/internal/routing/parallel_heading_candidates.go` parallélise l’évaluation par parent. Le parallélisme est déterministe à la sortie, mais peut être contre-productif si le fournisseur météo est déjà rapide, saturé ou partagé. Le benchmark existant couvre un cas synthétique, sans comparaison des principaux postes de calcul.

**À améliorer :** mesurer séparément les appels météo, les calculs polaires, les contraintes et l’élagage avec des scénarios réalistes avant de modifier la concurrence ou d’ajouter du cache.

## À vérifier côté modèle

Le calcul simule chaque pas avec le vent au départ du segment, puis avance le bateau à vitesse constante jusqu’au pas suivant. Sa précision dépend donc directement de la durée du pas et des variations météo dans le temps et l’espace. Il faut aussi vérifier que les règles de pénalité, d’équipement et de stamina reflètent bien celles du jeu : une recherche plus fine ne compensera pas un modèle physique ou une règle de jeu inexacte.

## Ordre conseillé

1. Améliorer l’élagage et le classement.
2. Tester une grille de caps adaptative.
3. Mesurer les coûts sur des scénarios réalistes, puis optimiser les parties les plus coûteuses.

---

## Retour après lecture du code (12 novembre 2026)

Relu dans `backend/internal/routing` et `backend/internal/httpapi`. Une partie du constat ci-dessus ne correspond plus au code.

### Ce qui est déjà en place

- **Point 1, élagage :** il ne fusionne pas deux positions proches. `pruneIsochroneNodesUnsorted` regroupe par cellule (0,5 NM en mode précis, 1 NM en mode rapide), cap, voile, amure, pénalité active et prochaine porte. Dans chaque groupe, il garde tous les nœuds **non dominés** sur deux critères : progression vers la cible et stamina. Un nœud moins avancé mais plus reposé survit donc déjà. La diversité en stamina demandée existe.
- **Point 3, grille de caps :** elle n'est pas uniforme partout. `heading_refinement.go` ajoute des caps entiers autour des meilleurs caps de la grille (jusqu'à trois maxima par parent, en général le meilleur sur chaque amure). C'est actif pour les deux modes (`RefineHeadings: true`), sur une grille de 6° en mode précis et de 30° en mode rapide.
- **Point 5, appels météo :** ils ne sont pas répétés par cap. Tous les caps d'un même parent partent de la même position au même instant, et `headingWeatherCache` ne demande le vent qu'une fois par parent. Le coût par cap vient surtout de la polaire, des pénalités et des contraintes de trajet.
- **Mesure sur routes réelles :** `TestVRFrontierPruningRoutes` (commit `ae1c0ea`) existe déjà. Il donne, sur de vraies courses, les milles restants après 48 h, la taille des fronts et la durée du calcul. C'est l'outil pour valider toute modification de l'élagage ou du classement.

### Ce qui reste juste

- **Limite dure du point 1 :** quand il reste plus de `MaxCandidates` nœuds (2 400 en précis, 1 200 en rapide), `retainAngularSectors` garde **un seul nœud par secteur de cap vu depuis le départ**, choisi par `nodeBetter`. À ce stade, voile, amure, stamina et porte suivante ne protègent plus rien : c'est là que des états non comparables se coupent, pas dans le regroupement par cellule.
- **Point 2, classement :** il est confirmé. `nodeBetter` compare la porte franchie, puis la progression vers la cible, puis la distance, puis la stamina. Il n'y a aucune estimation du temps restant. Le coût des manœuvres n'entre qu'au choix final (`finalNodeBetter`, commit `dfa1d6f`).
- **Point 4, caps inutiles :** il est confirmé. Les contraintes (terre, zones, portes) sont vérifiées après la simulation de chaque cap. Aucun cap n'est écarté avant.
- **Modèle :** il est confirmé aussi. `simulation.EvaluateStep` prend le vent au point et à l'instant de départ du segment et le garde tout le pas (10 min en précis, 15 min en rapide).

### Ce que j'ajoute

1. **Les secteurs sont centrés sur le point de départ.** Plus la route s'éloigne du départ ou contourne une île ou une porte, moins un secteur vu du départ correspond à une direction du front. Derrière un obstacle, des nœuds en retard mais utiles partagent un secteur avec des nœuds mieux avancés et disparaissent. Centrer les secteurs sur le départ **du pas précédent**, ou sur la cible suivante, est une piste plus simple qu'une heuristique de temps restant.
2. **Une estimation du temps restant est risquée près des côtes.** Une estimation à vol d'oiseau pénalise les routes qui contournent une île, alors que ce sont souvent les seules possibles. Elle doit tenir compte des terres, ou n'entrer en jeu qu'au-delà d'une certaine distance. Sans cela, elle aggraverait le cas d'Antigua relevé le 11 novembre.
3. **Le contour affiché n'est pas le front réel.** `IsochroneFrontier.Contour` dessine une enveloppe convexe. Près d'une île, elle relie des points éloignés de plus de 150 NM et masque où le front est réellement. C'est un défaut d'affichage seulement (le calcul n'utilise pas ce contour), mais il empêche de juger visuellement l'élagage. Un contour qui suit les creux du front (par secteurs) aiderait à vérifier les points 1 et 2.
4. **Mesurer sur le PC du backend.** Les durées mesurées sur le poste de développement ne valent pas pour le serveur. Les mesures du point 5 doivent être faites là où le backend tourne.

### Ordre que je propose

1. **Mesurer d'abord.** Lancer `TestVRFrontierPruningRoutes` sur le PC du backend pour avoir une référence : milles restants à 48 h et durée.
2. **Revoir la limite dure.** Garder plusieurs nœuds par secteur quand ils diffèrent d'amure ou de porte suivante, et tester le centrage des secteurs. Comparer à la référence.
3. **Tester une estimation du temps restant**, limitée à la haute mer, seulement si l'étape 2 ne suffit pas.
4. **Affiner la grille et filtrer les caps** : le gain attendu est faible, puisque l'affinage existe déjà.

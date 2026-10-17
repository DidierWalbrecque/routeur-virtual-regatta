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

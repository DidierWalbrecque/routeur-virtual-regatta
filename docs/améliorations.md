# Améliorations prévues

## Accélérer le calcul des routes

Le calcul actuel peut devenir long car l'API évalue les caps pratiques sur
quatre heures, génère une recherche par isochrones indépendante et applique
les contraintes terrestres à de nombreux segments.

### Réglages immédiats

Pour une destination courte, par exemple environ 10 NM :

```text
Durée : 60 minutes
Pas : 10 minutes
Caps espacés de 30 degrés
Mode : caps pratiques uniquement
```

Exemple de caps :

```text
0,30,60,90,120,150,180,210,240,270,300,330
```

Pour une course longue, conserver un réglage plus précis :

```text
Durée : 240 minutes
Pas : 10 minutes
Caps espacés de 15 degrés
```

### Mode rapide et mode précis

Ajouter deux profils de calcul :

- **Rapide** : caps tous les 30 degrés, pas de 15 minutes, maximum 50
  candidats et horizon adapté à la distance ;
- **Précis** : caps tous les 15 degrés, pas de 10 minutes et maximum 100 à
  500 candidats selon la puissance disponible.

### Calcul adaptatif

Le routeur devra pouvoir :

- adapter l'horizon à la distance de la destination ;
- arrêter le calcul dès que la destination est atteinte ;
- commencer avec une grille de caps espacée ;
- affiner uniquement autour des meilleurs caps, par exemple 75, 90 et 105
  degrés après un premier passage à 30 degrés.

Cette approche doit réduire le temps de calcul sans supprimer les alternatives
pertinentes.

### Éviter les calculs en double

L'API calcule actuellement les caps pratiques puis une recherche par
isochrones indépendante. Ajouter un choix entre :

- caps pratiques uniquement ;
- route par isochrones uniquement ;
- les deux calculs.

Le mode « caps pratiques uniquement » doit être privilégié pour les tests
rapides et les destinations courtes.

### Mettre les données en cache

Les données suivantes devront être chargées une seule fois et réutilisées :

- la carte terrestre Natural Earth ;
- les grilles GRIB décodées ;
- les polaires ;
- les configurations de course.

Le cache devra être invalidé lorsqu'un fichier est remplacé ou lorsqu'un
nouveau run météo est téléchargé.

La mise en cache doit accélérer les calculs répétés, en particulier lors des
tests de plusieurs caps ou de plusieurs stratégies sur une même zone météo.
Elle réduira le temps de chargement des données, mais ne supprimera pas le
coût principal de la simulation des caps et des isochrones.

Priorité recommandée :

1. mettre en cache les grilles GRIB décodées ;
2. mettre en cache la carte terrestre Natural Earth ;
3. mettre en cache la polaire ;
4. optimiser ensuite la simulation elle-même.

Le cache devra être invalidé lorsque :

- un fichier GRIB est remplacé ;
- une nouvelle exécution météo est téléchargée ;
- la polaire est modifiée ;
- la carte terrestre est modifiée.

Les tests devront mesurer séparément le temps de chargement des données et le
temps de calcul de la route, avant et après activation du cache.

### Optimiser les contraintes terrestres

Pour réduire le coût des vérifications :

- utiliser une grille spatiale adaptée à la zone de course ;
- simplifier les polygones pour le filtrage préliminaire ;
- appliquer la géométrie détaillée uniquement aux routes conservées ;
- conserver une marge côtière de sécurité dans la validation finale.

### Tests de performance à ajouter

- mesurer le temps avec des caps espacés de 30, 15 et 5 degrés ;
- comparer les horizons de 60 et 240 minutes ;
- comparer avec et sans cache des GRIB et des terres ;
- vérifier que l'optimisation ne produit aucun segment terrestre ;
- mesurer l'écart de progression entre le mode rapide et le mode précis ;
- conserver un temps de référence sur le scénario de course réel.

Les benchmarks Go de référence peuvent être lancés depuis `backend`.

Bash :

```bash
go test ./internal/routing -run '^$' -bench 'BenchmarkBuildIsochroneSearch' -benchmem -count=3
```

PowerShell :

```powershell
go test ./internal/routing -run '^$' -bench 'BenchmarkBuildIsochroneSearch' -benchmem -count=3
```

Le résultat à conserver avant toute optimisation est le temps moyen des deux
benchmarks et leur consommation mémoire. Une optimisation sera retenue si elle
réduit le temps sans autoriser de segment interdit.

## Modes de calcul

Le mode `precise` conserve le calcul complet avec isochrones, caps espacés de
15 degrés et jusqu'à 100 candidats. Le mode `quick` conserve une route
affichable avec des isochrones simplifiés, des caps espacés de 30 degrés et
jusqu'à 50 candidats.

Le mode précis reste le mode par défaut pour préserver le comportement
existant. Le mode rapide est recommandé pour une destination courte ou pour
comparer rapidement plusieurs stratégies.

L'API accepte les champs JSON suivants :

```json
{"mode":"quick"}
```

Les valeurs permises sont `quick` et `precise`.

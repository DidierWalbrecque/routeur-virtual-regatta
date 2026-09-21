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

Une première version du cache est maintenant utilisée par l'API pour les
chargements de GRIB, de polaire et de carte terrestre. Elle réutilise les
données tant que la taille et la date de modification des fichiers restent
identiques.

Les données suivantes devront être chargées une seule fois et réutilisées :

- la carte terrestre Natural Earth ;
- les grilles GRIB décodées ;
- les polaires ;
- les configurations de course.

Le cache devra être invalidé lorsqu'un fichier est remplacé ou lorsqu'un
nouveau run météo est téléchargé.

La mise en cache doit réduire le temps de chargement des données lors des
calculs répétés, en particulier lors des tests de plusieurs caps sur une même
zone météo. Elle ne supprimera pas le coût principal de la simulation des caps
et des isochrones ; un gain visible dans le temps total ne doit donc pas être
supposé sans mesure.

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
temps de calcul de la route, avant et après activation du cache. Le second
calcul doit rester fonctionnel et identique au premier ; le bénéfice du cache
doit être confirmé par ces mesures et non par une impression visuelle.

L'API expose maintenant ces mesures dans `performance` : temps total, temps de
chargement météo, polaire et carte terrestre, temps des caps pratiques, temps
des isochrones, ainsi que le nombre de lectures servies par le cache et de
rechargements effectués. Ces informations sont également affichées dans
l'interface après chaque calcul.

### Nettoyer les anciens fichiers GRIB

Le nettoyage automatique est maintenant effectué après chaque téléchargement
complet réussi, par l'API et par la commande `gribrefresh`.

Le téléchargement d'un nouveau run météo ne doit pas laisser s'accumuler
indéfiniment les anciens fichiers GRIB dans `data/weather`.

Après un téléchargement réussi, le routeur devra :

- identifier le run météo conservé comme courant ;
- supprimer les fichiers GRIB des anciens runs qui ne sont plus utilisés ;
- conserver tous les horizons du run courant nécessaires aux calculs ;
- ne supprimer aucun fichier si le nouveau téléchargement est incomplet ou en
  échec.

Le nettoyage devra être explicite et contrôlé afin d'éviter de supprimer un
fichier encore utilisé par une simulation en cours. Un test devra vérifier que
les anciens fichiers sont supprimés après un téléchargement complet et qu'ils
restent présents après un téléchargement partiellement échoué.

### Optimiser les contraintes terrestres

Pour réduire le coût des vérifications :

- utiliser une grille spatiale adaptée à la zone de course ;
- simplifier les polygones pour le filtrage préliminaire ;
- appliquer la géométrie détaillée uniquement aux routes conservées ;
- conserver une marge côtière de sécurité dans la validation finale.

Le contrôle géométrique réutilise maintenant les anneaux dépliés d'un polygone
pour vérifier à la fois l'intersection et la distance à la côte. Cela évite de
parcourir et projeter deux fois les mêmes coordonnées pour chaque segment.
La validation finale reste inchangée : tout segment qui touche une terre ou
qui entre dans la marge côtière est refusé.

### Paralléliser l'expansion des isochrones

L'expansion des différents parents d'une couche d'isochrone est indépendante
pendant l'évaluation des caps. Cette étape est maintenant exécutée en
parallèle, avec un nombre de workers adapté au nombre de processeurs
disponibles.

Les résultats sont ensuite regroupés dans l'ordre des parents. Le traitement
des transitions, des contraintes terrestres et de l'élagage reste déterministe
et séquentiel, afin de préserver la sécurité des routes et la reproductibilité
des résultats.

Le benchmark de référence montre une réduction du temps d'environ 30 % sans
contrainte et d'environ 45 % avec contrainte sur la machine de développement.
Ces valeurs doivent être confirmées avec le scénario réel de course.

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

## Tester le backend

Depuis la racine du projet.

### PowerShell

```powershell
Set-Location ".\backend"
go test ./...
Set-Location ".."
```

### Bash

```bash
cd backend
go test ./...
cd ..
```

## Démarrer directement avec Go

### PowerShell

```powershell
Set-Location ".\backend"
go run ./cmd/api
```

### Bash

```bash
cd backend
go run ./cmd/api
```

L'API est disponible sur :

```text
http://localhost:8080
```

Route de contrôle :

```text
GET http://localhost:8080/health
```

Pour arrêter le serveur dans les deux terminaux :

```text
Ctrl + C
```

## Démarrer avec Docker Compose

Les commandes suivantes doivent être exécutées depuis la racine du projet.

### PowerShell

```powershell
docker compose up --build
```

### Bash

```bash
docker compose up --build
```

Pour arrêter l'application :

```text
Ctrl + C
```

Puis supprimer les conteneurs et le réseau Compose :

### PowerShell

```powershell
docker compose down
```

### Bash

```bash
docker compose down
```

## Tester la route de contrôle

Lorsque l'application fonctionne :

### PowerShell

```powershell
$response = Invoke-WebRequest -UseBasicParsing -Uri "http://localhost:8080/health"
$response.StatusCode
$response.Content
```

### Bash

```bash
curl --fail http://localhost:8080/health
```

Réponse attendue :

```json
{"status":"ok"}
```

## Vérifier une situation de course

La commande `polarcheck` compare une situation relevée manuellement dans
Virtual Regatta avec la polaire locale.

Depuis le dossier `backend` :

```bash
go run ./cmd/polarcheck -polar ../data/polars/ultim_bp_11.json -tws 20 -twa 90 -category 6 -budget 440
```

Pour déclarer des équipements déjà possédés :

```bash
go run ./cmd/polarcheck -polar ../data/polars/ultim_bp_11.json -tws 20 -twa 90 -category 6 -budget 780 -owned code0,foils
```

Valeurs acceptées par `-owned` : `light`, `heavy`, `code0`, `foils`,
`hull_polish` et `pro_winches`.

La commande affiche la meilleure voile, la vitesse attendue et les prochains
achats abordables. Elle ne se connecte pas au site Virtual Regatta.

## Vérifier un déplacement pendant une course

La commande `navigationcheck` calcule une position théorique à partir d'un
relevé manuel effectué dans Virtual Regatta.

Depuis le dossier `backend` :

```bash
go run ./cmd/navigationcheck -position "13°N 04'08\" - 121°E 37'42\"" -heading 346 -speed 5.8 -minutes 10
```

La position peut également être fournie en degrés décimaux :

```bash
go run ./cmd/navigationcheck -latitude 13.068889 -longitude 121.628333 -heading 346 -speed 5.8 -minutes 10
```

Les paramètres correspondent à :

- `-position` : coordonnées au format affiché par Virtual Regatta ;
- `-latitude` et `-longitude` : autre saisie possible en degrés décimaux ;
- `-heading` : cap suivi, de 0° inclus à 360° exclus ;
- `-speed` : vitesse moyenne en nœuds ;
- `-minutes` : durée du déplacement en minutes.

La commande affiche la distance parcourue et la nouvelle position théorique,
à la fois au format Virtual Regatta et en degrés décimaux. Dans une valeur
décimale comme `121.613088°`, `.613088` est une fraction de degré et non un
nombre de minutes ; la sortie VR correspondante est `121°E 36'47"`.
Pour comparer le résultat au jeu, relever la position au début puis à la fin de
la période. Le calcul suppose un cap et une vitesse constants : il ne tient pas
encore compte des variations de vent, des manœuvres ou de la stamina.

## Télécharger une série météo NOAA

La commande `gribdownload` télécharge depuis le filtre officiel NOMADS
uniquement les composantes U et V à 10 mètres du modèle GFS 0,25°. Le
téléchargement reste indépendant du moteur de routage.

Depuis le dossier `backend` :

```bash
go run ./cmd/gribdownload \
  -date 20260905 \
  -cycle 06 \
  -hours 0,3,6 \
  -output ../data/weather
```

Les paramètres correspondent à :

- `-date` : date du run au format `YYYYMMDD` ;
- `-cycle` : cycle `00`, `06`, `12` ou `18` ;
- `-hours` : échéances séparées par des virgules ;
- `-output` : dossier local de destination.

La date et le cycle doivent encore être disponibles sur NOMADS. Les fichiers
déjà présents et valides sont réutilisés. Chaque téléchargement est écrit dans
un fichier temporaire, entièrement décodé et validé, puis installé sous un nom
comme `gfs.2026090506.f003.grb2`.

Service officiel :
[NOAA NOMADS — filtre GFS 0,25°](https://nomads.ncep.noaa.gov/cgi-bin/filter_gfs_0p25.pl).

## Vérifier un fichier météo GRIB2

La commande `gribcheck` vérifie que le lecteur natif Go peut extraire et
valider les composantes U et V du vent à 10 mètres d'un fichier NOAA GFS.

Depuis le dossier `backend` :

```bash
go run ./cmd/gribcheck -file ../data/weather/gfs.2026042906.f003.grb2
```

Pour interpoler le vent à une position précise :

```bash
go run ./cmd/gribcheck -file ../data/weather/gfs.2026042906.f003.grb2 -latitude 13.121111 -longitude 121.586667
```

Pour contrôler la météo pendant une course, répéter `-file` et fournir
l'instant recherché au format RFC3339 UTC avec `-at` :

```bash
cd ~/Documents/dev/routeur-virtual-regatta/backend

go run ./cmd/gribcheck \
  -file ../data/weather/gfs.2026090506.f000.grb2 \
  -file ../data/weather/gfs.2026090506.f003.grb2 \
  -file ../data/weather/gfs.2026090506.f006.grb2 \
  -latitude 13.121111 \
  -longitude 121.586667 \
  -at 2026-09-05T07:30:00Z
```

Remplacer la latitude, la longitude et l'heure UTC par les valeurs de la course.
La date, le cycle et les noms de fichiers doivent correspondre à la série
préalablement téléchargée avec `gribdownload`.

L'instant doit être compris entre les échéances chargées. Le vent est interpolé
sur les composantes U et V, d'abord dans l'espace pour chaque grille, puis dans
le temps. La vitesse et la direction sont calculées après ces interpolations.
Cette commande vérifie actuellement la météo seulement : elle ne calcule pas
encore une route complète et ne se connecte pas à Virtual Regatta.

La commande affiche la vitesse en mètres par seconde et en nœuds, ainsi que la
direction météorologique. Pour une seule échéance exacte, elle affiche également
les composantes U et V interpolées.

Les fichiers météo restent locaux dans `data/weather/` et ne sont pas ajoutés
au dépôt Git. Chaque fichier attendu contient deux messages, correspondant aux
composantes U et V du vent à 10 mètres.

## Vérifier une étape complète pendant une course

La commande `racecheck` réunit les composants déjà disponibles : météo GRIB2,
calcul du TWA et de l'amure, polaire, équipements, meilleure voile, vitesse et
position suivante.

Depuis le dossier `backend` :

```bash
go run ./cmd/racecheck \
  -file ../data/weather/gfs.2026090506.f000.grb2 \
  -file ../data/weather/gfs.2026090506.f003.grb2 \
  -file ../data/weather/gfs.2026090506.f006.grb2 \
  -polar ../data/polars/ultim_bp_11.json \
  -position "13°N 07'16\" - 121°E 35'12\"" \
  -at 2026-09-05T07:30:00Z \
  -heading 321 \
  -minutes 240 \
  -step-minutes 10 \
  -owned code0,foils
```

La position peut aussi être fournie avec `-latitude` et `-longitude` en
degrés décimaux. Les équipements acceptés par `-owned` sont les mêmes que pour
`polarcheck`.

Cette commande simule une navigation à cap constant pendant la durée totale
donnée par `-minutes`. L'option `-step-minutes` fixe le pas maximal de
calcul : à chaque pas, le vent, le TWA, l'amure, la meilleure voile et la
vitesse sont recalculés à la nouvelle position et au nouvel instant. Le dernier
pas est raccourci lorsque la durée totale n'est pas un multiple du pas.

Pour couvrir quatre heures avec un recalcul toutes les dix minutes, utiliser
`-minutes 240 -step-minutes 10`. Les fichiers GRIB2 doivent couvrir toute la
période simulée.

La commande ne recherche pas encore le meilleur cap, ne modélise pas les
manœuvres ou la stamina et ne se connecte pas à Virtual Regatta.

## Calculer une première route par isochrones

La commande `routecheck` explore plusieurs caps à chaque pas, construit des
fronts d'isochrones successifs, élimine les positions proches les moins
intéressantes puis reconstruit le meilleur chemin conservé vers la destination.

Depuis le dossier `backend` :

```bash
go run ./cmd/routecheck \
  -file ../data/weather/gfs.2026090506.f000.grb2 \
  -file ../data/weather/gfs.2026090506.f003.grb2 \
  -file ../data/weather/gfs.2026090506.f006.grb2 \
  -polar ../data/polars/ultim_bp_11.json \
  -position "13°N 07'16\" - 121°E 35'12\"" \
  -destination "14°N 00'00\" - 120°E 00'00\"" \
  -at 2026-09-05T07:30:00Z \
  -minutes 60 \
  -step-minutes 10 \
  -heading-spacing 15 \
  -cell-size 1 \
  -owned code0,foils
```

Les paramètres spécifiques au routage correspondent à :

- `-destination` : point recherché au format Virtual Regatta ;
- `-minutes` : horizon total du calcul, multiple exact du pas ;
- `-step-minutes` : durée séparant deux fronts d'isochrones ;
- `-heading-spacing` : écart en degrés entre les caps explorés ;
- `-cell-size` : côté des cellules d'élagage en milles nautiques.

La destination peut également être fournie en degrés décimaux avec
`-destination-latitude` et `-destination-longitude`. Les paramètres de
position initiale, météo, polaire et équipements utilisent les mêmes formats
que `racecheck`.

La commande affiche chaque étape de la route retenue avec le cap, le vent, le
TWA, l'amure, la voile, la vitesse, la position, la distance restante et la
progression globale. Elle indique aussi le nombre de positions conservées dans
chaque front.

Un espacement de caps ou une taille de cellule plus petits augmentent la
précision, mais aussi le temps de calcul et la mémoire utilisés. Commencer avec
`-heading-spacing 15 -cell-size 1` pour un test d'une heure. Pour tester quatre
heures, utiliser `-minutes 240` avec des fichiers GRIB2 couvrant toute cette
période.

Il s'agit encore d'une route théorique sans terres, portes de course, stamina
ni pénalités de manœuvre. La commande ne se connecte pas à Virtual Regatta et
ne commande jamais le bateau.

## État du projet

Phases 1 et 3 terminées : environnement Go et Docker opérationnel, chargement et interpolation des polaires validés.

Phase 2 en cours : modèle métier construit progressivement avec les modules concernés.

Travail actuel : phase 6 — lecture et interpolation de la météo GRIB2.

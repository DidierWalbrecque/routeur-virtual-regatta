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
  -gate "porte nord|13.2,121.4|13.3,121.5|either" \
  -at 2026-09-05T07:30:00Z \
  -minutes 60 \
  -step-minutes 10 \
  -heading-spacing 15 \
  -cell-size 1 \
  -max-candidates 500 \
  -arrival-radius 1 \
  -stamina 100 \
  -maneuver-model experimental-additive \
  -stamina-recovery-rate 0 \
  -current-heading 321 \
  -current-sail Jib \
  -land-geojson ../data/land/natural-earth-10m-land.geojson \
  -coastal-margin 2 \
  -geojson ../data/routes/route.geojson \
  -owned code0,foils
```

Les paramètres spécifiques au routage correspondent à :

- `-destination` : point recherché au format Virtual Regatta ;
- `-gate` : porte obligatoire au format
  `nom|marque 1|marque 2|sens` ; option répétable et respectée dans l'ordre ;
- `-minutes` : horizon total du calcul, multiple exact du pas ;
- `-step-minutes` : durée séparant deux fronts d'isochrones ;
- `-heading-spacing` : écart en degrés entre les caps explorés ;
- `-cell-size` : taille minimale des cellules d'élagage en milles nautiques ;
- `-max-candidates` : nombre maximal de positions conservées par front,
  par défaut 500 ; la valeur 0 désactive la limite ;
- `-arrival-radius` : distance maximale de la destination considérée comme
  une arrivée, par défaut 1 NM ;
- `-stamina` : réserve initiale du bateau entre 0 et 100, par défaut 100 ;
- `-maneuver-model` : modèle de manœuvres à utiliser, `off` par défaut ou
  `experimental-additive` pour activer les pénalités et la consommation ;
- `-stamina-recovery-rate` : récupération pendant un pas sans virement,
  empannage ni changement de voile, en points de stamina par heure ; valeur
  expérimentale nulle par défaut et utilisable uniquement avec
  `experimental-additive` ;
- `-current-heading` : cap actuellement suivi, de 0° inclus à 360° exclus ;
- `-current-sail` : nom de la voile actuelle tel qu'il apparaît dans la
  polaire, sans distinction entre majuscules et minuscules ;
- `-land-geojson` : chemin optionnel d'une `FeatureCollection` GeoJSON
  contenant les terres sous forme de `Polygon` ou `MultiPolygon` ;
- `-coastal-margin` : distance minimale de la côte en milles nautiques,
  nulle par défaut et utilisable uniquement avec `-land-geojson` ;
- `-exclusion-geojson` : chemin optionnel d'une `FeatureCollection`
  GeoJSON contenant les zones interdites de la course sous forme de
  `Polygon` ou `MultiPolygon` ;
- `-geojson` : chemin optionnel du fichier GeoJSON à produire. Le dossier
  parent est créé automatiquement.

Pour ajouter des zones interdites propres à une course, créer une
`FeatureCollection` GeoJSON locale puis compléter la commande avec :

```bash
-exclusion-geojson ../data/course/exclusions.geojson
```

### Télécharger les côtes Natural Earth

Pour les premiers essais, la source recommandée est **Natural Earth 1:10m
Land** : elle contient les polygones terrestres et les îles principales, et ses
[données vectorielles sont dans le domaine public](https://www.naturalearthdata.com/downloads/10m-physical-vectors/).

Depuis le dossier `backend`, télécharger directement le GeoJSON validé :

```bash
go run ./cmd/landdownload -output ../data/land
```

La commande utilise une version épinglée du
[dépôt vectoriel Natural Earth](https://github.com/nvkelso/natural-earth-vector),
limite la taille reçue, vérifie entièrement les géométries `Polygon` et
`MultiPolygon`, puis installe atomiquement :

```text
../data/land/natural-earth-10m-land.geojson
```

Un fichier local déjà valide est réutilisé sans nouveau téléchargement. Il
reste dans `data/land/`, qui est ignoré par Git.

Le routeur refuse tout segment qui entre dans un polygone, touche sa limite ou
passe à moins de la distance définie par `-coastal-margin`. Commencer avec
2 NM pour les essais. Natural Earth convient à un MVP mondial mais ne
représente pas tous les petits îlots ni le trait de côte avec une précision
nautique : même avec une marge, ce résultat ne constitue pas une garantie de
navigation sûre au ras de la côte.

Chaque marque d'une `-gate` accepte soit `latitude,longitude` en degrés
décimaux, soit une position complète au format Virtual Regatta. Le sens vaut
`either`, `right-to-left` ou `left-to-right`, par rapport à la ligne
orientée de la première marque vers la seconde. Lorsque plusieurs options
`-gate` sont fournies, elles doivent être franchies dans leur ordre
d'apparition avant que la destination puisse être déclarée atteinte.

La destination peut également être fournie en degrés décimaux avec
`-destination-latitude` et `-destination-longitude`. Les paramètres de
position initiale, météo, polaire et équipements utilisent les mêmes formats
que `racecheck`.

`-current-heading` et `-current-sail` sont facultatifs, mais doivent être
fournis ensemble. Ils permettent au moteur de détecter une manœuvre dès le
premier front à partir de l'état réel du bateau. La voile doit être présente
dans la polaire et disponible avec les équipements déclarés par `-owned`.

La commande affiche chaque étape de la route retenue avec le cap, le vent, le
TWA, l'amure, la voile, la vitesse, la manœuvre détectée, le changement de
voile, la stamina, la position, la distance restante et la progression globale.
Lorsqu'une pénalité de vitesse est active, elle affiche également son ratio, sa
durée appliquée, son reliquat et la vitesse moyenne réellement parcourue.
Elle indique aussi le nombre de positions conservées dans
chaque front, la taille de cellule effectivement utilisée et si la destination
a été atteinte. Le calcul s'arrête au premier front dont le meilleur point
entre dans le rayon d'arrivée.

Avec `-geojson`, elle écrit également une `FeatureCollection` comprenant la
ligne de route, le départ, la destination et chaque étape. Les points d'étape
conservent le cap, la voile, le vent, le TWA, l'amure, la vitesse, la
manœuvre, le changement de voile, la stamina et la progression ; le fichier
peut être chargé directement dans MapLibre ou un
logiciel SIG.

Lorsque le nombre de positions dépasse `-max-candidates`, le moteur agrandit
progressivement les cellules pour réduire le front tout en conservant une
répartition géographique. La taille demandée par `-cell-size` reste donc un
minimum ; la sortie permet de voir si elle a dû être augmentée.

Un espacement de caps plus petit, une cellule minimale plus petite ou une
limite de candidats plus élevée préservent davantage de routes alternatives,
mais augmentent le temps de calcul et la mémoire utilisés. Commencer avec
`-heading-spacing 15 -cell-size 1` pour un test d'une heure. Pour tester quatre
heures, utiliser `-minutes 240` avec des fichiers GRIB2 couvrant toute cette
période.

Sans `-land-geojson`, il s'agit encore d'une route théorique sans terres.
Sans `-gate`, le routeur vise directement la destination. Le fichier fourni
avec `-exclusion-geojson` ajoute des polygones interdits propres à la course ;
tout segment qui entre dans une zone ou touche sa limite est rejeté. Avec les
options de terres, les intersections terrestres du fichier
fourni et les segments trop proches selon `-coastal-margin` sont rejetés. Avec
`-maneuver-model off`, les manœuvres restent seulement observées. Le profil
`experimental-additive` utilise les pénalités de winch de la polaire, les coûts
de stamina connus de 10 points pour un virement ou un empannage et de 20 points
pour un changement de voile. Pour permettre les essais avant confirmation de
la règle Virtual Regatta, il additionne les consommations simultanées, multiplie
leurs ratios de vitesse et additionne leurs durées. Les coefficients de vent et
de stamina restent neutres, égaux à 1. La récupération peut être testée avec
`-stamina-recovery-rate`, mais son taux est nul par défaut et doit rester
considéré comme une hypothèse configurable tant qu'il n'est pas confirmé par
des mesures ou une source officielle. Ce profil est donc expérimental et ses
résultats ne doivent pas être considérés comme une reproduction officielle du
jeu. La commande ne se connecte pas à Virtual Regatta et ne commande jamais le
bateau.

## État du projet

Phases 1 et 3 terminées : environnement Go et Docker opérationnel, chargement et interpolation des polaires validés.

Phase 2 en cours : modèle métier construit progressivement avec les modules concernés.

Travail actuel : phase 8 — moteur de routage par isochrones en pleine mer.

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

### Interface, extension et relais Flask

Les tests de l'interface React et de l'extension navigateur (Node 22.18 ou
plus récent) :

```bash
cd frontend
npm install
npm test
```

Les tests du relais Flask (environnement `.venv-flask` activé) :

```bash
python -m unittest discover -s flaskapp/tests -p "test_*.py"
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

## Utiliser l'interface web

L'interface est une application React (`frontend`) : carte MapLibre, formulaire
de calcul, état Virtual Regatta, portes et zones de la course, run GFS utilisé
et rapport de route.

### Lancement

Lancer l'API, depuis un premier terminal à la racine du dépôt :

```powershell
go run -C backend ./cmd/api
```

Dans un second terminal, lancer l'interface :

```powershell
Set-Location .\frontend
npm install
npm run dev
```

Ouvrir ensuite l'adresse affichée par Vite, habituellement
`http://localhost:5173`. Vite redirige `/api` vers l'API Go (port 8080).
Redémarrer l'API après chaque mise à jour du code Go : un ancien processus
continue sinon de répondre avec l'ancienne version.

### Premier calcul

1. Cliquer sur le bouton du nuage (actualisation des GRIB) : le dernier run GFS
   disponible est téléchargé dans `data/weather`. Une carte en bas à gauche
   indique le run, l'horizon de prévision et l'âge des données.
2. Ouvrir « Calculer une route » avec le premier bouton de la barre d'actions.
   Si l'extension envoie l'état du bateau, le départ, la stamina, les
   équipements et la polaire sont renseignés automatiquement.
3. Choisir l'arrivée (saisie au format Virtual Regatta ou une porte restante),
   l'heure de départ, l'horizon, le mode et vérifier la polaire.
4. Lancer le calcul : les isochrones s'affichent au fur et à mesure, puis la
   route. « Détails de la route » ouvre le rapport.

### Mode et horizon du routage

| Mode | Pas des isochrones | Caps explorés | Horizon maximal |
|---|---|---|---|
| Précis | 10 min | tous les 15°, affinés au degré | 3 jours |
| Rapide | 15 min | tous les 30°, affinés au degré | 5 jours |

L'horizon se choisit dans une liste : « Jusqu'à l'arrivée » (le calcul s'arrête
dès que l'arrivée est atteinte), 4 heures, 24 heures, 2 jours, 3 jours et, en
mode Rapide seulement, 5 jours. Il ne dépasse jamais ce que couvrent les
prévisions téléchargées depuis le départ : l'interface le raccourcit et le
signale. Le bouton GRIB télécharge de quoi couvrir l'horizon choisi (72 h par
défaut, 84 h pour 3 jours, 132 h pour 5 jours). L'API refuse une durée au-delà
du maximum du mode. Au-delà de 3 jours, la prévision GFS devient peu fiable :
la route donne une tendance, pas des caps à suivre.

Mesuré sur une route Bretagne–Antilles avec les côtes : le mode Précis demande
environ 0,15 s par heure d'horizon (3 à 4 s pour 24 h, 9 s pour 63 h), le mode
Rapide environ deux fois moins.

### Polaire du bateau

La liste des polaires vient de `GET /api/polars` (fichiers de `data/polars`).
L'extension envoie, avec l'état du bateau, les valeurs de la réponse du jeu qui
peuvent l'identifier (`boatHints`) : l'interface les rapproche de l'identifiant
(`_id`) ou du nom (`label`) de chaque polaire, sélectionne la bonne et affiche
« Bateau détecté : … » sous le champ. Sans détection, choisir la polaire à la
main : le message « Bateau non fourni par Virtual Regatta » le rappelle.

### Rapport de route

Chaque ligne décrit le départ d'un pas : date, temps écoulé, cap à programmer,
TWA, bord (T tribord, B bâbord), position, TWD, TWS, vitesse, activation des
foils, coefficient de manœuvre, voile et stamina. Les lignes où une manœuvre
(virement, empannage ou changement de voile) a lieu sont surlignées.

### Caps et affinage

Le moteur explore une grille de caps puis affine, autour des meilleurs, des caps
entiers (paliers de 5°, 2° puis 1° pour un pas de 15°), car le jeu n'accepte que
des caps entiers. La route ne dépend ainsi plus du pas de la grille. Cet affinage
double environ le temps de calcul des isochrones ; il est toujours activé dans
l'interface.

### Zones interdites

Le bouton « Zones interdites » de la barre d'actions ouvre la liste des zones.
Elles sont appliquées à tout le trajet et à l'approche finale, en plus de la
côte ; un départ ou une arrivée situé dans une zone est refusé.

- **Zones de la course.** L'extension `vr-state-extension` relève les polygones
  interdits dans la réponse du jeu et les transmet à l'API. Ils apparaissent
  dans « Zones de la course » (cochables, non modifiables) et sont appliqués
  comme les zones dessinées. Le nom du champ du jeu n'étant pas documenté,
  l'extension cherche les listes de polygones dont le nom évoque une zone ; si
  rien n'apparaît, le panneau affiche la structure de la manche reçue (« Champs
  reçus de la manche ») pour retrouver le bon champ.
- **Zones dessinées.** « Dessiner une zone » permet de tracer un polygone sur la
  carte : un clic par sommet, clic sur le premier point ou Entrée pour terminer,
  Retour arrière pour défaire le dernier point, Échap pour abandonner. Elles sont
  conservées dans le navigateur.

Les zones cochées sont envoyées à chaque calcul (`exclusionZones`).

### Portes de la course

L'extension relève les portes de la manche (`leg.checkpoints`) et les groupes
déjà franchis (`gateGroupCounters`). L'API en déduit les portes à franchir, dans
l'ordre reçu, avec leur sens (tribord, bâbord ou libre). Les portes dont le milieu
est sur une terre sont ignorées, comme dans la liste de l'interface. Le calcul s'appuie sur
ces portes lorsque « Utiliser la position, la stamina et les options reçues de
Virtual Regatta » est coché.

### Extension et relais Flask

Flask n'est plus une interface : c'est un relais qui sert le build React
(`npm run build`) et relaie les appels vers l'API Go. Il reste nécessaire pour
l'extension `vr-state-extension`, qui envoie l'état du bateau à
`http://localhost:5000`. Voir [`docs/flask.md`](docs/flask.md) et
[`vr-state-extension/README.md`](vr-state-extension/README.md).

### Actualiser les GRIB depuis l'API

Lorsque l'API est lancée sur `http://localhost:8080`, la même actualisation
peut être déclenchée directement avec `curl` :

```bash
curl -X POST http://localhost:8080/api/weather/refresh \
  -H "Content-Type: application/json" \
  -d '{"forecastHours":[0,3,6,9,12,15,18,21,24],"lookbackHours":48}'
```

Cet exemple demande les échéances `f000` à `f024` par pas de trois heures. Sans
`forecastHours`, l'API utilise cette même liste ; l'interface en demande davantage
selon l'horizon choisi (jusqu'à `f072`, `f084` ou `f132`). Un calcul ne peut pas
dépasser la dernière échéance téléchargée.
`lookbackHours` indique à l'API de rechercher un run GFS disponible dans les
48 dernières heures. L'API essaie les cycles récents et revient au run
précédent si le dernier n'est pas encore publié par NOAA.

PowerShell équivalent :

```powershell
$body = @{
    forecastHours = @(0, 3, 6, 9, 12, 15, 18, 21, 24)
    lookbackHours = 48
} | ConvertTo-Json

Invoke-RestMethod `
  -Uri http://localhost:8080/api/weather/refresh `
  -Method Post `
  -ContentType "application/json" `
  -Body $body
```

Exemple de réponse :

```json
{"run":"2026092012","files":["..\\data\\weather\\gfs.2026092012.f000.grb2","..\\data\\weather\\gfs.2026092012.f003.grb2","..\\data\\weather\\gfs.2026092012.f006.grb2","..\\data\\weather\\gfs.2026092012.f009.grb2","..\\data\\weather\\gfs.2026092012.f012.grb2"],"refreshed":"2026-09-20T19:44:17Z"}
```

Dans cette réponse :

- `run` est le run GFS retenu, ici le 20 septembre 2026 à 12 UTC ;
- `files` contient les chemins des fichiers GRIB validés et installés dans
  `data/weather` ;
- `refreshed` est l'heure UTC à laquelle l'actualisation a été terminée.

Les fichiers retournés peuvent ensuite être transmis à `racecheck`,
`routecheck` ou à l'interface web.

Les chemins météo dépendent du run GFS téléchargé : l'interface les reprend de
la réponse de l'actualisation et refuse de calculer tant que les GRIB n'ont pas
été actualisés. Les chemins de polaire sont relatifs au dossier `backend`, par
exemple `../data/polars/ultim_bp_11.json`, puisque l'API est lancée depuis ce
dossier.

## API HTTP

| Route | Rôle |
|---|---|
| `GET /health` | contrôle de disponibilité |
| `GET /api/polars` | polaires de `data/polars` : fichier, `id` et nom du bateau |
| `POST /api/weather/refresh` | télécharge le dernier run GFS (`forecastHours`, `lookbackHours`) |
| `POST /api/practical-headings` | calcule la route et renvoie le résultat complet |
| `POST /api/practical-headings/stream` | même calcul, envoyé en flux SSE (`start`, `frontier`, `complete`) |
| `POST` / `GET /api/vr/state` | enregistre / lit le dernier état reçu de Virtual Regatta |

Champs principaux de la requête de calcul : `weatherFiles`, `polarPath`,
`position` et `destination` (`latitude`/`longitude` ou `virtualRegatta`),
`instant` (RFC 3339), `durationMinutes`, `stepMinutes`, `mode` (`quick` ou
`precise`), `owned`, `useVirtualRegattaState` et `exclusionZones` (liste de
`{name, points:[{latitude, longitude}]}`). La durée est limitée à 3 jours en mode
précis et à 5 jours en mode rapide. Chaque point de la route renvoyée porte le
détail du pas qui y mène (`twd`, `tws`, `twa`, `tack`, `speedKnots`,
`foilPercent`, `speedCoefficient`, `maneuver`).

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
- `-refine-headings` : ajoute des caps entiers autour des meilleurs caps de la
  grille (paliers de 5°, 2° puis 1° pour un pas de 15°). La route ne dépend alors
  plus du pas de la grille, au prix d'un calcul environ deux fois plus long.
  L'interface web l'active toujours ; l'option est désactivée par défaut ici ;
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

Chaque polygone volumineux (Eurasie, Afrique…) est indexé par une grille
d'arêtes : un segment n'est testé que contre les arêtes voisines. Un calcul avec
Natural Earth 10 m qui prenait plusieurs minutes dure ainsi une à quelques
secondes, avec exactement les mêmes résultats.

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

## Calibrer le routeur avec un relevé Virtual Regatta

Ajouter `-actual-position` à `racecheck` après un relevé à 30 minutes, 1 heure
ou 4 heures. Le rapport indique l'écart de position, les vitesses moyennes
simulée et réelle, et compare la voile si `-actual-sail` est fourni.

```bash
go run ./cmd/racecheck \
  -file ../data/weather/f000.grb2 -file ../data/weather/f003.grb2 \
  -polar ../data/polars/ultim_bp_11.json \
  -position '13°N 07'16" - 121°E 35'12"' \
  -actual-position '13°N 10'00" - 121°E 37'30"' \
  -at 2026-04-29T09:00:00Z -heading 90 -minutes 30 -actual-sail Jib
```

Conserver le même cap, les mêmes équipements et des GRIB couvrant toute la
période pour que l'écart soit interprétable.

## État du projet

Phases 1 et 3 terminées : environnement Go et Docker opérationnel, chargement et interpolation des polaires validés.

Interface web React opérationnelle : calcul par isochrones avec affinage des caps,
côtes et zones interdites (dessinées ou reçues du jeu), portes de la course,
détection de la polaire du bateau, horizon jusqu'à 3 jours (précis) ou 5 jours
(rapide) et rapport de route détaillé.

Phase 2 en cours : modèle métier construit progressivement avec les modules concernés.

Travail actuel : phase 8 — moteur de routage par isochrones en pleine mer.

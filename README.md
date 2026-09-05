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

La commande affiche alors les composantes U et V interpolées, la vitesse en
mètres par seconde et en nœuds, ainsi que la direction météorologique.

Le fichier météo reste local dans `data/weather/` et n'est pas ajouté au dépôt
Git. Le fichier d'exemple contient deux messages, correspondant aux composantes
U et V du vent à 10 mètres.

## État du projet

Phases 1 et 3 terminées : environnement Go et Docker opérationnel, chargement et interpolation des polaires validés.

Phase 2 en cours : modèle métier construit progressivement avec les modules concernés.

Travail actuel : phase 6 — lecture et interpolation de la météo GRIB2.

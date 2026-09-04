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

## État du projet

Phases 1 et 3 terminées : environnement Go et Docker opérationnel, chargement et interpolation des polaires validés.

Phase 2 en cours : modèle métier construit progressivement avec les modules concernés.

Travail actuel : phase 5 — calculs géographiques de navigation.

# Actualiser les GRIB NOAA

La commande `gribrefresh` recherche le run GFS le plus récent disponible. Elle
essaie les cycles de six heures dans l'ordre décroissant, réutilise les
fichiers locaux valides et revient automatiquement au run précédent si le plus
récent n'est pas encore publié par NOAA.

Depuis le dossier `backend`, PowerShell :

```powershell
go run ./cmd/gribrefresh -hours 0,3,6,9,12,15,18,21,24 -output ../data/weather
```

Depuis le dossier `backend`, Bash :

```bash
go run ./cmd/gribrefresh -hours 0,3,6,9,12,15,18,21,24 -output ../data/weather
```

La recherche couvre les 48 dernières heures par défaut. Cette période peut
être modifiée avec `-lookback-hours`. Pour forcer un run précis :

PowerShell :

```powershell
go run ./cmd/gribrefresh -date 20260920 -cycle 06 -hours 0,3,6 -output ../data/weather
```

Bash :

```bash
go run ./cmd/gribrefresh -date 20260920 -cycle 06 -hours 0,3,6 -output ../data/weather
```

Pour un routage de plusieurs jours, demander des échéances plus lointaines : par
exemple `-hours 0,3,...,72` pour trois jours (25 fichiers) ou jusqu'à `132` pour
cinq jours (45 fichiers). Un calcul ne peut pas dépasser la dernière échéance
téléchargée ; le bouton GRIB de l'interface choisit les échéances selon
l'horizon demandé.

Chaque fichier est entièrement décodé et validé avant d'être installé. La
commande ne planifie pas encore son exécution : elle doit être relancée avant
un nouveau calcul de route.

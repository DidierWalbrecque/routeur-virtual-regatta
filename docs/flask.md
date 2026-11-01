# Relais Flask

Flask n'est plus une interface web. L'interface est l'application React du
dossier `frontend`. Flask joue deux rôles :

- relayer les appels `/api/...` vers l'API Go, notamment l'état du bateau envoyé
  par l'extension `vr-state-extension` (`POST /api/vr/state`) ;
- servir l'interface React construite (`frontend/dist`) sur un seul port, sans
  serveur de développement.

Pour travailler sur l'interface, utilisez plutôt `npm run dev` dans `frontend`
(voir le README) : Vite redirige `/api` directement vers l'API Go.

## Installation

Depuis la racine du dépôt :

### PowerShell

```powershell
python -m venv .venv-flask
.\.venv-flask\Scripts\Activate.ps1
python -m pip install -r flaskapp\requirements.txt
```

### Bash

```bash
python -m venv .venv-flask
source .venv-flask/Scripts/activate
python -m pip install -r flaskapp/requirements.txt
```

## Lancement

Construire l'interface (à refaire après chaque modification du frontend) :

```bash
cd frontend
npm install
npm run build
cd ..
```

Puis, depuis la racine du dépôt, avec l'API Go lancée sur le port 8080 :

```bash
python -m flask --app flaskapp.app run --host 127.0.0.1 --port 5000
```

Ouvrir ensuite <http://127.0.0.1:5000>. Si `frontend/dist` n'existe pas, la page
d'accueil explique comment construire l'interface.

## Routes relayées

| Route Flask | Destination |
|---|---|
| `GET /api/polars` | API Go : liste des polaires disponibles |
| `POST /api/weather/refresh` | API Go : actualisation des GRIB |
| `POST /api/practical-headings` | API Go : calcul de la route |
| `POST /api/practical-headings/stream` | API Go : calcul en flux (SSE) |
| `POST` / `GET /api/vr/state` | API Go : état Virtual Regatta |

L'URL de l'API Go peut être modifiée avec `ROUTEUR_API_URL`. Le délai maximal
des appels est configurable avec `ROUTEUR_API_TIMEOUT_SECONDS`.

## Tests

```bash
python -m unittest discover -s flaskapp/tests -p "test_*.py"
```

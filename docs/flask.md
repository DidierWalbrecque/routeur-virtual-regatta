# Interface Flask

Le projet propose une interface Flask minimale composée uniquement d’une carte
MapLibre. Cette première étape affiche le fond OpenStreetMap ainsi que les
marqueurs de départ et de destination par défaut. Le formulaire et le calcul
seront ajoutés lors des étapes suivantes. Un bouton flottant permet déjà de
demander l’actualisation des GRIB ; le détail de l’opération est écrit dans la
console du navigateur et dans les logs Flask, notamment le run, le nombre de
fichiers téléchargés et le nombre d’anciens fichiers supprimés. Un second
bouton ouvre le formulaire de calcul ; celui-ci permet de renseigner le départ,
la destination, la date, la polaire, la durée, le pas, les caps, le mode et les
équipements avant de lancer le calcul.

La carte MapLibre est chargée depuis un CDN : une connexion Internet est donc
nécessaire pour le script MapLibre et les tuiles OpenStreetMap.

## Installation

Depuis la racine du dépôt :

### PowerShell

```powershell
python -m venv .venv-flask
.\.venv-flask\Scripts\Activate.ps1
python -m pip install -r flaskapp\requirements.txt

# À faire une seule fois si frontend/node_modules n'existe pas
Set-Location frontend
npm install
Set-Location ..
```

### Bash

```bash
python -m venv .venv-flask
source .venv-flask/Scripts/activate
python -m pip install -r flaskapp/requirements.txt

# À faire une seule fois si frontend/node_modules n'existe pas
cd frontend
npm install
cd ..
```

## Lancement

Depuis la racine du dépôt :

```bash
python -m flask --app flaskapp.app run --host 127.0.0.1 --port 5000
```

Ouvrir ensuite <http://127.0.0.1:5000>.

L’API Go n’est pas nécessaire pour cette étape, car la page affiche uniquement
la carte et les marqueurs par défaut. MapLibre est servi localement lorsque
`frontend/node_modules` existe, puis deux CDN sont utilisés en secours. Les
tuiles OpenStreetMap nécessitent toujours une connexion Internet.

L’URL de l’API Go peut être modifiée avec `ROUTEUR_API_URL`. Le délai maximal
des appels est configurable avec `ROUTEUR_API_TIMEOUT_SECONDS`.

## Tests

```bash
python -m unittest discover -s flaskapp/tests -p "test_*.py"
```

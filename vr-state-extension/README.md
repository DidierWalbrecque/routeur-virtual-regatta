# VR State Lite

Extension Chrome/Edge Manifest V3 minimale pour afficher l'état du bateau Virtual Regatta.

Elle récupère uniquement la dernière réponse `/getboatinfos` et expose :

- position latitude/longitude ;
- stamina ;
- options et statut Full Pack ;
- liste des voiles disponibles selon les options ;
- cap, vitesse et voile ;
- portes de parcours reçues avec leurs extrémités et leur sens de franchissement ;
- course, étape, joueur et date de mise à jour ;
- zones interdites de la course (polygones) quand la réponse du jeu en contient,
  avec un résumé de la structure de la manche (`legShape`) pour le diagnostic ;
- indices permettant d'identifier le bateau (`boatHints`) : valeurs de la réponse du
  jeu dont la clé contient `polar`, `boat`, `hull`, `yacht` ou `class`, utilisées par
  l'interface pour choisir la polaire ;
- type de course (`raceType`, par exemple `record`), dates d'ouverture et de
  fermeture du leg (`startWindow.open`/`close`, issues de `leg.open`/`leg.close`)
  et date associée à l'arrivée (`endDate`, issue de `leg.end.date`), en
  millisecondes. Leur sens exact pour un record reste à confirmer : dans la
  capture du Trophée Jules Verne, `close` (1er avril 2027) tombe après `endDate`
  (25 mars 2027).

Épreuve à record avant le départ : le jeu ne renvoie pas d'état de bateau (`bs`
est nul). L'extension publie quand même la manche avec `preStart: true` et la
ligne de départ comme position. Stamina, cap, vitesse et voile sont nuls. Les
options sont vides, car inconnues, et le routeur garde alors les équipements
choisis dans le formulaire. Une course classique sans état de bateau ne publie
rien. Au changement de course, les portes franchies, l'arrivée et les zones
mémorisées de l'ancienne course sont oubliées.

Si Flask est démarré sur `http://localhost:5000`, chaque état est envoyé à `POST http://localhost:5000/api/vr/state`, puis relayé vers l'API Go. Le dernier état est consultable avec `GET http://localhost:5000/api/vr/state`. Les logs Flask utilisent le préfixe `[VR STATE]`.

## Installation locale

1. Ouvrir `chrome://extensions` ou `edge://extensions`.
2. Activer le mode développeur.
3. Cliquer sur **Charger l'extension non empaquetée**.
4. Sélectionner le dossier `vr-state-extension`.
5. Recharger la page Virtual Regatta.
6. Ouvrir la popup de l'extension.

## Jouer depuis un autre ordinateur (Tailscale)

Le routeur tourne sur le PC fixe ; le jeu peut être ouvert sur un portable du
même réseau Tailscale. Le relais Flask n'écoute que sur le PC lui-même :
l'extension du portable envoie donc directement à l'API Go, par le proxy de
l'interface (port 5173, ouvert au réseau Tailscale par le pare-feu du PC).

1. Dans la popup de l'extension du portable, renseigner l'endpoint
   `http://didier-pc.tail2e09f2.ts.net:5173/api/vr/state` (ou l'adresse
   Tailscale du PC, `http://100.83.59.61:5173/api/vr/state`), puis
   **Enregistrer**.
2. Le manifeste autorise ces deux adresses (`host_permissions`). Pour un autre
   PC ou une autre adresse, l'ajouter au manifeste puis recharger l'extension.

## Vérifier ce que reçoit l'application

Après avoir rechargé l'extension et la page du jeu, ouvrir
`http://localhost:8080/api/vr/state` (API Go lancée) :

- `boatHints` : indices du bateau. Vide, la polaire n'est pas détectée et doit
  être choisie à la main ;
- `zones` : zones interdites relevées dans la manche ;
- `legShape` : structure de la manche (nom des champs et taille des listes). Si
  `zones` est vide alors que la course en comporte, ces champs indiquent où le jeu
  les place : les transmettre pour adapter la recherche ;
- `checkpoints` et `gateGroupCounters` : portes et groupes déjà franchis.

Les zones sont cherchées dans les listes dont le nom évoque une zone (`zone`,
`restrict`, `forbid`, `exclu`, `polygon`, `danger`, `obstacle`) et dont les
éléments portent au moins trois sommets `{lat, lon}`, des paires `[lat, lon]` ou
un polygone GeoJSON. Les portes et les limites des glaces sont écartées.

Les voiles sont déduites des règles du jeu : Jib et Spi sont toujours disponibles, `heavy` ajoute Staysail et Heavy Gennaker, `light` ajoute Light Jib et Light Gennaker, et `reach` ajoute Code 0.

L’extension ne modifie pas les requêtes du jeu et ne conserve pas les réponses complètes : elle mémorise uniquement l’état normalisé du bateau, des portes et des zones du parcours.

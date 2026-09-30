# VR State Lite

Extension Chrome/Edge Manifest V3 minimale pour afficher l'état du bateau Virtual Regatta.

Elle récupère uniquement la dernière réponse `/getboatinfos` et expose :

- position latitude/longitude ;
- stamina ;
- options et statut Full Pack ;
- liste des voiles disponibles selon les options ;
- cap, vitesse et voile ;
- portes de parcours reçues avec leurs extrémités et leur sens de franchissement ;
- course, étape, joueur et date de mise à jour.

Si Flask est démarré sur `http://localhost:5000`, chaque état est envoyé à `POST http://localhost:5000/api/vr/state`, puis relayé vers l'API Go. Le dernier état est consultable avec `GET http://localhost:5000/api/vr/state`. Les logs Flask utilisent le préfixe `[VR STATE]`.

## Installation locale

1. Ouvrir `chrome://extensions` ou `edge://extensions`.
2. Activer le mode développeur.
3. Cliquer sur **Charger l'extension non empaquetée**.
4. Sélectionner le dossier `vr-state-extension`.
5. Recharger la page Virtual Regatta.
6. Ouvrir la popup de l'extension.

Les voiles sont déduites des règles du jeu : Jib et Spi sont toujours disponibles, `heavy` ajoute Staysail et Heavy Gennaker, `light` ajoute Light Jib et Light Gennaker, et `reach` ajoute Code 0.

L’extension ne modifie pas les requêtes du jeu et ne conserve pas les réponses complètes : elle mémorise uniquement l’état normalisé du bateau et des portes du parcours.

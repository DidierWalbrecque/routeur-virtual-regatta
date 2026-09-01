# Conversations du projet — Routeur Virtual Regatta

Ce document définit les conversations à créer dans le projet ChatGPT **Routeur Virtual Regatta**.

Chaque conversation possède un périmètre précis afin d'éviter de mélanger les décisions, le code, les tests et les recherches. Les fichiers placés dans les sources du projet restent accessibles à toutes les conversations.

## Mode d'emploi

1. Créer les conversations progressivement, au moment de commencer le sujet concerné.
2. Utiliser exactement le titre proposé pour les retrouver facilement.
3. Copier le **message de démarrage** dans la nouvelle conversation.
4. Terminer les critères de validation avant de passer à la conversation suivante.
5. Reporter les décisions importantes dans `roadmap-routeur-virtual-regatta.md`.
6. Conserver le code source dans le projet Visual Studio Code et non dans plusieurs conversations différentes.

## Sources communes

- `roadmap-routeur-virtual-regatta.md`
- `conversation_phase.md`
- `ultim_bp_11.json`
- `gfs.2026042906.f003.grb2`
- tableau des coûts des équipements
- documentation officielle de Virtual Regatta sur la gestion de l'énergie

---

## Conversation 00 — Cadrage et décisions

### Phases concernées

- Phase 0 — Consolider les règles et les données
- Suivi des décisions pendant tout le projet

### Objectif

Centraliser les règles confirmées, les hypothèses, les informations manquantes et les décisions d'architecture qui concernent plusieurs parties de l'application.

### Sujets à traiter

- périmètre de la première version ;
- différences entre route théorique et route pratique ;
- règles de stamina et pénalités de manœuvre ;
- voiles gratuites et voiles achetables ;
- correspondance entre les options Light, Heavy et les voiles du JSON ;
- coûts des équipements selon la catégorie de course ;
- contraintes légales et techniques concernant Virtual Regatta ;
- décisions pouvant avoir un impact sur plusieurs modules.

### Livrables attendus

- liste des règles confirmées ;
- liste des hypothèses configurables ;
- liste des informations restant à vérifier ;
- journal des décisions importantes ;
- définition claire de la première version utile.

### Critères de validation

- chaque règle est marquée comme confirmée, supposée ou inconnue ;
- aucune valeur incertaine n'est intégrée définitivement dans le code ;
- les décisions importantes sont reportées dans la roadmap.

### Message de démarrage

> Ouvrons la conversation de cadrage du routeur Virtual Regatta. À partir de la roadmap et des fichiers du projet, construis trois listes : règles confirmées, hypothèses configurables et informations manquantes. Ne commençons pas encore le développement. Aide-moi ensuite à choisir les points indispensables pour la première version.

---

## Conversation 01 — Environnement Go et Docker

### Phase concernée

- Phase 1 — Préparer l'environnement Go et Docker

### Objectif

Créer une base de projet exécutable dans Visual Studio Code et comprendre le rôle de chaque fichier.

### Sujets à traiter

- installation et vérification de Go, Git et Docker Desktop ;
- structure des dossiers du projet ;
- initialisation du module Go ;
- premier serveur HTTP ;
- tests Go ;
- image Docker multi-stage ;
- fichier `compose.yaml` ;
- configuration Visual Studio Code ;
- commandes courantes de développement.

### Livrables attendus

- dépôt de code initialisé ;
- application Go minimale ;
- premier test automatisé ;
- `Dockerfile` ;
- `compose.yaml` ;
- fichier README contenant les commandes de lancement.

### Critères de validation

- l'application démarre directement avec Go ;
- l'application démarre également avec Docker Compose ;
- une route de contrôle répond correctement ;
- les tests passent ;
- chaque fichier créé a été expliqué.

### Message de démarrage

> Commençons la phase 1 de la roadmap. Aide-moi à créer dans Visual Studio Code la structure initiale du projet Go avec Docker Compose. Avance étape par étape, donne-moi les commandes exactes pour Windows et explique chaque fichier avant de l'utiliser. Attends ma validation après chaque étape importante.

---

## Conversation 02 — Lecture et interpolation des polaires

### Phases concernées

- Phase 2 — Définir le modèle métier
- Phase 3 — Charger et interpoler les polaires

### Objectif

Transformer le fichier de polaires en un composant Go capable de calculer la vitesse du bateau pour un vent, un angle et une voile donnés.

### Sujets à traiter

- analyse de la structure de `ultim_bp_11.json` ;
- unités et conventions utilisées ;
- structures Go représentant le bateau, les voiles et les points de polaire ;
- validation des données chargées ;
- interpolation entre deux forces de vent ;
- interpolation entre deux angles au vent ;
- sélection de la meilleure voile disponible ;
- comportement aux limites et gestion des erreurs.

### Livrables attendus

- modèle métier initial ;
- lecteur JSON ;
- fonction d'interpolation des polaires ;
- fonction de sélection de voile ;
- tests unitaires avec des valeurs connues.

### Critères de validation

- le fichier fourni est chargé sans erreur ;
- une vitesse peut être calculée pour une valeur située entre deux points ;
- le moteur ne sélectionne que les voiles possédées ;
- les résultats sont reproductibles et couverts par des tests.

### Message de démarrage

> Travaillons sur les phases 2 et 3 de la roadmap. Commence par analyser `ultim_bp_11.json` et explique-moi sa structure, ses unités et les points à vérifier. Propose ensuite les structures Go nécessaires, sans coder tout le module d'un seul coup. Nous ajouterons les tests à chaque fonction d'interpolation.

---

## Conversation 03 — Optimisation des équipements

### Phase concernée

- Phase 4 — Optimiser les équipements et le budget

### Objectif

Comparer les combinaisons de voiles et d'équipements afin de conseiller les achats respectant le plafond total de crédits de la course.

### Sujets à traiter

- saisie de la catégorie de course et du plafond total ;
- voiles de base gratuites ;
- coûts de Light, Heavy, Code 0, foils, polish et winchs pro ;
- correspondance entre équipements et effets sur les performances ;
- génération des combinaisons autorisées ;
- élimination des combinaisons dépassant le budget ;
- comparaison du temps théorique et du temps pratique ;
- recommandations avant et pendant la course.

### Livrables attendus

- modèle des équipements et tarifs ;
- configuration des coûts par catégorie ;
- générateur de combinaisons ;
- calcul du coût total ;
- classement des solutions ;
- explication lisible de chaque recommandation.

### Critères de validation

- aucune combinaison proposée ne dépasse le plafond ;
- les équipements gratuits et achetés sont distingués ;
- les options dont l'effet est inconnu sont signalées ;
- les deux classements, théorique et pratique, peuvent être comparés.

### Message de démarrage

> Travaillons sur la phase 4 de la roadmap. À partir du tableau des coûts et des règles déjà confirmées, définis d'abord le modèle de données des équipements. Distingue clairement les prix connus, les effets connus et les informations encore manquantes, notamment la correspondance des voiles Light et Heavy. Ensuite, construisons progressivement l'optimiseur sous contrainte de budget.

---

## Conversation 04 — Lecture des fichiers GRIB2

### Phase concernée

- Phase 6 — Lire et interpoler la météo GRIB2

### Objectif

Créer un composant capable d'extraire le vent du fichier NOAA GFS puis de fournir le vent attendu pour une position et une heure données.

### Sujets à traiter

- structure d'un fichier GRIB2 ;
- champs U et V du vent à 10 mètres ;
- conversion des composantes en vitesse et direction ;
- gestion des latitudes, longitudes et dates ;
- interpolation spatiale ;
- interpolation temporelle entre plusieurs prévisions ;
- gestion des longitudes autour de l'antiméridien ;
- stratégie de téléchargement et de cache des fichiers NOAA ;
- choix d'une bibliothèque Go ou d'un outil de conversion adapté.

### Livrables attendus

- lecteur météo testable ;
- fonction donnant le vent pour une position et une date ;
- représentation interne indépendante du format GRIB2 ;
- tests sur des points connus ;
- documentation des unités et conventions.

### Critères de validation

- les champs U et V du fichier d'exemple sont lus correctement ;
- la vitesse et la direction calculées sont cohérentes ;
- l'interpolation retourne une valeur pour un point entre plusieurs cellules ;
- le reste du moteur ne dépend pas directement du format GRIB2.

### Message de démarrage

> Travaillons sur la phase 6 de la roadmap avec `gfs.2026042906.f003.grb2`. Commence par m'expliquer les informations présentes dans ce fichier et les unités utilisées. Compare ensuite les solutions raisonnables pour lire du GRIB2 depuis Go, puis recommande une approche adaptée au projet avant d'écrire le code.

---

## Conversation 05 — Navigation, manœuvres et stamina

### Phases concernées

- Phase 5 — Développer les calculs de navigation
- Phase 7 — Modéliser les manœuvres et la stamina

### Objectif

Simuler le déplacement du bateau en tenant compte de sa route, du vent, des changements de voile, des virements, des empannages et de l'énergie disponible.

### Sujets à traiter

- distance et cap entre deux coordonnées ;
- projection d'une nouvelle position ;
- angle réel du vent par rapport au bateau ;
- détection d'un virement ou d'un empannage ;
- changement de voile ;
- coût énergétique des actions ;
- récupération de stamina ;
- influence de l'énergie sur la durée des manœuvres ;
- fonction simulant un pas de temps complet.

### Livrables attendus

- bibliothèque de calculs géographiques ;
- état du bateau ;
- modèle configurable de stamina ;
- moteur de pénalités ;
- fonction de simulation d'un déplacement ;
- tests unitaires et scénarios de manœuvre.

### Critères de validation

- les distances et caps sont vérifiés sur des cas simples ;
- chaque manœuvre détectée applique la règle attendue ;
- la stamina ne sort jamais de ses limites ;
- les valeurs provisoires sont configurables ;
- un scénario de plusieurs heures peut être rejoué de manière identique.

### Message de démarrage

> Travaillons sur les phases 5 et 7 de la roadmap. Commençons uniquement par les calculs géographiques indispensables : distance, cap et projection d'une position. Explique-moi les formules et écrivons leurs tests. Nous ajouterons ensuite les manœuvres, les pénalités et la stamina avec des règles configurables.

---

## Conversation 06 — Moteur de routage

### Phases concernées

- Phase 8 — Construire le moteur de routage par isochrones
- Phase 9 — Ajouter les terres et les contraintes de course
- Phase 10 — Comparer route théorique et route pratique

### Objectif

Calculer une route complète entre le départ et l'arrivée, puis produire une version théorique et une version pratique adaptée à une absence d'environ quatre heures.

### Sujets à traiter

- principe des isochrones ;
- création des positions candidates ;
- choix du pas de temps et des angles explorés ;
- élimination des routes dominées ;
- critères de rapprochement de l'arrivée ;
- reconstruction du meilleur chemin ;
- détection des collisions avec les terres ;
- portes et zones interdites ;
- pénalisation des manœuvres trop fréquentes ;
- stabilité des recommandations pendant quatre heures ;
- comparaison durée, risque, stamina et nombre de manœuvres.

### Livrables attendus

- moteur d'isochrones ;
- mécanisme de réduction des candidats ;
- prise en compte des obstacles ;
- route théorique ;
- route pratique ;
- indicateurs expliquant les différences entre les deux résultats.

### Critères de validation

- une route simple est trouvée sans obstacle ;
- le moteur ne traverse pas une terre ou une zone interdite ;
- les pénalités modifient réellement le choix de route ;
- la route pratique limite les décisions fragiles pendant la période d'absence ;
- le calcul reste suffisamment rapide pour une utilisation interactive.

### Message de démarrage

> Travaillons sur les phases 8 à 10 de la roadmap. Explique-moi d'abord le principe du routage par isochrones avec un exemple simple. Définissons ensuite les entrées, les sorties et les structures Go du moteur. Construisons une première version sans obstacle, puis ajoutons progressivement les terres, les pénalités et la route pratique de quatre heures.

---

## Conversation 07 — API et interface cartographique

### Phase concernée

- Phase 11 — Créer l'API, les workers et l'interface web

### Objectif

Rendre le moteur accessible depuis une application web et afficher les routes sur une carte interactive.

### Sujets à traiter

- contrat JSON entre le navigateur et Go ;
- saisie du départ, de l'arrivée, de la date et du budget ;
- lancement et suivi d'un calcul ;
- gestion des calculs longs avec des workers ;
- création du frontend TypeScript avec Vite ;
- carte MapLibre ;
- affichage des routes théorique et pratique ;
- affichage des points de route, vents, voiles et manœuvres ;
- comparaison des temps et équipements ;
- messages d'erreur compréhensibles.

### Livrables attendus

- API Go documentée ;
- interface de saisie ;
- carte interactive ;
- affichage des deux routes ;
- panneau comparatif ;
- lancement complet avec Docker Compose.

### Critères de validation

- un utilisateur peut saisir les informations d'une course ;
- le calcul peut être lancé depuis le navigateur ;
- les deux routes sont visibles et distinguables ;
- les résultats importants sont expliqués ;
- une erreur de saisie ou de calcul est affichée clairement.

### Message de démarrage

> Travaillons sur la phase 11 de la roadmap. Commençons par définir le contrat JSON minimal entre l'interface web et le moteur Go. Propose les endpoints nécessaires sans multiplier inutilement les services. Nous créerons ensuite l'interface TypeScript, puis l'affichage MapLibre des deux routes.

---

## Conversation 08 — Tests et performances

### Phase concernée

- Phase 12 — Tester, optimiser et ouvrir aux testeurs

### Objectif

Vérifier la fiabilité du routeur, mesurer les performances et préparer une première utilisation par deux ou trois testeurs.

### Sujets à traiter

- stratégie globale de tests ;
- tests unitaires, d'intégration et de bout en bout ;
- scénarios météo et polaires de référence ;
- comparaison avec des résultats calculés manuellement ;
- mesure du temps de calcul et de la mémoire ;
- profilage Go ;
- optimisation uniquement des véritables points lents ;
- journalisation et diagnostic des erreurs ;
- configuration simplifiée pour les testeurs ;
- collecte structurée des retours.

### Livrables attendus

- suite de tests automatisés ;
- scénarios de référence ;
- rapport de performances ;
- procédure d'installation pour les testeurs ;
- fiche de retour de test ;
- liste priorisée des anomalies et améliorations.

### Critères de validation

- les calculs essentiels sont couverts par des tests ;
- les mêmes entrées donnent les mêmes résultats ;
- le temps de calcul est mesuré avant toute optimisation ;
- une autre personne peut lancer l'application avec la documentation fournie ;
- les erreurs sont exploitables pour corriger le problème.

### Message de démarrage

> Travaillons sur la phase 12 de la roadmap. Dresse l'inventaire des fonctions critiques déjà développées et propose une stratégie de tests par niveau de risque. Construisons ensuite des scénarios de référence reproductibles avant de mesurer les performances du moteur Go.

---

## Conversation 09 — Extension Chrome et Edge

### Phase concernée

- Phase 13 — Extension et ouverture publique

### Objectif

Étudier puis développer une extension navigateur limitée aux interactions autorisées, sans fragiliser le cœur du routeur.

### Sujets à traiter

- vérification des conditions d'utilisation et autorisations de Virtual Regatta ;
- définition précise des données pouvant être récupérées ;
- saisie ou transfert de la position vers le routeur ;
- architecture Manifest V3 ;
- communication entre l'extension et l'API locale ou distante ;
- permissions minimales ;
- sécurité et protection des données ;
- publication Chrome et Edge ;
- authentification et limitation des calculs lors de l'ouverture publique ;
- possibilité d'abandonner l'intégration si elle n'est pas autorisée.

### Livrables attendus

- étude de faisabilité ;
- liste des interactions autorisées et interdites ;
- extension minimale si l'intégration est permise ;
- documentation d'installation ;
- stratégie de sécurité et de publication.

### Critères de validation

- aucune donnée n'est interceptée sans autorisation ;
- l'extension ne commande pas automatiquement le bateau dans la première version ;
- les permissions demandées sont justifiées et minimales ;
- le routeur reste utilisable manuellement sans l'extension ;
- l'ouverture publique dispose de protections contre les abus.

### Message de démarrage

> Travaillons sur la phase 13 de la roadmap. Avant tout développement, vérifions la faisabilité et les règles applicables à une extension connectée à Virtual Regatta. Séparons clairement les fonctions autorisées, incertaines et interdites. Ne produisons aucun mécanisme d'interception ou d'automatisation tant que son autorisation n'est pas confirmée.

---

## Tableau de suivi

| Conversation | État | Décision ou résultat principal |
| --- | --- | --- |
| 00 — Cadrage et décisions | En cours | Règles confirmées progressivement dans la roadmap. |
| 01 — Environnement Go et Docker | Terminée | API Go, tests et environnement Docker opérationnels. |
| 02 — Lecture et interpolation des polaires | Terminée | Chargement validé, interpolation bilinéaire, meilleure voile et équipements testés. |
| 03 — Optimisation des équipements | En cours | Modèle du budget, catégories et tarifs de référence en validation. |
| 04 — Lecture des fichiers GRIB2 | À démarrer | |
| 05 — Navigation, manœuvres et stamina | À démarrer | |
| 06 — Moteur de routage | À démarrer | |
| 07 — API et interface cartographique | À démarrer | |
| 08 — Tests et performances | À démarrer | |
| 09 — Extension Chrome et Edge | À démarrer | |

États conseillés : `À démarrer`, `En cours`, `Bloquée`, `À valider` ou `Terminée`.

## Première action recommandée

Poursuivre avec la conversation **03 — Optimisation des équipements** afin de modéliser les coûts, le budget de course et les combinaisons d'options.

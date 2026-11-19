# Analyse de l'interface SERMAR — idées à retenir, sans copier

*Analyse du 8 novembre 2026, à partir d'une capture de SERMAR essentiel ([`semar.png`](../semar.png)) : même zone que nos essais (Vieux-Fort vers Pointe-à-Pitre en Ultim).*

L'objectif n'est pas de reproduire cette interface, mais d'en tirer des **principes** à réaliser à notre façon. Voir aussi [l'audit de notre interface](audit-interface.md).

---

## Ce qu'elle fait très bien

1. **Le résultat d'abord.** En haut à droite, quatre chiffres suffisent : arrivée, durée, distance et manœuvres. Chez nous, ces informations sont rangées dans le panneau Détails.
2. **« Prochains ordres ».** La route est traduite en **actions à faire dans le jeu** (« 23:39 TWA 57° T », « 23:49 TWA 56° T »). C'est la meilleure idée de l'écran : pour un joueur de Virtual Regatta, c'est ce qui compte vraiment.
3. **Une frise temporelle en bas**, avec heure, cap et vitesse à l'instant choisi, pour rejouer la route.
4. **La terre se distingue nettement de la mer.** Terre gris clair, mer sombre, et la route jaune ressort parfaitement.
5. **Des repères lisibles.** S et F pour départ et arrivée, portes numérotées 1 et 2, cercle pointillé du rayon d'arrivée.
6. **Le contexte de la course en tête :** nom, trajet Saint-Malo → Pointe-à-Pitre et bateau avec son coefficient.
7. **Comparaison avec une route enregistrée.**

## Ce qui l'alourdit

- **Bordures vert fluo partout :** chaque panneau crie, et rien ne ressort plus que le reste.
- **Trop d'îlots flottants :** barre latérale, menu du haut, deux cartes à droite, boutons à droite, frise en bas. La carte n'occupe qu'à peine la moitié de l'écran.
- **Barres d'icônes énigmatiques :** « Actions route » et « Actions carto » ne disent pas ce que fait chaque bouton.
- **Couleurs sans légende :** route jaune, bleue et blanche, sans explication. La zone interdite est **verte**, une couleur qui évoque habituellement le « permis ».
- **Le logo et le bouton « Soutenir »** prennent la meilleure place, en haut à gauche, au détriment de l'outil.

## Ce que notre interface a déjà en mieux

- **Isochrones en direct** pendant le calcul, et annulation du routage.
- **Stamina, pénalités et manœuvres** simulées à chaque pas, avec leur détail.
- **Transparence sur la météo :** run utilisé, bascule vers un run plus récent, avertissements.
- **Un panneau plus sobre** depuis la refonte du formulaire, avec une seule couleur d'accent.

---

## Ce qu'on en retient pour notre routeur

Ce sont des idées à réaliser à notre façon, sans en copier la forme :

| # | Idée | Pourquoi | Effort |
|---|---|---|---|
| 1 | **Ordres à donner dans le jeu** : liste « à HH:MM, régler TWA ou cap X, voile Y » tirée de la route | la plus utile pour jouer ; les données existent déjà (cap, TWA, voile, manœuvres à chaque pas) | moyen |
| 2 | **Carte de résultat compacte** après le calcul : arrivée, durée, distance, manœuvres, stamina finale | répond tout de suite à « combien de temps ? » | faible |
| 3 | **Frise temporelle** pour rejouer la route et voir l'état du bateau à chaque instant | lien entre carte et tableau | moyen |
| 4 | **Repères plus lisibles** : S et A, portes numérotées dans l'ordre, cercle du rayon d'arrivée | répond aussi au manque de légende relevé dans l'audit | faible |
| 5 | **Meilleur contraste terre/mer** sur le fond de nuit | aujourd'hui, les îles se distinguent mal de la mer assombrie | faible à moyen |
| 6 | **Enregistrer une route et la comparer** à un nouveau calcul | utile pour évaluer un changement de réglage | moyen |

**Ordre conseillé :** commencer par les points 1 et 2, qui apportent le plus à un joueur et s'appuient sur des données déjà calculées.

**À éviter :** les bordures fluo, la multiplication des panneaux flottants et les barres d'icônes sans libellé. Notre interface gagne à rester centrée sur la carte, avec un seul panneau à la fois et un seul accent de couleur, le jaune.

---

## Remarque sur la route

Leur route fait 3 h 10 avec 3 manœuvres. Notre essai pendant l'audit faisait 3 h 45 avec 2 manœuvres, mais avec un autre départ (un peu plus à l'ouest), une autre heure, une marge côtière à 0 et la zone AGOA désactivée. Les deux ne sont donc pas comparables tels quels.

Leur route **part de la côte sans blocage**. Au moment de cette analyse, la nôtre échouait dans ce cas avec la marge de 2 NM (point P1 de [l'audit](audit-interface.md#3-calcul-impossible-près-de-la-côte-p1)). C'est corrigé depuis le 9 novembre : le moteur laisse un bateau déjà dans la marge côtière s'éloigner de la côte (commit `2da0422`), et le cas a été vérifié sur le vrai backend.

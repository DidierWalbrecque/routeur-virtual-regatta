# CLAUDE.md

## Graphify : carte du code

Une carte du code est générée dans `graphify-out/`.

- Pour une question d'architecture (où se trouve X, comment Y est relié à Z, qui appelle W), lire d'abord `graphify-out/GRAPH_REPORT.md`.
- Pour les liens entre deux éléments, utiliser `python -m graphify query "<question>"`, `python -m graphify path "<A>" "<B>"` ou `python -m graphify explain "<concept>"` (la commande `graphify` seule n'est pas dans le PATH).
- Ne pas lire `graphify-out/graph.json` en entier : il est trop volumineux.
- La carte peut être en retard sur les dernières modifications : vérifier dans le code avant de modifier quoi que ce soit.
- Les liens marqués INFERRED sont des déductions : les vérifier avant de s'en appuyer.
- Les tests sont exclus de la carte (voir `.graphifyignore`).

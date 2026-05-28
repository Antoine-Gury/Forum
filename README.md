# Forum WoW (Go)

Petit forum minimal en Go utilisant `html/template` et assets statiques.

## Exécution

Lancer le serveur localement:

```bash
go run .
```

Le serveur écoute sur `http://localhost:8080`.

## Fichiers importants

- `main.go`: point d'entrée, démarre le serveur
- `models.go`: définitions `Thread` et `Post`
- `store.go`: logique de stockage en mémoire
- `handlers.go`: gestion des routes et templates
- `templates/` et `static/`: assets front-end
# Forum WoW

Application de forum communautaire développée en Go, avec authentification via Supabase (Auth + PostgreSQL) et mise à jour en temps réel des discussions via Server-Sent Events (SSE).

## Description

Le projet propose un forum minimaliste sur le thème *World of Warcraft* : inscription et connexion des utilisateurs, création et consultation de discussions, profil personnel, et diffusion en direct des nouveaux messages à tous les visiteurs connectés.

## Stack technique

- Backend : Go 1.25, `net/http` (sans framework)
- Base de données : PostgreSQL via pgx/v5 (pool de connexions)
- Authentification : Supabase Auth (API REST)
- Templates : `html/template` (Go natif)
- Frontend : HTML / CSS / JavaScript natif, Server-Sent Events

## Fonctionnalités

- Inscription, connexion, déconnexion et confirmation par email via Supabase Auth
- Gestion de session par cookies HttpOnly (`sb-access-token` / `sb-refresh-token`) avec rafraîchissement automatique du token expiré
- Page de profil affichant l'email, le pseudo et l'identifiant Supabase de l'utilisateur
- Création et consultation de discussions, avec page de détail dédiée
- Diffusion en temps réel des nouvelles discussions à tous les clients connectés (`/events`), sans rechargement de page
- Persistance double : écriture prioritaire en PostgreSQL via `pgx`, avec bascule automatique sur l'API REST de Supabase si la base n'est pas joignable
- Compatibilité de schéma : la couche Supabase REST gère indifféremment les colonnes `title/author/content` ou `titre/auteur/contenu`

## Structure du projet

```
Forum/
├── main.go                  # Point d'entrée, routes HTTP, chargement du .env
├── go.mod / go.sum          # Dépendances Go
├── src/
│   ├── go/                  # Package "handlers" (logique serveur)
│   │   ├── handlers.go      #   Pages, discussions, SSE
│   │   ├── auth.go          #   Cookies de session, récupération de l'utilisateur connecté
│   │   ├── login.go         #   Handler de connexion
│   │   ├── register.go      #   Handler d'inscription
│   │   ├── logout.go        #   Handler de déconnexion
│   │   ├── db.go            #   Connexion PostgreSQL, requêtes, création des tables
│   │   └── supabase.go      #   Appels REST à l'API Supabase (Auth + données)
│   └── js/                  # Scripts frontend
│       ├── button.js        #   Navigation entre les boutons du menu
│       ├── live.js          #   Écoute des événements SSE et mise à jour du DOM
│       └── script.js
├── templates/                # Vues HTML (Go templates)
│   ├── index.html            #   Accueil / liste des discussions
│   ├── Login.html             #   Connexion
│   ├── register.html         #   Inscription
│   ├── create.html           #   Création d'une discussion
│   ├── discussion.html       #   Détail d'une discussion
│   ├── profil.html           #   Profil utilisateur
│   └── auth_callback.html    #   Page intermédiaire de confirmation email
└── assets/
    ├── css/                  # Feuilles de style par page
    └── Picture/              # Images et icônes
```

## Prérequis

- Go ≥ 1.25 (https://go.dev/dl/)
- Un projet Supabase (https://supabase.com/) pour l'authentification
- Une base PostgreSQL accessible (optionnel ; celle fournie par Supabase fonctionne directement)

## Installation

1. Cloner le dépôt

   ```bash
   git clone https://github.com/Antoine-Gury/Forum.git
   cd Forum
   ```

2. Installer les dépendances

   ```bash
   go mod download
   ```

3. Configurer les variables d'environnement

   Créer un fichier `.env` à la racine du projet (chargé automatiquement au démarrage) :

   ```
   SUPABASE_URL=https://votre-projet.supabase.co
   SUPABASE_ANON_KEY=eyJ...
   SUPABASE_DB_URL=postgres://user:password@host:port/database
   PORT=8080
   ```

   | Variable                   | Description                                                            | Obligatoire |
   |-----------------------------|--------------------------------------------------------------------------|:-----------:|
   | SUPABASE_URL                | URL du projet Supabase                                                   | recommandé |
   | SUPABASE_ANON_KEY           | Clé publique (anon) de l'API Supabase                                    | oui |
   | SUPABASE_SERVICE_ROLE_KEY   | Clé service-role, utilisée à la place de la clé anon si présente         | non |
   | SUPABASE_DB_URL             | Chaîne de connexion PostgreSQL (active la persistance directe en base)   | non |
   | PORT                        | Port d'écoute du serveur HTTP (défaut : 8080)                             | non |

   Si `SUPABASE_DB_URL` n'est pas définie, le serveur démarre quand même et bascule automatiquement sur l'API REST de Supabase pour lire et écrire les discussions et les profils.

4. Lancer le serveur

   ```bash
   go run .
   ```

   Le serveur affiche l'URL d'écoute dans la console (par défaut `http://localhost:8081`).

## Routes principales

| Méthode  | Route               | Description                                                |
|----------|----------------------|--------------------------------------------------------------|
| GET      | /                    | Accueil, liste des discussions                                |
| GET      | /login               | Page de connexion                                             |
| POST     | /auth/login          | Traitement du formulaire de connexion                          |
| GET      | /register            | Page d'inscription                                            |
| POST     | /auth/register       | Traitement du formulaire d'inscription                         |
| GET      | /logout              | Déconnexion (suppression des cookies de session)               |
| GET      | /profil              | Profil de l'utilisateur connecté (redirige vers /login sinon)  |
| GET/POST | /create              | Formulaire et création d'une nouvelle discussion                |
| GET      | /discussion?id=...   | Détail d'une discussion                                        |
| GET      | /events              | Flux SSE pour les mises à jour en temps réel                   |
| GET      | /auth/callback       | Retour de confirmation d'email / échange de code Supabase      |
| POST     | /debug/insert        | Endpoint de test pour insérer une discussion (développement)   |

## Fonctionnement interne

- Sessions : à chaque requête authentifiée, `getAuthenticatedUser` vérifie le cookie `sb-access-token`. S'il est expiré, le `sb-refresh-token` est utilisé pour obtenir un nouveau token auprès de Supabase, qui est ensuite réinjecté dans les cookies.
- Discussions : `GetDiscussionsFromDB` et `InsertDiscussion` utilisent le pool PostgreSQL si disponible (`InitDB` réussi), sinon ils basculent sur les fonctions équivalentes de `supabase.go`, qui passent par l'API REST `/rest/v1/discussions`.
- Temps réel : chaque nouvelle discussion créée est diffusée (`broadcastNewDiscussion`) à tous les clients connectés au flux `/events`, qui l'ajoutent dynamiquement au DOM côté client (`live.js`).
- Initialisation de la base : au démarrage, `InitDB` crée automatiquement les tables `discussions`, `profiles` et `password_recovery_requests` si elles n'existent pas encore.

## Sécurité

- Ne jamais committer le fichier `.env` ni les clés Supabase dans le dépôt. Ajouter un fichier `.gitignore` contenant au minimum :

  ```
  .env
  *.exe
  *.log
  ```

- Si des secrets ont déjà été poussés sur GitHub par erreur, les révoquer et les régénérer depuis le tableau de bord Supabase, puis les purger de l'historique Git.
- Les routes de type `/debug/insert` sont prévues pour le développement uniquement ; les retirer ou les protéger avant un déploiement public.

## Pistes d'amélioration

- Pagination des discussions
- Système de réponses/commentaires par discussion
- Modération (suppression, édition) réservée aux auteurs ou administrateurs
- Tests automatisés des handlers Go

## Licence

Aucune licence n'est définie pour ce projet à ce jour.
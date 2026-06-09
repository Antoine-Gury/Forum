# Forum simple en Go

Structure minimale d'un site de forum avec authentification Supabase.

## Exécution

Avant de lancer le serveur, définissez vos variables d'environnement Supabase :

```bash
set SUPABASE_URL=https://votre-projet.supabase.co
set SUPABASE_ANON_KEY=eyJ...
set SUPABASE_DB_URL=postgres://user:password@host:port/database
```

Ou créez un fichier `.env` à la racine du projet. Le serveur va le charger automatiquement au démarrage.

Vous pouvez utiliser `.env.example` comme modèle pour vos valeurs.

Puis lancez :

```bash
go run .
```

Ouvrez `http://localhost:8080`

## Routes principales

- `/` : accueil
- `/login` : page de connexion
- `/register` : page d'inscription
- `/forgot` : mot de passe oublié
- `/profil` : page de profil protégé

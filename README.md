# Song Battle CLI

Une application CLI interactive pour comparer et classer vos chansons Spotify préférées en utilisant un système de rating Elo.

## 🎯 Fonctionnalités

- **Duels de chansons** : Comparez deux chansons et choisissez votre préférée
- **Système Elo** : Algorithme de classement adaptatif basé sur vos votes
- **Intégration Spotify** : Authentification OAuth2, lecture audio, gestion des playlists
- **Interface TUI** : Interface terminal interactive avec Bubble Tea
- **Matchmaking intelligent** : Sélection équilibrée des paires basée sur l'Elo
- **Export de playlists** : Créez des playlists Spotify avec vos meilleurs titres
- **Audio features** : Visualisez les caractéristiques audio de vos chansons

## 📋 Prérequis

- **Go 1.22+**
- **Compte Spotify Premium** (pour la lecture audio)
- **Application Spotify** créée sur [Spotify for Developers](https://developer.spotify.com/dashboard)

## 🚀 Installation

### 1. Cloner et compiler

```bash
git clone <votre-repo>
cd songbattle-cli
go mod tidy
go build ./cmd/song-battle
```

### 2. Créer une application Spotify

1. Allez sur https://developer.spotify.com/dashboard
2. Cliquez sur "Create App"
3. Remplissez les informations :
   - **App Name** : Song Battle
   - **App Description** : Application de duel de chansons
   - **Redirect URI** : Choisir selon votre situation :
     - **Recommandé (2025+)** : `songbattle://callback` (Custom scheme sécurisé)
     - **Alternative** : `https://localhost:8080/callback` (HTTPS)
     - **Fallback** : `http://localhost:8080/callback` (anciennes apps seulement)
   - **API/SDKs** : Cochez "Web API"
4. **📅 Important - Nouvelles exigences Spotify (2025)** :
   - Apps créées après avril 2025 : **DOIVENT** utiliser `songbattle://callback`
   - Apps existantes : migration recommandée avant novembre 2025
   - `http://localhost` sera progressivement déprécié
5. Sauvegardez et notez votre **Client ID**

### 3. Configuration des scopes

Dans les paramètres de votre app Spotify, assurez-vous que ces scopes sont activés :
- `user-read-playbook-state`
- `user-modify-playbook-state`  
- `user-read-currently-playing`
- `playlist-modify-private`
- `user-top-read`

## 🎵 Utilisation

### Première utilisation - Import des données

```bash
# Importer vos top tracks Spotify
./song-battle -import -client-id=VOTRE_CLIENT_ID

# Ou avec une variable d'environnement
export SPOTIFY_CLIENT_ID=votre_client_id
./song-battle -import
```

### Lancer l'application

```bash
# Lancer l'interface de duel
./song-battle -client-id=VOTRE_CLIENT_ID

# Ou avec la variable d'environnement
./song-battle
```

## 🎮 Contrôles

| Touche | Action |
|--------|--------|
| `←` `→` | Naviguer entre les chansons |
| `Espace` | Écouter la chanson sélectionnée |
| `Entrée` | Voter pour la chanson sélectionnée |
| `S` | Passer le duel (skip) |
| `T` | Afficher les caractéristiques audio |
| `G` | Ouvrir la chanson dans Spotify |
| `P` | Exporter une playlist des meilleurs titres |
| `Q` | Quitter l'application |

## 🏗️ Architecture

```
cmd/song-battle/        # Point d'entrée principal
internal/
├── auth/              # Authentification OAuth2 PKCE
├── core/              # Logique métier réutilisable  
├── elo/               # Système de rating Elo
├── export/            # Export de playlists Spotify
├── matchmaker/        # Algorithme de sélection des paires
├── models/            # Structures de données
├── spotify/           # Client API Spotify
├── store/             # Persistance SQLite
└── ui/                # Interface utilisateur Bubble Tea
configs/               # Fichiers de configuration
```

## 📊 Système Elo

Le système utilise l'algorithme Elo adaptatif :

- **Elo initial** : 1200 points
- **Facteur K adaptatif** :
  - Nouveaux titres (< 10 duels) : K = 32
  - Titres intermédiaires (10-30 duels) : K = 24  
  - Titres expérimentés (> 30 duels) : K = 16

**Formule** :
```
E_A = 1 / (1 + 10^((Elo_B - Elo_A)/400))
Elo_A' = Elo_A + K * (S_A - E_A)
```

## 🎯 Matchmaking

L'algorithme de sélection des paires privilégie :

- **Matchs équilibrés** : Différence d'Elo ≤ 100 points
- **Exploration** : 15% des duels incluent un titre peu joué
- **Variété** : Évite les adversaires récents

## 🔒 Sécurité et URI de redirection (IMPORTANT - Nouvelle politique Spotify)

⚠️ **ATTENTION : Spotify applique de nouvelles validations depuis avril 2025**
- Les nouvelles apps créées après le 9 avril 2025 doivent utiliser des URI sécurisés
- Migration obligatoire pour toutes les apps avant novembre 2025
- `http://localhost` ne sera plus accepté pour les nouvelles apps

### Solutions conformes aux nouvelles exigences

**Option 1 - Custom URI Scheme (Recommandé pour apps desktop) :**
```bash
# Dans Spotify Dashboard, utilisez :
songbattle://callback

# L'application gérera automatiquement ce scheme
```

**Option 2 - HTTPS avec certificat local :**
```bash
# Générer un certificat auto-signé
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes -subj "/CN=localhost"

# URI à utiliser : https://localhost:8080/callback
```

**Option 3 - Serveur de développement sécurisé :**
```bash
# Utiliser un domaine de test HTTPS
# URI : https://votre-domaine-test.com/callback
```

## �🔧 Options de ligne de commande

```bash
songbattle [OPTIONS]

OPTIONS:
    -client-id string    Client ID Spotify (requis)
    -db-path string      Chemin base de données (défaut: ~/.songbattle/songbattle.db)
    -import              Mode import des données Spotify
    -version             Afficher la version
    -help                Afficher l'aide
```

## 📁 Fichiers de données

- **Base de données** : `~/.songbattle/songbattle.db`
- **Configuration** : `configs/config.yaml`

## 🔮 Roadmap Future (Version Web)

Cette CLI est conçue pour une migration future vers une application web :

- **Architecture modulaire** : Logique métier séparée de l'interface
- **API REST** : Endpoints pour exposer les fonctionnalités core
- **Base de données partagée** : Même SQLite réutilisé
- **Authentification compatible** : OAuth2 transposable au web

## �️ Configuration avancée des URI de redirection

L'application propose plusieurs méthodes pour gérer les URI de redirection selon vos besoins :

### Détection automatique (Recommandé)
```bash
# L'app choisit automatiquement le meilleur URI disponible
./songbattle -client-id=VOTRE_CLIENT_ID
```

### Forcer un type spécifique
```bash
# Utiliser le schéma personnalisé (conforme 2025)
./songbattle -use-custom-scheme -client-id=VOTRE_CLIENT_ID

# Utiliser HTTPS sur localhost
./songbattle -use-https -client-id=VOTRE_CLIENT_ID

# URI personnalisé spécifique
./songbattle -redirect-uri=songbattle://callback -client-id=VOTRE_CLIENT_ID
```

## 🐛 Dépannage

### Problèmes d'authentification

**❌ "This redirect URI is not secure" (Avertissement Spotify)**
```bash
# Solution 1 : Utiliser le schéma personnalisé (recommandé)
./songbattle -use-custom-scheme -client-id=VOTRE_CLIENT_ID

# Solution 2 : Ajouter l'URI sécurisé dans votre app Spotify
# Dans Spotify Dashboard > Settings > Redirect URIs, ajoutez :
# songbattle://callback
```

**❌ "New validation will start from November XX, 2025"**
```bash
# Migration nécessaire avant novembre 2025
# Utilisez le schéma personnalisé ou HTTPS :
./songbattle -use-custom-scheme -client-id=VOTRE_CLIENT_ID
./songbattle -use-https -client-id=VOTRE_CLIENT_ID
```

**❌ "invalid_client" ou "invalid_redirect_uri"**
1. Vérifiez que l'URI est configuré dans votre app Spotify
2. Utilisez la détection automatique : `./songbattle -client-id=VOTRE_CLIENT_ID`
3. Si le problème persiste, essayez un URI spécifique :
   ```bash
   ./songbattle -redirect-uri=http://localhost:8080/callback -client-id=VOTRE_CLIENT_ID
   ```

**❌ "PKCE verification failed"**
```bash
# Supprimez le cache d'authentification et recommencez
rm ~/.songbattle/songbattle.db
./song-battle -import -client-id=VOTRE_CLIENT_ID
```

### Problèmes de lecture audio

**🔇 Pas de son / "No active device found"**
1. Assurez-vous d'avoir **Spotify Premium**
2. Ouvrez l'application Spotify (desktop, mobile, ou web)
3. Lancez une chanson pour activer un appareil
4. Relancez Song Battle

**🔇 "Premium required for playback control"**
- Spotify Premium est obligatoire pour contrôler la lecture
- Les comptes gratuits peuvent voir les métadonnées mais pas lire la musique

### Problèmes de données

**📊 "Aucune données disponibles pour les duels"**
```bash
# Importez d'abord vos données Spotify
./song-battle -import -client-id=VOTRE_CLIENT_ID

# Si l'import échoue, vérifiez les scopes de votre app
# user-top-read doit être activé dans Spotify Dashboard
```

**💾 Base de données corrompue**
```bash
# Supprimer et recréer la base de données
rm ~/.songbattle/songbattle.db
./song-battle -import -client-id=VOTRE_CLIENT_ID
```

**📂 Problèmes de permissions sur les fichiers**
```bash
# Créer manuellement le répertoire et ajuster les permissions
mkdir -p ~/.songbattle
chmod 755 ~/.songbattle
./song-battle -import -client-id=VOTRE_CLIENT_ID
```

### Problèmes de réseau

**🌐 "Connection timeout" ou erreurs réseau**
```bash
# Vérifiez votre connexion internet et les proxies
# Testez avec curl :
curl -I https://api.spotify.com/v1/me
```

**🔒 Problèmes de proxy/firewall**
```bash
# Si vous êtes derrière un proxy d'entreprise :
export HTTP_PROXY=http://votre-proxy:port
export HTTPS_PROXY=http://votre-proxy:port
./song-battle -client-id=VOTRE_CLIENT_ID
```

### Messages d'erreur détaillés

**🏃‍♂️ Mode verbose pour diagnostiquer**
```bash
# Activez les logs détaillés (ajoutez cette variable d'environnement)
export SONGBATTLE_DEBUG=true
./song-battle -client-id=VOTRE_CLIENT_ID
```

### Configuration alternative avec variables d'environnement

```bash
# Définir le Client ID de façon permanente
echo 'export SPOTIFY_CLIENT_ID=votre_client_id' >> ~/.zshrc
source ~/.zshrc

# Utiliser une base de données alternative
export SONGBATTLE_DB_PATH=/path/to/your/db.sqlite
./song-battle
```

### ⚙️ Script de diagnostic automatique

Un script de demo est inclus pour tester différentes configurations :

```bash
# Rendre le script exécutable et le lancer
chmod +x demo.sh
./demo.sh
```

### 🆘 Obtenir de l'aide

Si les solutions ci-dessus ne résolvent pas votre problème :

1. **Vérifiez les logs** : L'application affiche des messages d'erreur détaillés
2. **Testez la connectivité** : Assurez-vous de pouvoir accéder à `https://accounts.spotify.com`
3. **Vérifiez votre app Spotify** : Confirmer les URI de redirection et les scopes
4. **Essayez le mode verbose** : `export SONGBATTLE_DEBUG=true`

Pour signaler un bug, incluez :
- Votre système d'exploitation
- La commande exacte utilisée
- Le message d'erreur complet
- La configuration de votre app Spotify (sans révéler le Client ID)

## 📝 Développement

### Compiler

```bash
make build
```

### Tests

```bash
make test
```

### Lancer en mode développement

```bash
make run
```

## 📄 License

MIT License - Voir le fichier LICENSE pour les détails.

## 🙏 Crédits

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - Framework TUI
- [Spotify Web API](https://developer.spotify.com/documentation/web-api/) - API musicale
- [go-sqlite3](https://github.com/mattn/go-sqlite3) - Driver SQLite
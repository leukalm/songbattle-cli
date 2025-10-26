# 🔒 Song Battle CLI - Mise à jour de sécurité Spotify 2025

## 📅 Contexte - Nouvelle politique Spotify

Spotify a annoncé de nouvelles exigences de sécurité pour les URI de redirection :

- **Avril 2025** : Nouvelles apps créées ne peuvent plus utiliser `http://localhost`
- **Novembre 2025** : Toutes les apps existantes doivent migrer vers des URI sécurisés
- **Solutions conformes** : Custom URI schemes (ex: `songbattle://`) ou HTTPS

## 🚀 Nouvelles fonctionnalités

### 1. Détection automatique d'URI
L'application détecte automatiquement le meilleur URI de redirection :
```bash
# Mode automatique (recommandé)
./songbattle -client-id=VOTRE_CLIENT_ID
```

**Ordre de priorité :**
1. `songbattle://callback` (custom scheme sécurisé)
2. `https://localhost:8080/callback` (HTTPS, si certificats disponibles)
3. `http://localhost:8080/callback` (fallback pour anciennes apps)

### 2. Options de forçage d'URI

**Forcer le custom scheme :**
```bash
./songbattle -use-custom-scheme -client-id=VOTRE_CLIENT_ID
```

**Forcer HTTPS :**
```bash
./songbattle -use-https -client-id=VOTRE_CLIENT_ID
```

**URI spécifique :**
```bash
./songbattle -redirect-uri=myapp://callback -client-id=VOTRE_CLIENT_ID
```

### 3. Mode debug verbose

Activez les logs détaillés pour diagnostiquer les problèmes :
```bash
export SONGBATTLE_DEBUG=1
./songbattle -client-id=VOTRE_CLIENT_ID
```

**Informations affichées en mode debug :**
- Détection d'URI et logique de sélection
- Chargement/sauvegarde des tokens
- Tentatives d'authentification et refresh
- URLs d'authentification générées
- Messages d'erreur détaillés

## 🎯 Configuration Spotify Developer Dashboard

### URIs recommandés (ajoutez tous pour compatibilité maximale)

✅ **Conforme 2025+ (Prioritaire) :**
```
songbattle://callback
```

✅ **Alternative HTTPS :**
```
https://localhost:8080/callback
```

⚠️ **Fallback (Déprécié en 2025) :**
```
http://localhost:8080/callback
```

### Scopes requis
Dans les paramètres de votre app, activez ces permissions :
- `user-read-playback-state`
- `user-modify-playbook-state`
- `user-read-currently-playing`
- `playlist-modify-private`
- `user-top-read`

## 🛠️ Exemples d'utilisation

### Configuration recommandée (2025+)
```bash
# 1. Créer app Spotify avec songbattle://callback
# 2. Import avec custom scheme
./songbattle -import -use-custom-scheme -client-id=ABC123

# 3. Utilisation normale avec détection auto
./songbattle -client-id=ABC123
```

### Migration d'app existante
```bash
# 1. Ajouter songbattle://callback dans Spotify Dashboard
# 2. Tester avec le nouveau scheme
./songbattle -use-custom-scheme -client-id=EXISTING_ID

# 3. Si ça marche, c'est maintenant le défaut automatique
./songbattle -client-id=EXISTING_ID
```

### Développement et tests
```bash
# Test avec debug pour voir la détection
SONGBATTLE_DEBUG=1 ./songbattle -client-id=TEST_ID

# Test de différents URI
./test-uri.sh

# Demo interactive
./demo.sh
```

## 🐛 Troubleshooting avancé

### "This redirect URI is not secure"
**Cause :** Utilisation de `http://localhost` avec une app récente

**Solutions :**
1. **Préférée :** Utiliser custom scheme
   ```bash
   ./songbattle -use-custom-scheme -client-id=VOTRE_ID
   ```

2. **Alternative :** Générer certificat HTTPS
   ```bash
   openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes -subj "/CN=localhost"
   ./songbattle -use-https -client-id=VOTRE_ID
   ```

### "New validation will start from November"
**Cause :** App existante qui doit migrer avant novembre 2025

**Action :** Migrer vers custom scheme
1. Ajouter `songbattle://callback` dans votre app Spotify
2. Tester : `./songbattle -use-custom-scheme -client-id=VOTRE_ID`
3. Une fois validé, le mode automatique utilisera le custom scheme

### Debug de la détection d'URI
```bash
# Voir quelle URI est choisie et pourquoi
SONGBATTLE_DEBUG=1 ./songbattle -client-id=TEST_ID

# Exemple de sortie debug :
# 🐛 [DEBUG] Début de détection automatique d'URI
# 🐛 [DEBUG] Vérification du support custom scheme: toujours supporté (true)
# 🐛 [DEBUG] Custom scheme supporté, utilisation de songbattle://callback
# 🔒 Utilisation du custom scheme sécurisé: songbattle://callback
```

## 📋 Checklist de migration

- [ ] **Créer/modifier app Spotify** : Ajouter `songbattle://callback`
- [ ] **Tester custom scheme** : `./songbattle -use-custom-scheme -client-id=ID`
- [ ] **Vérifier détection auto** : `./songbattle -client-id=ID` (doit choisir custom scheme)
- [ ] **Test complet** : Import + duels + export playlist
- [ ] **Documentation équipe** : Partager nouvelles commandes
- [ ] **CI/CD** : Mettre à jour scripts de déploiement si nécessaire

## 🎉 Avantages des custom schemes

✅ **Conformité 2025+** : Répond aux nouvelles exigences Spotify
✅ **Sécurité** : Pas d'exposition de ports web locaux
✅ **Simplicité** : Pas besoin de certificats HTTPS
✅ **Compatibilité** : Fonctionne sur tous les OS modernes
✅ **Future-proof** : Solution recommandée par Spotify

## 📞 Support

En cas de problème :
1. **Activez le debug** : `export SONGBATTLE_DEBUG=1`
2. **Testez les scripts** : `./test-uri.sh` et `./demo.sh`
3. **Vérifiez la config Spotify** : URI et scopes correctement configurés
4. **Consultez le troubleshooting** : Section complète dans README.md

La migration vers les custom schemes garantit la compatibilité future avec Spotify et améliore la sécurité de l'application.
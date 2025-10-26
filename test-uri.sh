#!/bin/bash

# Test URI Detection - Song Battle CLI
# Ce script teste la détection automatique d'URI et simule différents scénarios

echo "🔍 Test de détection d'URI - Song Battle CLI"
echo "============================================="
echo ""

CLIENT_ID="${SPOTIFY_CLIENT_ID:-demo_client_id}"

# Test 1: Mode automatique
echo "📋 Test 1: Détection automatique d'URI"
echo "Commande: ./songbattle -client-id=$CLIENT_ID"
echo "URI attendu: Détection automatique (songbattle:// ou https://localhost:8080)"
echo ""

# Test 2: Forcer custom scheme
echo "📋 Test 2: Schéma personnalisé forcé"
echo "Commande: ./songbattle -use-custom-scheme -client-id=$CLIENT_ID"
echo "URI attendu: songbattle://callback"
echo ""

# Test 3: Forcer HTTPS
echo "📋 Test 3: HTTPS forcé"
echo "Commande: ./songbattle -use-https -client-id=$CLIENT_ID"
echo "URI attendu: https://localhost:8080/callback"
echo ""

# Test 4: URI spécifique
echo "📋 Test 4: URI personnalisé"
echo "Commande: ./songbattle -redirect-uri=myapp://auth -client-id=$CLIENT_ID"
echo "URI attendu: myapp://auth"
echo ""

# Informations sur la configuration Spotify
echo "🎯 Configuration dans Spotify Developer Dashboard:"
echo "=================================================="
echo ""
echo "URI à ajouter dans 'Redirect URIs' (cochez tous pour compatibilité) :"
echo "✓ songbattle://callback         (Recommandé - Conforme 2025)"
echo "✓ https://localhost:8080/callback   (Alternative HTTPS)"
echo "⚠ http://localhost:8080/callback    (Fallback - Déprécié en 2025)"
echo ""
echo "Scopes requis dans 'User permissions' :"
echo "✓ user-read-playback-state"
echo "✓ user-modify-playbook-state"
echo "✓ user-read-currently-playing"
echo "✓ playlist-modify-private"
echo "✓ user-top-read"
echo ""

echo "🚀 Pour tester avec votre Client ID réel:"
echo "export SPOTIFY_CLIENT_ID=votre_client_id"
echo "./test-uri.sh"
echo ""

# Si le Client ID est défini, proposer un test réel
if [ "$SPOTIFY_CLIENT_ID" != "" ] && [ "$SPOTIFY_CLIENT_ID" != "demo_client_id" ]; then
    echo "🎵 Client ID détecté: ${SPOTIFY_CLIENT_ID:0:8}..."
    read -p "Voulez-vous tester la détection d'URI maintenant ? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Test de détection automatique..."
        if [ -f "./songbattle" ]; then
            echo "Lancement: ./songbattle -help"
            ./songbattle -help
        else
            echo "❌ Binaire './songbattle' non trouvé. Compilez d'abord avec 'go build ./cmd/song-battle'"
        fi
    fi
else
    echo "💡 Définissez SPOTIFY_CLIENT_ID pour tester avec votre app réelle"
fi
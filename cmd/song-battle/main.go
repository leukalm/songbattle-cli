package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"songbattle/internal/auth"
	"songbattle/internal/models"
	"songbattle/internal/spotify"
	"songbattle/internal/store"
	"songbattle/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	spotifyapi "github.com/zmb3/spotify/v2"
)

const (
	AppName         = "Song Battle"
	AppVersion      = "1.0.0"
	DBName          = "songbattle.db"
	DefaultClientID = "c0bf7a0584f544dbb3e6fc14dce4716c" // Client ID public par défaut (à définir après déploiement)
)

func main() {
	// Configuration des flags
	var (
		clientID    = flag.String("client-id", "", "Spotify Client ID (requis)")
		redirectURI = flag.String("redirect-uri", "", "URI de redirection (défaut: détection automatique)")
		useCustom   = flag.Bool("use-custom-scheme", false, "Force l'utilisation du schéma personnalisé 'songbattle://'")
		useHTTPS    = flag.Bool("use-https", false, "Force l'utilisation de HTTPS sur localhost:8080")
		dbPath      = flag.String("db-path", getDefaultDBPath(), "Chemin vers la base de données SQLite")
		importData  = flag.Bool("import", false, "Importer des données depuis Spotify")
		showHelp    = flag.Bool("help", false, "Afficher l'aide")
		version     = flag.Bool("version", false, "Afficher la version")
	)
	flag.Parse()

	// Afficher la version
	if *version {
		fmt.Printf("%s v%s\n", AppName, AppVersion)
		return
	}

	// Afficher l'aide
	if *showHelp {
		showUsage()
		return
	}

	// Initialiser la base de données
	db, err := store.NewDB(*dbPath)
	if err != nil {
		log.Fatalf("❌ Erreur initialisation base de données: %v", err)
	}
	defer db.Close()

	// Vérifier le Client ID - ordre de priorité:
	// 1. Flag -client-id
	// 2. Variable d'environnement
	// 3. Valeur sauvegardée en DB
	// 4. Client ID par défaut (si défini)
	if *clientID == "" {
		if envClientID := os.Getenv("SPOTIFY_CLIENT_ID"); envClientID != "" {
			*clientID = envClientID
		} else if savedClientID, err := db.GetMeta("spotify_client_id"); err == nil && savedClientID != "" {
			*clientID = savedClientID
			fmt.Println("✓ Client ID récupéré depuis la configuration")
		} else if DefaultClientID != "" {
			*clientID = DefaultClientID
			fmt.Println("ℹ️  Utilisation du Client ID par défaut")
			fmt.Println("   Vous pouvez utiliser votre propre Client ID avec -client-id=VOTRE_CLIENT_ID")
		}
	}

	// Si toujours pas de Client ID, afficher l'erreur
	if *clientID == "" {
		fmt.Println("❌ Erreur: Client ID Spotify requis")
		fmt.Println("Utilisez -client-id=VOTRE_CLIENT_ID ou définissez la variable d'environnement SPOTIFY_CLIENT_ID")
		showUsage()
		os.Exit(1)
	}

	// Sauvegarder le client ID pour les prochaines fois
	if err := db.SetMeta("spotify_client_id", *clientID); err != nil {
		// Non bloquant, juste un warning
		fmt.Printf("⚠️  Impossible de sauvegarder le client ID: %v\n", err)
	}

	// Mode import de données explicite
	if *importData {
		if err := runImportMode(db, *clientID, *redirectURI, *useCustom, *useHTTPS); err != nil {
			log.Fatalf("❌ Erreur import données: %v", err)
		}
		fmt.Println("\n🎵 Lancement des duels...")
	}

	// Vérifier qu'on a des données pour les duels
	tracks, err := db.GetAllTracksWithRatings()
	if err != nil {
		log.Fatalf("❌ Erreur vérification données: %v", err)
	}

	// Si pas assez de tracks, lancer l'import automatiquement
	if len(tracks) < 2 {
		fmt.Printf("📥 Aucune chanson détectée (%d tracks)\n", len(tracks))
		fmt.Println("🔄 Import automatique de vos top tracks Spotify...\n")

		if err := runImportMode(db, *clientID, *redirectURI, *useCustom, *useHTTPS); err != nil {
			log.Fatalf("❌ Erreur import automatique: %v", err)
		}

		fmt.Println("\n🎵 Lancement des duels...")
	}

	// Lancer l'interface TUI
	if err := runTUI(db, *clientID, *redirectURI, *useCustom, *useHTTPS); err != nil {
		log.Fatalf("❌ Erreur interface: %v", err)
	}
}

// runTUI lance l'interface utilisateur Bubble Tea
func runTUI(db *store.DB, clientID, redirectURI string, useCustom, useHTTPS bool) error {
	// Créer le modèle avec les options d'URI
	model := ui.NewModelWithOptions(db, clientID, redirectURI, useCustom, useHTTPS)

	// Options du programme
	opts := []tea.ProgramOption{
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	}

	// Créer et lancer le programme
	program := tea.NewProgram(model, opts...)

	fmt.Printf("🎵 Lancement de %s v%s...\n", AppName, AppVersion)

	if _, err := program.Run(); err != nil {
		return fmt.Errorf("erreur lancement TUI: %w", err)
	}

	return nil
}

// runImportMode lance le mode import de données
func runImportMode(db *store.DB, clientID, redirectURI string, useCustom, useHTTPS bool) error {
	ctx := context.Background()

	fmt.Printf("🎵 %s - Import de données v%s\n", AppName, AppVersion)
	fmt.Println("════════════════════════════════════════")

	// Initialiser l'authentification avec les options d'URI
	auth := auth.NewSpotifyAuthWithOptions(clientID, db, redirectURI, useCustom, useHTTPS)

	fmt.Println("🔐 Authentification Spotify...")
	token, err := auth.GetValidToken(ctx)
	if err != nil {
		return fmt.Errorf("erreur authentification: %w", err)
	}

	// Créer le client Spotify
	spotifyClient := spotify.NewClient(ctx, token, clientID)

	// Importer les top tracks de l'utilisateur
	fmt.Println("📥 Import des top tracks...")
	if err := importUserTopTracks(db, spotifyClient); err != nil {
		return fmt.Errorf("erreur import top tracks: %w", err)
	}

	// Importer quelques recommandations (non bloquant)
	fmt.Println("🎲 Import de recommandations...")
	if err := importRecommendations(db, spotifyClient); err != nil {
		fmt.Printf("   ⚠️  Impossible d'importer les recommandations: %v\n", err)
		fmt.Println("   → Ce n'est pas grave, vous avez déjà assez de tracks pour jouer !")
	}

	fmt.Println("✅ Import terminé avec succès !")
	fmt.Printf("Vous pouvez maintenant lancer: songbattle -client-id=%s\n", clientID)

	return nil
}

// importUserTopTracks importe les top tracks de l'utilisateur
func importUserTopTracks(db *store.DB, client *spotify.Client) error {
	// Importer top tracks à court terme
	shortTermTracks, err := client.GetUserTopTracks(25, spotifyapi.ShortTermRange)
	if err != nil {
		fmt.Printf("⚠️  Erreur top tracks court terme: %v\n", err)
	} else {
		if err := saveTracks(db, shortTermTracks, client); err != nil {
			return err
		}
		fmt.Printf("   ✓ %d tracks court terme importés\n", len(shortTermTracks))
	}

	// Importer top tracks à moyen terme
	mediumTermTracks, err := client.GetUserTopTracks(25, spotifyapi.MediumTermRange)
	if err != nil {
		fmt.Printf("⚠️  Erreur top tracks moyen terme: %v\n", err)
	} else {
		if err := saveTracks(db, mediumTermTracks, client); err != nil {
			return err
		}
		fmt.Printf("   ✓ %d tracks moyen terme importés\n", len(mediumTermTracks))
	}

	// Importer top tracks à long terme
	longTermTracks, err := client.GetUserTopTracks(25, spotifyapi.LongTermRange)
	if err != nil {
		fmt.Printf("⚠️  Erreur top tracks long terme: %v\n", err)
	} else {
		if err := saveTracks(db, longTermTracks, client); err != nil {
			return err
		}
		fmt.Printf("   ✓ %d tracks long terme importés\n", len(longTermTracks))
	}

	return nil
}

// importRecommendations importe des recommandations basées sur les tracks existants
func importRecommendations(db *store.DB, client *spotify.Client) error {
	// Récupérer quelques tracks existants comme seeds
	existingTracks, err := db.GetTopTracks(5)
	if err != nil || len(existingTracks) == 0 {
		fmt.Println("   ⚠️  Pas de tracks existants pour les recommandations")
		return nil
	}

	// Utiliser les IDs Spotify comme seeds
	seeds := make([]string, 0, len(existingTracks))
	for _, track := range existingTracks {
		seeds = append(seeds, track.Track.SpotifyID)
	}

	// Obtenir des recommandations
	recommendations, err := client.GetRecommendations(seeds[:min(2, len(seeds))], []string{}, []string{}, 20)
	if err != nil {
		return err
	}

	if err := saveTracks(db, recommendations, client); err != nil {
		return err
	}

	fmt.Printf("   ✓ %d recommandations importées\n", len(recommendations))
	return nil
}

// saveTracks sauvegarde une liste de tracks en base
func saveTracks(db *store.DB, tracks []*models.Track, client *spotify.Client) error {
	for _, track := range tracks {
		// Vérifier si le track existe déjà
		if existing, _ := db.GetTrackBySpotifyID(track.SpotifyID); existing != nil {
			continue // Skip si déjà existant
		}

		// Enrichir avec les audio features
		if err := client.EnrichTrackWithAudioFeatures(track); err != nil {
			fmt.Printf("   ⚠️  Impossible d'enrichir %s: %v\n", track.Name, err)
		}

		// Sauvegarder en base
		if err := db.CreateTrack(track); err != nil {
			return fmt.Errorf("erreur sauvegarde track %s: %w", track.Name, err)
		}
	}

	return nil
}

// getDefaultDBPath retourne le chemin par défaut de la base de données
func getDefaultDBPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return DBName
	}

	configDir := filepath.Join(homeDir, ".songbattle")
	os.MkdirAll(configDir, 0755)

	return filepath.Join(configDir, DBName)
}

// showUsage affiche l'aide d'utilisation
func showUsage() {
	fmt.Printf(`🎵 %s v%s - Duel de chansons avec système Elo

USAGE:
    songbattle [OPTIONS]

OPTIONS:
    -client-id string       Client ID de votre application Spotify (requis)
    -db-path string         Chemin vers la base de données SQLite (défaut: ~/.songbattle/songbattle.db)
    -import                 Mode import: récupère vos top tracks Spotify
    -redirect-uri string    URI de redirection personnalisé (défaut: détection automatique)
    -use-custom-scheme      Force l'utilisation du schéma personnalisé 'songbattle://'
    -use-https              Force l'utilisation de HTTPS sur localhost:8080
    -version                Affiche la version
    -help                   Affiche cette aide

PREMIÈRE UTILISATION:
    1. Créez une application Spotify sur https://developer.spotify.com/dashboard
    2. Configurez l'URI de redirection (IMPORTANT - Spotify 2025):
       Ajoutez dans votre app Spotify: http://127.0.0.1:8080/callback

       ⚠️  ATTENTION: Spotify n'accepte plus "localhost", utilisez 127.0.0.1

    3. Définissez votre Client ID:
       export SPOTIFY_CLIENT_ID=VOTRE_CLIENT_ID

    4. Lancez l'application (l'import se fait automatiquement si besoin):
       ./songbattle

    Note: Le Client ID est sauvegardé après la première utilisation, vous n'aurez plus
    besoin de le fournir lors des prochains lancements.

CONFIGURATION URI DE REDIRECTION:
    - Par défaut (recommandé pour 2025):
      http://127.0.0.1:8080/callback
      → Ajoutez cet URI exact dans votre Spotify Dashboard

    - Options avancées:
      • Custom scheme: songbattle -use-custom-scheme (nécessite config OS)
      • HTTPS: songbattle -use-https (nécessite certificat)
      • URI personnalisé: songbattle -redirect-uri=VOTRE_URI

VARIABLES D'ENVIRONNEMENT:
    SPOTIFY_CLIENT_ID    Client ID Spotify (alternative au flag -client-id)

CONTRÔLES DANS L'APPLICATION:
    ←/→     Naviguer entre les chansons
    Espace  Écouter la chanson sélectionnée
    Entrée  Voter pour la chanson sélectionnée
    S       Passer le duel
    T       Voir les caractéristiques audio
    G       Ouvrir dans Spotify
    P       Exporter une playlist des meilleurs titres
    Q       Quitter

PRÉREQUIS:
    - Compte Spotify Premium (pour la lecture audio)
    - Application Spotify ouverte et connectée (recommandé)

`, AppName, AppVersion)
}

// min retourne le minimum de deux entiers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

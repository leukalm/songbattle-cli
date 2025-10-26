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
	AppVersion      = "1.0.1"
	DBName          = "songbattle.db"
	DefaultClientID = "c0bf7a0584f544dbb3e6fc14dce4716c" // Public default Client ID
)

func main() {
	// Flag configuration
	var (
		clientID    = flag.String("client-id", "", "Spotify Client ID (required)")
		redirectURI = flag.String("redirect-uri", "", "Redirect URI (default: auto-detect)")
		useCustom   = flag.Bool("use-custom-scheme", false, "Force custom scheme 'songbattle://'")
		useHTTPS    = flag.Bool("use-https", false, "Force HTTPS on localhost:8080")
		dbPath      = flag.String("db-path", getDefaultDBPath(), "SQLite database path")
		importData  = flag.Bool("import", false, "Import data from Spotify")
		showHelp    = flag.Bool("help", false, "Show help")
		version     = flag.Bool("version", false, "Show version")
	)
	flag.Parse()

	// Show version
	if *version {
		fmt.Printf("%s v%s\n", AppName, AppVersion)
		return
	}

	// Show help
	if *showHelp {
		showUsage()
		return
	}

	// Initialize database
	db, err := store.NewDB(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Check Client ID - priority order:
	// 1. -client-id flag
	// 2. Environment variable
	// 3. Saved value in DB
	// 4. Default Client ID (if set)
	if *clientID == "" {
		if envClientID := os.Getenv("SPOTIFY_CLIENT_ID"); envClientID != "" {
			*clientID = envClientID
		} else if savedClientID, err := db.GetMeta("spotify_client_id"); err == nil && savedClientID != "" {
			*clientID = savedClientID
			fmt.Println("✓ Using saved Client ID from configuration")
		} else if DefaultClientID != "" {
			*clientID = DefaultClientID
			fmt.Println("ℹ️  Using default Client ID")
			fmt.Println("   You can use your own with -client-id=YOUR_CLIENT_ID")
		}
	}

	// Still no Client ID, show error
	if *clientID == "" {
		fmt.Println("Error: Spotify Client ID required")
		fmt.Println("Use -client-id=YOUR_CLIENT_ID or set SPOTIFY_CLIENT_ID environment variable")
		showUsage()
		os.Exit(1)
	}

	// Save Client ID for next time
	if err := db.SetMeta("spotify_client_id", *clientID); err != nil {
		// Non-blocking, just a warning
		fmt.Printf("⚠️  Failed to save Client ID: %v\n", err)
	}

	// Explicit import mode
	if *importData {
		if err := runImportMode(db, *clientID, *redirectURI, *useCustom, *useHTTPS); err != nil {
			log.Fatalf("Failed to import data: %v", err)
		}
		fmt.Println("\n🎵 Starting battles...")
	}

	// Check if we have data for battles
	tracks, err := db.GetAllTracksWithRatings()
	if err != nil {
		log.Fatalf("Failed to check data: %v", err)
	}

	// Not enough tracks, auto-import
	if len(tracks) < 2 {
		fmt.Printf("📥 No songs detected (%d tracks)\n", len(tracks))
		fmt.Println("🔄 Auto-importing your Spotify top tracks...\n")

		if err := runImportMode(db, *clientID, *redirectURI, *useCustom, *useHTTPS); err != nil {
			log.Fatalf("Failed to auto-import: %v", err)
		}

		fmt.Println("\n🎵 Starting battles...")
	}

	// Launch TUI
	if err := runTUI(db, *clientID, *redirectURI, *useCustom, *useHTTPS); err != nil {
		log.Fatalf("Failed to start UI: %v", err)
	}
}

// runTUI launches the Bubble Tea user interface
func runTUI(db *store.DB, clientID, redirectURI string, useCustom, useHTTPS bool) error {
	// Create model with URI options
	model := ui.NewModelWithOptions(db, clientID, redirectURI, useCustom, useHTTPS)

	// Program options
	opts := []tea.ProgramOption{
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	}

	// Create and launch program
	program := tea.NewProgram(model, opts...)

	fmt.Printf("🎵 Starting %s v%s...\n", AppName, AppVersion)

	if _, err := program.Run(); err != nil {
		return fmt.Errorf("failed to start TUI: %w", err)
	}

	return nil
}

// runImportMode runs the data import mode
func runImportMode(db *store.DB, clientID, redirectURI string, useCustom, useHTTPS bool) error {
	ctx := context.Background()

	fmt.Printf("🎵 %s - Data Import v%s\n", AppName, AppVersion)
	fmt.Println("════════════════════════════════════════")

	// Initialize authentication with URI options
	auth := auth.NewSpotifyAuthWithOptions(clientID, db, redirectURI, useCustom, useHTTPS)

	fmt.Println("🔐 Authenticating with Spotify...")
	token, err := auth.GetValidToken(ctx)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Create Spotify client
	spotifyClient := spotify.NewClient(ctx, token, clientID)

	// Import user's top tracks
	fmt.Println("📥 Importing top tracks...")
	if err := importUserTopTracks(db, spotifyClient); err != nil {
		return fmt.Errorf("failed to import top tracks: %w", err)
	}

	// Import recommendations (non-blocking)
	fmt.Println("🎲 Importing recommendations...")
	if err := importRecommendations(db, spotifyClient); err != nil {
		fmt.Printf("   ⚠️  Failed to import recommendations: %v\n", err)
		fmt.Println("   → No worries, you have enough tracks to play!")
	}

	fmt.Println("✅ Import completed successfully!")
	fmt.Printf("You can now run: songbattle -client-id=%s\n", clientID)

	return nil
}

// importUserTopTracks imports user's top tracks
func importUserTopTracks(db *store.DB, client *spotify.Client) error {
	// Import short term top tracks
	shortTermTracks, err := client.GetUserTopTracks(25, spotifyapi.ShortTermRange)
	if err != nil {
		fmt.Printf("⚠️  Failed to get short term tracks: %v\n", err)
	} else {
		if err := saveTracks(db, shortTermTracks, client); err != nil {
			return err
		}
		fmt.Printf("   ✓ %d short term tracks imported\n", len(shortTermTracks))
	}

	// Import medium term top tracks
	mediumTermTracks, err := client.GetUserTopTracks(25, spotifyapi.MediumTermRange)
	if err != nil {
		fmt.Printf("⚠️  Failed to get medium term tracks: %v\n", err)
	} else {
		if err := saveTracks(db, mediumTermTracks, client); err != nil {
			return err
		}
		fmt.Printf("   ✓ %d medium term tracks imported\n", len(mediumTermTracks))
	}

	// Import long term top tracks
	longTermTracks, err := client.GetUserTopTracks(25, spotifyapi.LongTermRange)
	if err != nil {
		fmt.Printf("⚠️  Failed to get long term tracks: %v\n", err)
	} else {
		if err := saveTracks(db, longTermTracks, client); err != nil {
			return err
		}
		fmt.Printf("   ✓ %d long term tracks imported\n", len(longTermTracks))
	}

	return nil
}

// importRecommendations imports recommendations based on existing tracks
func importRecommendations(db *store.DB, client *spotify.Client) error {
	// Get some existing tracks as seeds
	existingTracks, err := db.GetTopTracks(5)
	if err != nil || len(existingTracks) == 0 {
		fmt.Println("   ⚠️  No existing tracks for recommendations")
		return nil
	}

	// Use Spotify IDs as seeds
	seeds := make([]string, 0, len(existingTracks))
	for _, track := range existingTracks {
		seeds = append(seeds, track.Track.SpotifyID)
	}

	// Get recommendations
	recommendations, err := client.GetRecommendations(seeds[:min(2, len(seeds))], []string{}, []string{}, 20)
	if err != nil {
		return err
	}

	if err := saveTracks(db, recommendations, client); err != nil {
		return err
	}

	fmt.Printf("   ✓ %d recommendations imported\n", len(recommendations))
	return nil
}

// saveTracks saves a list of tracks to database
func saveTracks(db *store.DB, tracks []*models.Track, client *spotify.Client) error {
	for _, track := range tracks {
		// Check if track already exists
		if existing, _ := db.GetTrackBySpotifyID(track.SpotifyID); existing != nil {
			continue // Skip if already exists
		}

		// Enrich with audio features
		if err := client.EnrichTrackWithAudioFeatures(track); err != nil {
			fmt.Printf("   ⚠️  Failed to enrich %s: %v\n", track.Name, err)
		}

		// Save to database
		if err := db.CreateTrack(track); err != nil {
			return fmt.Errorf("failed to save track %s: %w", track.Name, err)
		}
	}

	return nil
}

// getDefaultDBPath returns the default database path
func getDefaultDBPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return DBName
	}

	configDir := filepath.Join(homeDir, ".songbattle")
	os.MkdirAll(configDir, 0755)

	return filepath.Join(configDir, DBName)
}

// showUsage displays usage help
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

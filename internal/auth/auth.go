package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"songbattle/internal/models"
	"songbattle/internal/store"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/browser"
	"golang.org/x/oauth2"
)

const (
	SpotifyAuthURL    = "https://accounts.spotify.com/authorize"
	SpotifyTokenURL   = "https://accounts.spotify.com/api/token"
	RedirectURI       = "http://127.0.0.1:8080/callback"  // Conforme Spotify 2025 (127.0.0.1 requis)
	SecureRedirectURI = "songbattle://callback"           // Custom scheme (requires OS config)
	HTTPSRedirectURI  = "https://localhost:8080/callback" // HTTPS alternative
	CallbackPort      = ":8080"
	CustomSchemePort  = ":8081" // Alternative port for custom scheme
)

var RequiredScopes = []string{
	"user-read-playback-state",
	"user-modify-playback-state",
	"user-read-currently-playing",
	"playlist-modify-private",
	"user-top-read",
}

type SpotifyAuth struct {
	ClientID        string
	config          *oauth2.Config
	db              *store.DB
	redirectURI     string // Automatically detected redirect URI
	useCustomScheme bool   // Uses custom scheme or HTTP(S)
}

// NewSpotifyAuth creates a new Spotify authentication instance
func NewSpotifyAuth(clientID string, db *store.DB) *SpotifyAuth {
	// Automatic detection of the best redirect URI
	redirectURI, useCustomScheme := detectBestRedirectURI()

	return newSpotifyAuthWithOptions(clientID, db, redirectURI, useCustomScheme)
}

// NewSpotifyAuthWithOptions creates a new instance with specific options
func NewSpotifyAuthWithOptions(clientID string, db *store.DB, customRedirectURI string, forceCustom, forceHTTPS bool) *SpotifyAuth {
	if customRedirectURI != "" {
		// Specific URI provided
		return newSpotifyAuthWithOptions(clientID, db, customRedirectURI, strings.HasPrefix(customRedirectURI, "songbattle://"))
	}

	if forceCustom {
		// Force custom scheme
		return newSpotifyAuthWithOptions(clientID, db, SecureRedirectURI, true)
	}

	if forceHTTPS {
		// Force HTTPS
		return newSpotifyAuthWithOptions(clientID, db, HTTPSRedirectURI, false)
	}

	// Automatic detection
	redirectURI, useCustomScheme := detectBestRedirectURI()
	return newSpotifyAuthWithOptions(clientID, db, redirectURI, useCustomScheme)
}

// newSpotifyAuthWithOptions internal function to create the instance
func newSpotifyAuthWithOptions(clientID string, db *store.DB, redirectURI string, useCustomScheme bool) *SpotifyAuth {

	config := &oauth2.Config{
		ClientID:    clientID,
		RedirectURL: redirectURI,
		Scopes:      RequiredScopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  SpotifyAuthURL,
			TokenURL: SpotifyTokenURL,
		},
	}

	return &SpotifyAuth{
		ClientID:        clientID,
		config:          config,
		db:              db,
		redirectURI:     redirectURI,
		useCustomScheme: useCustomScheme,
	}
}

// isDebugEnabled checks if debug mode is enabled
func isDebugEnabled() bool {
	return os.Getenv("SONGBATTLE_DEBUG") != ""
}

// debugLog displays a debug message if debug mode is enabled
func debugLog(msg string, args ...interface{}) {
	if isDebugEnabled() {
		fmt.Printf("🐛 [DEBUG] "+msg+"\n", args...)
	}
}

// detectBestRedirectURI automatically detects the best redirect URI
func detectBestRedirectURI() (string, bool) {
	debugLog("Starting automatic URI detection")

	// Priority 1: Custom scheme (compliant with Spotify 2025 requirements)
	if isCustomSchemeSupported() {
		fmt.Println("🔒 Using secure custom scheme: songbattle://callback")
		debugLog("Custom scheme supported, using %s", SecureRedirectURI)
		return SecureRedirectURI, true
	}

	// Priority 2: HTTPS localhost (if certificate available)
	if isHTTPSLocalhostAvailable() {
		fmt.Println("🔒 Using HTTPS localhost: https://localhost:8080/callback")
		debugLog("HTTPS localhost available, using %s", HTTPSRedirectURI)
		return HTTPSRedirectURI, false
	}

	// Fallback: HTTP localhost (for legacy apps only)
	fmt.Println("⚠️  Using HTTP localhost (not compliant with new Spotify requirements)")
	fmt.Println("   Consider migrating to songbattle://callback for 2025 compliance")
	debugLog("Fallback to HTTP localhost: %s", RedirectURI)
	return RedirectURI, false
}

// isCustomSchemeSupported checks if custom scheme can be used
func isCustomSchemeSupported() bool {
	// Custom scheme requires specific OS configuration
	// For better UX, we use HTTP 127.0.0.1 by default (Spotify 2025 compliant)
	// Advanced users can force custom scheme with -use-custom-scheme
	debugLog("Checking custom scheme support: disabled by default (false)")
	return false
}

// isHTTPSLocalhostAvailable checks if HTTPS localhost is available
func isHTTPSLocalhostAvailable() bool {
	// Check if certificates are available
	// For now, return false to keep things simple
	debugLog("Checking HTTPS localhost: not available (false)")
	return false
}

// generateCodeVerifier generates a code verifier for PKCE
func generateCodeVerifier() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(bytes), nil
}

// generateCodeChallenge generates a code challenge from the verifier
func generateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(hash[:])
}

// Authenticate lance le processus d'authentification OAuth2 avec PKCE
func (sa *SpotifyAuth) Authenticate(ctx context.Context) (*oauth2.Token, error) {
	// Generate PKCE codes
	codeVerifier, err := generateCodeVerifier()
	if err != nil {
		return nil, fmt.Errorf("code verifier generation error: %w", err)
	}
	codeChallenge := generateCodeChallenge(codeVerifier)

	// Canal pour recevoir le code d'autorisation
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	// Configuration du serveur selon le type d'URI
	var server *http.Server
	var port string

	if sa.useCustomScheme {
		port = CustomSchemePort
		server = &http.Server{Addr: port}
	} else {
		port = CallbackPort
		server = &http.Server{Addr: port}
	}

	// Configuration du handler selon le type d'URI
	if sa.useCustomScheme {
		// Handler for custom scheme - listens on all paths
		http.HandleFunc("/", sa.handleCustomSchemeCallback(codeChan, errChan))
	} else {
		// Handler classique pour HTTP(S)
		http.HandleFunc("/callback", sa.handleHTTPCallback(codeChan, errChan))
	}

	// Launch server in background
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("erreur serveur callback: %w", err)
		}
	}()

	// Construire l'URL d'autorisation avec PKCE
	authURL := sa.config.AuthCodeURL("state",
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"))

	fmt.Println("🎵 Spotify authentication required")
	if sa.useCustomScheme {
		fmt.Println("🔒 Using secure mode (Custom Scheme)")
		fmt.Printf("🌐 Listening on: localhost%s\n", port)
	} else {
		fmt.Printf("🌐 Listening on: localhost%s\n", port)
	}
	fmt.Println("Opening your browser...")
	fmt.Printf("If it doesn't work, copy this URL: %s\n", authURL)

	// Open browser
	if err := browser.OpenURL(authURL); err != nil {
		fmt.Printf("Failed to open browser: %v\n", err)
		fmt.Printf("Please open manually: %s\n", authURL)
	}

	// Attendre le code ou une erreur
	var code string
	select {
	case code = <-codeChan:
		// Code received
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("timeout authentification")
	}

	// Fermer le serveur
	server.Shutdown(context.Background())

	// Exchange code for token with PKCE
	token, err := sa.exchangeCodeForToken(code, codeVerifier)
	if err != nil {
		return nil, fmt.Errorf("code/token exchange error: %w", err)
	}

	// Sauvegarder le token
	if err := sa.SaveToken(token); err != nil {
		return nil, fmt.Errorf("erreur sauvegarde token: %w", err)
	}

	return token, nil
}

// exchangeCodeForToken exchanges authorization code for access token
func (sa *SpotifyAuth) exchangeCodeForToken(code, codeVerifier string) (*oauth2.Token, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", RedirectURI)
	data.Set("client_id", sa.ClientID)
	data.Set("code_verifier", codeVerifier)

	ctx := context.Background()
	return sa.config.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", codeVerifier))
}

// SaveToken saves the token to database
func (sa *SpotifyAuth) SaveToken(token *oauth2.Token) error {
	if err := sa.db.SetMeta(models.MetaKeyAccessToken, token.AccessToken); err != nil {
		return err
	}

	if token.RefreshToken != "" {
		if err := sa.db.SetMeta(models.MetaKeyRefreshToken, token.RefreshToken); err != nil {
			return err
		}
	}

	if !token.Expiry.IsZero() {
		expiryStr := strconv.FormatInt(token.Expiry.Unix(), 10)
		if err := sa.db.SetMeta(models.MetaKeyTokenExpiry, expiryStr); err != nil {
			return err
		}
	}

	return nil
}

// LoadToken loads the token from database
func (sa *SpotifyAuth) LoadToken() (*oauth2.Token, error) {
	accessToken, err := sa.db.GetMeta(models.MetaKeyAccessToken)
	if err != nil {
		return nil, fmt.Errorf("no access token found: %w", err)
	}

	token := &oauth2.Token{AccessToken: accessToken}

	// Refresh token (optionnel)
	if refreshToken, err := sa.db.GetMeta(models.MetaKeyRefreshToken); err == nil {
		token.RefreshToken = refreshToken
	}

	// Expiry (optionnel)
	if expiryStr, err := sa.db.GetMeta(models.MetaKeyTokenExpiry); err == nil {
		if expiryUnix, err := strconv.ParseInt(expiryStr, 10, 64); err == nil {
			token.Expiry = time.Unix(expiryUnix, 0)
		}
	}

	return token, nil
}

// RefreshToken renews the access token
func (sa *SpotifyAuth) RefreshToken(ctx context.Context, token *oauth2.Token) (*oauth2.Token, error) {
	tokenSource := sa.config.TokenSource(ctx, token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("erreur renouvellement token: %w", err)
	}

	// Sauvegarder le nouveau token
	if err := sa.SaveToken(newToken); err != nil {
		return nil, fmt.Errorf("erreur sauvegarde nouveau token: %w", err)
	}

	return newToken, nil
}

// IsTokenValid checks if the token is still valid
func (sa *SpotifyAuth) IsTokenValid(token *oauth2.Token) bool {
	if token == nil || token.AccessToken == "" {
		return false
	}

	// If no expiry, consider as valid
	if token.Expiry.IsZero() {
		return true
	}

	// Check if token expires in the next 5 minutes
	return time.Now().Add(5 * time.Minute).Before(token.Expiry)
}

// GetValidToken retrieves a valid token (loads or renews if necessary)
func (sa *SpotifyAuth) GetValidToken(ctx context.Context) (*oauth2.Token, error) {
	debugLog("Starting GetValidToken - configured URI: %s", sa.redirectURI)

	// Tenter de charger le token existant
	token, err := sa.LoadToken()
	if err != nil {
		debugLog("No existing token found, new authentication required: %v", err)
		// Aucun token, authentification requise
		return sa.Authenticate(ctx)
	}

	// Vérifier si le token est valide
	if sa.IsTokenValid(token) {
		debugLog("Existing token valid, reusing")
		return token, nil
	}

	debugLog("Token expired, attempting refresh")
	// Token expiré, tenter de le renouveler
	if token.RefreshToken != "" {
		newToken, err := sa.RefreshToken(ctx, token)
		if err == nil {
			debugLog("Token refreshed successfully")
			return newToken, nil
		}
		debugLog("Token refresh failed: %v", err)
		// Si le refresh échoue, on continue vers une nouvelle authentification
	}

	// Authentification complète requise
	return sa.Authenticate(ctx)
}

// handleHTTPCallback gère les callbacks HTTP/HTTPS classiques
func (sa *SpotifyAuth) handleHTTPCallback(codeChan chan string, errChan chan error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no authorization code received")
			return
		}

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `
			<html>
			<head><title>Song Battle - Authentification réussie</title></head>
			<body style="font-family: Arial, sans-serif; text-align: center; padding: 50px;">
				<h1>🎵 Authentification réussie!</h1>
				<p>Vous pouvez maintenant fermer cette fenêtre et retourner au terminal.</p>
				<script>
					// Tenter de fermer automatiquement l'onglet
					setTimeout(function() { window.close(); }, 2000);
				</script>
			</body>
			</html>
		`)

		codeChan <- code
	}
}

// handleCustomSchemeCallback gère les callbacks de custom scheme
func (sa *SpotifyAuth) handleCustomSchemeCallback(codeChan chan string, errChan chan error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Pour les custom schemes, Spotify redirigera vers songbattle://callback?code=...
		// Mais l'OS peut rediriger vers http://localhost:8081/?code=...
		code := r.URL.Query().Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no authorization code received via custom scheme")
			return
		}

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `
			<html>
			<head><title>Song Battle - Authentification réussie (Sécurisée)</title></head>
			<body style="font-family: Arial, sans-serif; text-align: center; padding: 50px;">
				<h1>🔒 Authentification sécurisée réussie!</h1>
				<p>Custom scheme utilisé - Conforme aux nouvelles exigences Spotify 2025</p>
				<p>Vous pouvez fermer cette fenêtre et retourner au terminal.</p>
				<script>
					setTimeout(function() { window.close(); }, 1500);
				</script>
			</body>
			</html>
		`)

		codeChan <- code
	}
}

// Logout supprime les tokens stockés
func (sa *SpotifyAuth) Logout() error {
	if err := sa.db.DeleteMeta(models.MetaKeyAccessToken); err != nil {
		return err
	}
	if err := sa.db.DeleteMeta(models.MetaKeyRefreshToken); err != nil {
		return err
	}
	if err := sa.db.DeleteMeta(models.MetaKeyTokenExpiry); err != nil {
		return err
	}
	return nil
}

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
	SpotifyAuthURL        = "https://accounts.spotify.com/authorize"
	SpotifyTokenURL       = "https://accounts.spotify.com/api/token"
	RedirectURI           = "http://127.0.0.1:8080/callback"          // Conforme Spotify 2025 (127.0.0.1 requis)
	SecureRedirectURI     = "songbattle://callback"                   // Custom scheme (nécessite config OS)
	HTTPSRedirectURI      = "https://127.0.0.1:8080/callback"         // Alternative HTTPS
	CallbackPort          = ":8080"
	CustomSchemePort      = ":8081" // Port alternatif pour custom scheme
)

var RequiredScopes = []string{
	"user-read-playback-state",
	"user-modify-playback-state",
	"user-read-currently-playing",
	"playlist-modify-private",
	"user-top-read",
}

type SpotifyAuth struct {
	ClientID      string
	clientSecret  string // Optionnel pour PKCE
	config        *oauth2.Config
	db            *store.DB
	redirectURI   string // URI de redirection détecté automatiquement
	useCustomScheme bool // Utilise custom scheme ou HTTP(S)
}

// NewSpotifyAuth crée une nouvelle instance d'authentification Spotify
func NewSpotifyAuth(clientID string, db *store.DB) *SpotifyAuth {
	// Détection automatique du meilleur URI de redirection
	redirectURI, useCustomScheme := detectBestRedirectURI()
	
	return newSpotifyAuthWithOptions(clientID, db, redirectURI, useCustomScheme)
}

// NewSpotifyAuthWithOptions crée une nouvelle instance avec des options spécifiques
func NewSpotifyAuthWithOptions(clientID string, db *store.DB, customRedirectURI string, forceCustom, forceHTTPS bool) *SpotifyAuth {
	var redirectURI string
	var useCustomScheme bool
	
	if customRedirectURI != "" {
		// URI spécifique fourni
		redirectURI = customRedirectURI
		useCustomScheme = strings.HasPrefix(redirectURI, "songbattle://")
	} else if forceCustom {
		// Forcer le schéma personnalisé
		redirectURI = SecureRedirectURI
		useCustomScheme = true
	} else if forceHTTPS {
		// Forcer HTTPS
		redirectURI = HTTPSRedirectURI
		useCustomScheme = false
	} else {
		// Détection automatique
		redirectURI, useCustomScheme = detectBestRedirectURI()
	}
	
	return newSpotifyAuthWithOptions(clientID, db, redirectURI, useCustomScheme)
}

// newSpotifyAuthWithOptions fonction interne pour créer l'instance
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

// isDebugEnabled vérifie si le mode debug est activé
func isDebugEnabled() bool {
	return os.Getenv("SONGBATTLE_DEBUG") != ""
}

// debugLog affiche un message de debug si le mode debug est activé
func debugLog(msg string, args ...interface{}) {
	if isDebugEnabled() {
		fmt.Printf("🐛 [DEBUG] "+msg+"\n", args...)
	}
}

// detectBestRedirectURI détecte automatiquement le meilleur URI de redirection
func detectBestRedirectURI() (string, bool) {
	debugLog("Début de détection automatique d'URI")
	
	// Priorité 1: Custom scheme (conforme aux nouvelles exigences Spotify 2025)
	if isCustomSchemeSupported() {
		fmt.Println("🔒 Utilisation du custom scheme sécurisé: songbattle://callback")
		debugLog("Custom scheme supporté, utilisation de %s", SecureRedirectURI)
		return SecureRedirectURI, true
	}
	
	// Priorité 2: HTTPS localhost (si certificat disponible)
	if isHTTPSLocalhostAvailable() {
		fmt.Println("🔒 Utilisation de HTTPS localhost: https://localhost:8080/callback")
		debugLog("HTTPS localhost disponible, utilisation de %s", HTTPSRedirectURI)
		return HTTPSRedirectURI, false
	}
	
	// Fallback: HTTP localhost (pour anciennes apps uniquement)
	fmt.Println("⚠️  Utilisation de HTTP localhost (non conforme aux nouvelles exigences Spotify)")
	fmt.Println("   Considérez migrer vers songbattle://callback pour la conformité 2025")
	debugLog("Fallback vers HTTP localhost: %s", RedirectURI)
	return RedirectURI, false
}

// isCustomSchemeSupported vérifie si le custom scheme peut être utilisé
func isCustomSchemeSupported() bool {
	// Le custom scheme nécessite une configuration OS spécifique
	// Pour une meilleure UX, on utilise HTTP 127.0.0.1 par défaut (conforme Spotify 2025)
	// Les utilisateurs avancés peuvent forcer le custom scheme avec -use-custom-scheme
	debugLog("Vérification du support custom scheme: désactivé par défaut (false)")
	return false
}

// isHTTPSLocalhostAvailable vérifie si HTTPS localhost est disponible
func isHTTPSLocalhostAvailable() bool {
	// Vérifier si des certificats sont disponibles
	// Pour l'instant, on retourne false pour garder la simplicité
	debugLog("Vérification HTTPS localhost: non disponible (false)")
	return false
}

// generateCodeVerifier génère un code verifier pour PKCE
func generateCodeVerifier() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(bytes), nil
}

// generateCodeChallenge génère un code challenge à partir du verifier
func generateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(hash[:])
}

// Authenticate lance le processus d'authentification OAuth2 avec PKCE
func (sa *SpotifyAuth) Authenticate(ctx context.Context) (*oauth2.Token, error) {
	// Générer PKCE codes
	codeVerifier, err := generateCodeVerifier()
	if err != nil {
		return nil, fmt.Errorf("erreur génération code verifier: %w", err)
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
		// Handler pour custom scheme - écoute sur tous les paths
		http.HandleFunc("/", sa.handleCustomSchemeCallback(codeChan, errChan))
	} else {
		// Handler classique pour HTTP(S)
		http.HandleFunc("/callback", sa.handleHTTPCallback(codeChan, errChan))
	}

	// Lancer le serveur en arrière-plan
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("erreur serveur callback: %w", err)
		}
	}()

	// Construire l'URL d'autorisation avec PKCE
	authURL := sa.config.AuthCodeURL("state",
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"))

	fmt.Println("🎵 Authentification Spotify requise")
	if sa.useCustomScheme {
		fmt.Println("🔒 Utilisation du mode sécurisé (Custom Scheme)")
		fmt.Printf("🌐 Port d'écoute: localhost%s\n", port)
	} else {
		fmt.Printf("🌐 Port d'écoute: localhost%s\n", port)
	}
	fmt.Println("Ouverture de votre navigateur...")
	fmt.Printf("Si ça ne marche pas, copiez cette URL: %s\n", authURL)

	// Ouvrir le navigateur
	if err := browser.OpenURL(authURL); err != nil {
		fmt.Printf("Erreur ouverture navigateur: %v\n", err)
		fmt.Printf("Veuillez ouvrir manuellement: %s\n", authURL)
	}

	// Attendre le code ou une erreur
	var code string
	select {
	case code = <-codeChan:
		// Code reçu
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("timeout authentification")
	}

	// Fermer le serveur
	server.Shutdown(context.Background())

	// Échanger le code contre un token avec PKCE
	token, err := sa.exchangeCodeForToken(code, codeVerifier)
	if err != nil {
		return nil, fmt.Errorf("erreur échange code/token: %w", err)
	}

	// Sauvegarder le token
	if err := sa.SaveToken(token); err != nil {
		return nil, fmt.Errorf("erreur sauvegarde token: %w", err)
	}

	return token, nil
}

// exchangeCodeForToken échange le code d'autorisation contre un token d'accès
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

// SaveToken sauvegarde le token en base de données
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

// LoadToken charge le token depuis la base de données
func (sa *SpotifyAuth) LoadToken() (*oauth2.Token, error) {
	accessToken, err := sa.db.GetMeta(models.MetaKeyAccessToken)
	if err != nil {
		return nil, fmt.Errorf("aucun token d'accès trouvé: %w", err)
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

// RefreshToken renouvelle le token d'accès
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

// IsTokenValid vérifie si le token est encore valide
func (sa *SpotifyAuth) IsTokenValid(token *oauth2.Token) bool {
	if token == nil || token.AccessToken == "" {
		return false
	}

	// Si pas d'expiry, considérer comme valide
	if token.Expiry.IsZero() {
		return true
	}

	// Vérifier si le token expire dans les 5 prochaines minutes
	return time.Now().Add(5 * time.Minute).Before(token.Expiry)
}

// GetValidToken récupère un token valide (charge ou renouvelle si nécessaire)
func (sa *SpotifyAuth) GetValidToken(ctx context.Context) (*oauth2.Token, error) {
	debugLog("Début GetValidToken - URI configuré: %s", sa.redirectURI)
	
	// Tenter de charger le token existant
	token, err := sa.LoadToken()
	if err != nil {
		debugLog("Aucun token existant trouvé, nouvelle authentification requise: %v", err)
		// Aucun token, authentification requise
		return sa.Authenticate(ctx)
	}

	// Vérifier si le token est valide
	if sa.IsTokenValid(token) {
		debugLog("Token existant valide, réutilisation")
		return token, nil
	}

	debugLog("Token expiré, tentative de refresh")
	// Token expiré, tenter de le renouveler
	if token.RefreshToken != "" {
		newToken, err := sa.RefreshToken(ctx, token)
		if err == nil {
			debugLog("Token refreshé avec succès")
			return newToken, nil
		}
		debugLog("Échec du refresh token: %v", err)
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
			errChan <- fmt.Errorf("aucun code d'autorisation reçu")
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
			errChan <- fmt.Errorf("aucun code d'autorisation reçu via custom scheme")
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

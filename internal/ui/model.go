package ui

import (
	"context"
	"fmt"
	"songbattle/internal/auth"
	"songbattle/internal/elo"
	"songbattle/internal/matchmaker"
	"songbattle/internal/models"
	"songbattle/internal/spotify"
	"songbattle/internal/store"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pkg/browser"
)

// ViewState représente l'état actuel de la vue
type ViewState int

const (
	ViewDuel ViewState = iota
	ViewAudioFeatures
	ViewLoading
	ViewError
	ViewLeaderboard
)

// FocusPosition représente quel élément a le focus
type FocusPosition int

const (
	FocusLeft FocusPosition = iota
	FocusRight
)

// Model représente le modèle principal de l'application Bubble Tea
type Model struct {
	// État de la vue
	currentView ViewState
	focus       FocusPosition

	// Composants du système
	db            *store.DB
	eloSystem     *elo.EloSystem
	matchmaker    *matchmaker.Matchmaker
	auth          *auth.SpotifyAuth
	spotifyClient *spotify.Client

	// Configuration
	clientID string
	ctx      context.Context

	// État du duel actuel
	leftTrack  *models.TrackWithRating
	rightTrack *models.TrackWithRating

	// Messages et état
	statusMessage string
	errorMessage  string
	isLoading     bool

	// Dimensions de l'écran
	width  int
	height int

	// Audio features pour l'affichage détaillé
	currentAudioFeatures map[string]float64

	// Leaderboard
	leaderboard       []models.TrackWithRating
	leaderboardCursor int
}

// NewModel crée une nouvelle instance du modèle
func NewModel(db *store.DB, clientID string) *Model {
	return NewModelWithOptions(db, clientID, "", false, false)
}

// NewModelWithOptions crée une nouvelle instance du modèle avec des options d'URI
func NewModelWithOptions(db *store.DB, clientID, redirectURI string, useCustom, useHTTPS bool) *Model {
	ctx := context.Background()

	return &Model{
		currentView:   ViewLoading,
		focus:         FocusLeft,
		db:            db,
		eloSystem:     elo.NewEloSystem(db),
		matchmaker:    matchmaker.NewMatchmaker(db),
		auth:          auth.NewSpotifyAuthWithOptions(clientID, db, redirectURI, useCustom, useHTTPS),
		clientID:      clientID,
		ctx:           ctx,
		statusMessage: "Initialisation...",
		width:         100,
		height:        30,
	}
}

// Messages personnalisés pour Bubble Tea
type InitCompleteMsg struct {
	SpotifyClient *spotify.Client
}
type DuelSetupCompleteMsg struct {
	Left  *models.TrackWithRating
	Right *models.TrackWithRating
}
type ErrorMsg struct{ Err error }
type PlayTrackMsg struct{ TrackURI string }
type AudioFeaturesMsg struct{ Features map[string]float64 }

// Init initialise le modèle
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.initializeApp,
		tea.EnterAltScreen,
	)
}

// Update gère les événements et met à jour le modèle
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case InitCompleteMsg:
		m.spotifyClient = msg.SpotifyClient
		m.currentView = ViewDuel
		m.isLoading = false
		return m, m.setupNextDuel

	case DuelSetupCompleteMsg:
		m.leftTrack = msg.Left
		m.rightTrack = msg.Right
		m.statusMessage = "Prêt pour le duel !"
		return m, nil

	case ErrorMsg:
		m.currentView = ViewError
		m.errorMessage = msg.Err.Error()
		m.isLoading = false
		return m, nil

	case AudioFeaturesMsg:
		m.currentView = ViewAudioFeatures
		m.currentAudioFeatures = msg.Features
		return m, nil

	default:
		return m, nil
	}
}

// View génère la vue à afficher
func (m Model) View() string {
	switch m.currentView {
	case ViewLoading:
		return m.renderLoading()
	case ViewError:
		return m.renderError()
	case ViewAudioFeatures:
		return m.renderAudioFeatures()
	case ViewLeaderboard:
		return m.renderLeaderboard()
	case ViewDuel:
		return m.renderDuel()
	default:
		return m.renderDuel()
	}
}

// handleKeyPress gère les événements clavier
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		// Si dans le leaderboard, 'q' retourne au duel (pas de quit)
		if m.currentView == ViewLeaderboard {
			m.currentView = ViewDuel
			m.statusMessage = ""
			return m, nil
		}
		return m, tea.Quit

	case "left", "h":
		m.focus = FocusLeft
		return m, nil

	case "right", "l":
		m.focus = FocusRight
		return m, nil

	case "enter":
		if m.currentView == ViewLeaderboard {
			return m.handleLeaderboardSelect()
		}
		return m.handleVote()

	case " ":
		// Dans le leaderboard, jouer le track sélectionné
		if m.currentView == ViewLeaderboard {
			return m.handlePlayLeaderboardTrack()
		}
		// Dans le duel, jouer le track avec le focus
		return m.handlePlayTrack()

	case "s":
		return m.handleSkip()

	case "t":
		// Audio features désactivé temporairement (API 403)
		m.statusMessage = "⚠️  Audio features indisponible (permissions Spotify limitées)"
		return m, nil
		// return m.handleShowAudioFeatures()

	case "g":
		return m.handleOpenSpotify()

	case "p":
		return m.handleExportPlaylist()

	case "c":
		return m.handleShowLeaderboard()

	case "up", "k":
		if m.currentView == ViewLeaderboard && m.leaderboardCursor > 0 {
			m.leaderboardCursor--
		}
		return m, nil

	case "down", "j":
		if m.currentView == ViewLeaderboard && m.leaderboardCursor < len(m.leaderboard)-1 {
			m.leaderboardCursor++
		}
		return m, nil

	case "escape":
		// Return to duel from audio features, error or leaderboard
		if m.currentView == ViewLeaderboard {
			m.currentView = ViewDuel
			m.statusMessage = "Back to battles"
			return m, nil
		}
		if m.currentView == ViewAudioFeatures || m.currentView == ViewError {
			m.currentView = ViewDuel
			m.errorMessage = ""
			return m, nil
		}
		return m, nil

	case "r":
		// Réessayer (depuis erreur) ou retour
		if m.currentView == ViewError {
			m.currentView = ViewDuel
			m.errorMessage = ""
		}
		return m, nil

	default:
		return m, nil
	}

	return m, nil
}

// handleVote traite un vote pour le track avec le focus
func (m Model) handleVote() (tea.Model, tea.Cmd) {
	if m.leftTrack == nil || m.rightTrack == nil {
		return m, nil
	}

	var winner string
	var winnerName string

	if m.focus == FocusLeft {
		winner = models.WinnerLeft
		winnerName = m.leftTrack.Track.Name
	} else {
		winner = models.WinnerRight
		winnerName = m.rightTrack.Track.Name
	}

	// Traiter le duel
	if err := m.eloSystem.ProcessDuel(m.leftTrack.Track.ID, m.rightTrack.Track.ID, winner); err != nil {
		return m, m.sendError(fmt.Errorf("erreur traitement duel: %w", err))
	}

	m.statusMessage = "🏆 " + winnerName + " remporte le duel !"

	// Préparer le prochain duel après un court délai
	return m, tea.Sequence(
		tea.Tick(time.Second*2, func(time.Time) tea.Msg {
			return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("next")}
		}),
		m.setupNextDuel,
	)
}

// handleSkip handles a duel skip
func (m Model) handleSkip() (tea.Model, tea.Cmd) {
	if m.leftTrack == nil || m.rightTrack == nil {
		return m, nil
	}

	// Process skip
	if err := m.eloSystem.ProcessDuel(m.leftTrack.Track.ID, m.rightTrack.Track.ID, models.WinnerSkip); err != nil {
		return m, m.sendError(fmt.Errorf("failed to skip duel: %w", err))
	}

	m.statusMessage = "⏭️ Battle skipped!"
	return m, m.setupNextDuel
}

// handlePlayTrack traite la lecture d'un track
func (m Model) handlePlayTrack() (tea.Model, tea.Cmd) {
	var track *models.Track
	var side string
	if m.focus == FocusLeft && m.leftTrack != nil {
		track = &m.leftTrack.Track
		side = "gauche"
	} else if m.focus == FocusRight && m.rightTrack != nil {
		track = &m.rightTrack.Track
		side = "droite"
	}

	if track == nil {
		m.statusMessage = "⚠️ Aucun track sélectionné"
		return m, nil
	}

	m.statusMessage = fmt.Sprintf("🎵 Lecture : %s (%s)", track.Name, side)
	return m, m.playTrack(track.SpotifyURI)
}

// handleShowAudioFeatures affiche les caractéristiques audio
func (m Model) handleShowAudioFeatures() (tea.Model, tea.Cmd) {
	var track *models.Track
	if m.focus == FocusLeft && m.leftTrack != nil {
		track = &m.leftTrack.Track
	} else if m.focus == FocusRight && m.rightTrack != nil {
		track = &m.rightTrack.Track
	}

	if track == nil {
		return m, nil
	}

	return m, m.getAudioFeatures(track.SpotifyID)
}

// handleOpenSpotify ouvre Spotify dans le navigateur
func (m Model) handleOpenSpotify() (tea.Model, tea.Cmd) {
	var track *models.Track
	if m.focus == FocusLeft && m.leftTrack != nil {
		track = &m.leftTrack.Track
	} else if m.focus == FocusRight && m.rightTrack != nil {
		track = &m.rightTrack.Track
	}

	if track == nil {
		return m, nil
	}

	url := "https://open.spotify.com/track/" + track.SpotifyID
	go browser.OpenURL(url)

	m.statusMessage = "🌐 Ouverture de Spotify dans le navigateur..."
	return m, nil
}

// handleExportPlaylist exporte le top des tracks en playlist
func (m Model) handleExportPlaylist() (tea.Model, tea.Cmd) {
	m.statusMessage = "📝 Export de playlist en cours..."
	return m, m.exportPlaylist()
}

// handleShowLeaderboard shows the leaderboard
func (m Model) handleShowLeaderboard() (tea.Model, tea.Cmd) {
	// Get all tracks sorted by Elo
	tracks, err := m.db.GetAllTracksWithRatings()
	if err != nil {
		m.statusMessage = "⚠️  Failed to load leaderboard"
		return m, nil
	}

	m.leaderboard = tracks
	m.leaderboardCursor = 0
	m.currentView = ViewLeaderboard
	return m, nil
}

// handlePlayLeaderboardTrack joue le track sélectionné dans le leaderboard
func (m Model) handlePlayLeaderboardTrack() (tea.Model, tea.Cmd) {
	if len(m.leaderboard) == 0 || m.leaderboardCursor >= len(m.leaderboard) {
		m.statusMessage = "⚠️  Aucun track sélectionné"
		return m, nil
	}

	selectedTrack := &m.leaderboard[m.leaderboardCursor]
	m.statusMessage = fmt.Sprintf("🎵 Lecture : %s - %s", selectedTrack.Track.Name, selectedTrack.Track.Artist)

	return m, m.playTrack(selectedTrack.Track.SpotifyURI)
}

// handleLeaderboardSelect sélectionne un track du leaderboard pour un duel
func (m Model) handleLeaderboardSelect() (tea.Model, tea.Cmd) {
	if len(m.leaderboard) == 0 || m.leaderboardCursor >= len(m.leaderboard) {
		return m, nil
	}

	// Utiliser le track sélectionné comme adversaire pour le prochain duel
	selectedTrack := &m.leaderboard[m.leaderboardCursor]

	// Trouver un autre track aléatoire pour faire un duel
	var opponent *models.TrackWithRating
	for i := range m.leaderboard {
		if m.leaderboard[i].Track.ID != selectedTrack.Track.ID {
			opponent = &m.leaderboard[i]
			break
		}
	}

	if opponent == nil {
		m.statusMessage = "⚠️  Pas assez de tracks pour un duel"
		return m, nil
	}

	// Configurer le duel
	m.leftTrack = selectedTrack
	m.rightTrack = opponent
	m.focus = FocusLeft
	m.currentView = ViewDuel
	m.statusMessage = "Battle from leaderboard!"

	return m, nil
}

// Commandes Bubble Tea

// initializeApp initialise l'authentification et l'application
func (m Model) initializeApp() tea.Msg {
	// Vérifier l'authentification
	token, err := m.auth.GetValidToken(m.ctx)
	if err != nil {
		return ErrorMsg{Err: fmt.Errorf("erreur authentification: %w", err)}
	}

	// Créer le client Spotify
	spotifyClient := spotify.NewClient(m.ctx, token, m.clientID)

	return InitCompleteMsg{SpotifyClient: spotifyClient}
}

// setupNextDuel configure le prochain duel
func (m Model) setupNextDuel() tea.Msg {
	left, right, err := m.matchmaker.GetNextMatch()
	if err != nil {
		return ErrorMsg{Err: fmt.Errorf("erreur matchmaking: %w", err)}
	}

	return DuelSetupCompleteMsg{Left: left, Right: right}
}

// playTrack joue un track sur Spotify
func (m Model) playTrack(trackURI string) tea.Cmd {
	return func() tea.Msg {
		if m.spotifyClient == nil {
			return ErrorMsg{Err: fmt.Errorf("client Spotify non initialisé")}
		}

		err := m.spotifyClient.PlayTrack(trackURI)
		if err != nil {
			// Fallback: ouvrir dans le navigateur
			url := "https://open.spotify.com/track/" + trackURI[14:] // Enlever "spotify:track:"
			browser.OpenURL(url)
			return ErrorMsg{Err: fmt.Errorf("lecture Spotify échouée, ouverture navigateur: %w", err)}
		}

		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("played")}
	}
}

// getAudioFeatures récupère les caractéristiques audio
func (m Model) getAudioFeatures(trackID string) tea.Cmd {
	return func() tea.Msg {
		if m.spotifyClient == nil {
			return ErrorMsg{Err: fmt.Errorf("client Spotify non initialisé")}
		}

		features, err := m.spotifyClient.GetAudioFeatures(trackID)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("erreur récupération audio features: %w", err)}
		}

		// Convertir en map pour l'affichage
		featuresMap := map[string]float64{
			"danceability": features.Danceability,
			"energy":       features.Energy,
			"valence":      features.Valence,
			"acousticness": features.Acousticness,
			"tempo":        features.Tempo,
		}

		return AudioFeaturesMsg{Features: featuresMap}
	}
}

// exportPlaylist exporte une playlist des meilleurs tracks
func (m Model) exportPlaylist() tea.Cmd {
	return func() tea.Msg {
		// Récupérer les top tracks
		topTracks, err := m.eloSystem.GetEloRanking(50)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("erreur récupération top tracks: %w", err)}
		}

		if len(topTracks) == 0 {
			return ErrorMsg{Err: fmt.Errorf("aucun track trouvé pour l'export")}
		}

		// Créer la playlist (simulation, nécessite l'utilisateur Spotify)
		// TODO: Implémenter l'export réel avec l'API Spotify

		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("exported")}
	}
}

// sendError envoie un message d'erreur
func (m Model) sendError(err error) tea.Cmd {
	return func() tea.Msg {
		return ErrorMsg{Err: err}
	}
}

// Fonctions de rendu

// renderLoading affiche l'écran de chargement
func (m Model) renderLoading() string {
	content := fmt.Sprintf(`
%s

🔄 %s

Veuillez patienter...
`, RenderHeader(), m.statusMessage)

	return ContainerStyle.Width(m.width - 4).Height(m.height - 4).Render(content)
}

// renderError affiche l'écran d'erreur
func (m Model) renderError() string {
	errorStyle := lipgloss.NewStyle().
		Foreground(ColorError).
		Bold(true).
		Padding(1, 2)

	helpStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Padding(1, 0)

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		RenderHeader(),
		"",
		errorStyle.Render("❌ "+m.errorMessage),
		"",
		helpStyle.Render("Press 'r' or Escape to return  •  'q' to quit"),
	)

	return content
}

// renderDuel affiche l'écran principal de duel
func (m Model) renderDuel() string {
	if m.leftTrack == nil || m.rightTrack == nil {
		return m.renderLoading()
	}

	// Cards des tracks
	leftCard := RenderTrackCard(
		m.leftTrack.Track.Name,
		m.leftTrack.Track.Artist,
		m.leftTrack.Track.Album,
		m.leftTrack.Track.Year,
		m.leftTrack.Rating.Elo,
		m.leftTrack.Rating.Wins,
		m.leftTrack.Rating.Losses,
		m.focus == FocusLeft,
	)

	rightCard := RenderTrackCard(
		m.rightTrack.Track.Name,
		m.rightTrack.Track.Artist,
		m.rightTrack.Track.Album,
		m.rightTrack.Track.Year,
		m.rightTrack.Rating.Elo,
		m.rightTrack.Rating.Wins,
		m.rightTrack.Rating.Losses,
		m.focus == FocusRight,
	)

	// Assemblage de la vue - placer les cartes côte à côte avec VS au milieu
	duelArea := lipgloss.JoinHorizontal(
		lipgloss.Center,
		leftCard,
		RenderVersus(),
		rightCard,
	)

	// Calculer la largeur totale de la zone de duel
	// 40 (carte gauche) + 6 (VS) + 40 (carte droite) = 86
	totalWidth := 86

	// Centrer le header et les contrôles sur la même largeur
	centeredHeader := lipgloss.NewStyle().Width(totalWidth).Align(lipgloss.Center).Render(RenderHeader())
	centeredControls := lipgloss.NewStyle().Width(totalWidth).Align(lipgloss.Center).Render(RenderControls())
	centeredFooter := lipgloss.NewStyle().Width(totalWidth).Align(lipgloss.Center).Render(RenderFooter(m.statusMessage))

	// Assembler le contenu verticalement de manière compacte
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		centeredHeader,
		"",
		duelArea,
		"",
		centeredControls,
		centeredFooter,
	)

	return content
}

// renderAudioFeatures affiche les caractéristiques audio
func (m Model) renderAudioFeatures() string {
	content := fmt.Sprintf(`
%s

%s

%s

Press 'Escape' to return to battle.
`,
		RenderHeader(),
		RenderAudioFeatures(m.currentAudioFeatures),
		RenderFooter("Audio features details"),
	)

	return ContainerStyle.Width(m.width - 4).Height(m.height - 4).Render(content)
}

// renderLeaderboard affiche le classement des tracks
func (m Model) renderLeaderboard() string {
	if len(m.leaderboard) == 0 {
		return lipgloss.JoinVertical(
			lipgloss.Center,
			RenderHeader(),
			"",
			"No tracks in leaderboard",
			"",
			"Press Escape to return",
		)
	}

	// Styles
	rankStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Width(4).
		Align(lipgloss.Right)

	nameStyle := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Width(40)

	artistStyle := lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Width(30)

	eloStyle := lipgloss.NewStyle().
		Foreground(ColorSuccess).
		Bold(true).
		Width(10).
		Align(lipgloss.Right)

	statsStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Width(15).
		Align(lipgloss.Right)

	selectedStyle := lipgloss.NewStyle().
		Background(ColorPrimary).
		Foreground(lipgloss.Color("#000000")).
		Bold(true)

	// Header du tableau
	header := lipgloss.JoinHorizontal(
		lipgloss.Top,
		rankStyle.Render("#"),
		nameStyle.Bold(true).Render("Titre"),
		artistStyle.Bold(true).Render("Artiste"),
		eloStyle.Render("Elo"),
		statsStyle.Render("W/L"),
	)

	// Lignes du classement (afficher 15 max)
	var lines []string
	lines = append(lines, header)
	lines = append(lines, lipgloss.NewStyle().Foreground(ColorBorder).Render("─────────────────────────────────────────────────────────────────────────────────────────────"))

	start := 0
	end := len(m.leaderboard)
	if end > 15 {
		// Centrer sur le curseur
		start = m.leaderboardCursor - 7
		if start < 0 {
			start = 0
		}
		end = start + 15
		if end > len(m.leaderboard) {
			end = len(m.leaderboard)
			start = end - 15
			if start < 0 {
				start = 0
			}
		}
	}

	for i := start; i < end; i++ {
		track := m.leaderboard[i]

		rankStr := rankStyle.Render(fmt.Sprintf("%d", i+1))
		nameStr := nameStyle.Render(truncate(track.Track.Name, 38))
		artistStr := artistStyle.Render(truncate(track.Track.Artist, 28))
		eloStr := eloStyle.Render(fmt.Sprintf("%d", track.Rating.Elo))
		statsStr := statsStyle.Render(fmt.Sprintf("%d/%d", track.Rating.Wins, track.Rating.Losses))

		line := lipgloss.JoinHorizontal(
			lipgloss.Top,
			rankStr,
			nameStr,
			artistStr,
			eloStr,
			statsStr,
		)

		if i == m.leaderboardCursor {
			line = selectedStyle.Render(line)
		}

		lines = append(lines, line)
	}

	// Contrôles
	controls := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Padding(1, 0).
		Render("↑↓ navigate  ␣ play  ↵ battle  q back")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		RenderHeader(),
		"",
		lipgloss.JoinVertical(lipgloss.Left, lines...),
		"",
		controls,
		RenderFooter(fmt.Sprintf("Leaderboard - %d tracks", len(m.leaderboard))),
	)

	return content
}

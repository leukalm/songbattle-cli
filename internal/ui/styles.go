package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Couleurs du thème
var (
	ColorPrimary   = lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A855F7"}
	ColorSecondary = lipgloss.AdaptiveColor{Light: "#0891B2", Dark: "#06B6D4"}
	ColorSuccess   = lipgloss.AdaptiveColor{Light: "#059669", Dark: "#10B981"}
	ColorWarning   = lipgloss.AdaptiveColor{Light: "#D97706", Dark: "#F59E0B"}
	ColorError     = lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#EF4444"}
	ColorMuted     = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
	ColorBorder    = lipgloss.AdaptiveColor{Light: "#E5E7EB", Dark: "#374151"}
)

// Styles principaux
var (
	// Titre de l'application
	TitleStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			Align(lipgloss.Center).
			MarginBottom(1)

	// Conteneur principal
	ContainerStyle = lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder)

	// Card pour les tracks
	TrackCardStyle = lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Width(40).
			Height(8)

	// Card active (focus)
	TrackCardActiveStyle = TrackCardStyle.Copy().
				BorderForeground(ColorPrimary).
				Bold(true)

	// Nom de la track
	TrackNameStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			Width(36).
			Align(lipgloss.Center)

	// Artiste
	ArtistStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Italic(true).
			Width(36).
			Align(lipgloss.Center)

	// Album et année
	AlbumStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Width(36).
			Align(lipgloss.Center)

	// Elo score
	EloStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true).
			Width(36).
			Align(lipgloss.Center)

	// Statistiques (wins/losses)
	StatsStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Width(36).
			Align(lipgloss.Center)

	// Instructions/controls
	ControlsStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			MarginTop(1).
			Align(lipgloss.Center)

	// Messages d'état
	StatusStyle = lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true).
			Align(lipgloss.Center).
			MarginTop(1)

	// Messages d'erreur
	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true).
			Align(lipgloss.Center).
			MarginTop(1)

	// Messages de succès
	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true).
			Align(lipgloss.Center).
			MarginTop(1)

	// Séparateur
	SeparatorStyle = lipgloss.NewStyle().
			Foreground(ColorBorder).
			Align(lipgloss.Center).
			MarginTop(1).
			MarginBottom(1)

	// Indicators
	IndicatorActiveStyle = lipgloss.NewStyle().
				Foreground(ColorPrimary).
				Bold(true)

	IndicatorInactiveStyle = lipgloss.NewStyle().
				Foreground(ColorMuted)

	// Boutons/actions
	ButtonStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#000000"}).
			Background(ColorPrimary).
			Padding(0, 2).
			Bold(true)

	ButtonActiveStyle = ButtonStyle.Copy().
				Background(ColorSecondary)

	// Header avec logo
	HeaderStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			Align(lipgloss.Center).
			MarginBottom(2)

	// Footer
	FooterStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Align(lipgloss.Center).
			MarginTop(2)
)

// Fonctions utilitaires pour les styles

// RenderTrackCard génère le rendu d'une card de track
func RenderTrackCard(name, artist, album string, year, elo, wins, losses int, active bool) string {
	style := TrackCardStyle
	if active {
		style = TrackCardActiveStyle
	}

	yearStr := ""
	if year > 0 {
		yearStr = fmt.Sprintf(" (%d)", year)
	}

	content := lipgloss.JoinVertical(lipgloss.Center,
		TrackNameStyle.Render(truncate(name, 34)),
		ArtistStyle.Render(truncate(artist, 34)),
		AlbumStyle.Render(truncate(album, 30)+yearStr),
		"",
		EloStyle.Render(fmt.Sprintf("Elo: %d", elo)),
		StatsStyle.Render(fmt.Sprintf("%d W • %d L", wins, losses)),
	)

	return style.Render(content)
}

// RenderVersus génère l'affichage "VS" avec hauteur fixe alignée
func RenderVersus() string {
	// Même hauteur que les cartes (8) pour un alignement parfait
	vs := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true).
		AlignVertical(lipgloss.Center).
		AlignHorizontal(lipgloss.Center).
		Width(6).
		Height(8).
		Render("VS")

	return vs
}

// RenderControls génère l'affichage des contrôles
func RenderControls() string {
	// Style pour les raccourcis
	keyStyle := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)

	labelStyle := lipgloss.NewStyle().
		Foreground(ColorMuted)

	// Contrôles principaux
	mainControls := fmt.Sprintf("%s %s  %s %s  %s %s",
		keyStyle.Render("←→"),
		labelStyle.Render("naviguer"),
		keyStyle.Render("␣"),
		labelStyle.Render("écouter"),
		keyStyle.Render("↵"),
		labelStyle.Render("voter"),
	)

	// Contrôles secondaires
	secondaryControls := fmt.Sprintf("%s %s  %s %s  %s %s  %s %s",
		keyStyle.Render("s"),
		labelStyle.Render("skip"),
		keyStyle.Render("c"),
		labelStyle.Render("classement"),
		keyStyle.Render("g"),
		labelStyle.Render("spotify"),
		keyStyle.Render("q"),
		labelStyle.Render("quitter"),
	)

	return lipgloss.JoinVertical(
		lipgloss.Center,
		mainControls,
		secondaryControls,
	)
}

// RenderHeader génère l'en-tête de l'application
func RenderHeader() string {
	title := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true).
		Render("🎵 Song Battle 🎵")

	separator := lipgloss.NewStyle().
		Foreground(ColorBorder).
		Render("────────────────────────────────────────────────────────────────")

	return lipgloss.JoinVertical(lipgloss.Center, title, separator)
}

// RenderFooter génère le pied de page
func RenderFooter(message string) string {
	if message == "" {
		message = "Prêt pour le duel !"
	}
	return lipgloss.NewStyle().
		Foreground(ColorSuccess).
		Padding(1, 0, 0, 0).
		Render(message)
}

// RenderStatus génère un message de statut
func RenderStatus(message string, statusType string) string {
	switch statusType {
	case "error":
		return ErrorStyle.Render("❌ " + message)
	case "success":
		return SuccessStyle.Render("✅ " + message)
	case "warning":
		return StatusStyle.Render("⚠️ " + message)
	default:
		return StatusStyle.Render(message)
	}
}

// RenderSeparator génère un séparateur
func RenderSeparator() string {
	return SeparatorStyle.Render("──────────────────────────────")
}

// Fonctions utilitaires

// truncate tronque une chaîne si elle est trop longue
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

// RenderAudioFeatures génère l'affichage des caractéristiques audio
func RenderAudioFeatures(af map[string]float64) string {
	if len(af) == 0 {
		return ErrorStyle.Render("Aucune caractéristique audio disponible")
	}

	features := []string{
		lipgloss.NewStyle().Bold(true).Render("🎵 Caractéristiques Audio 🎵"),
		"",
	}

	if val, ok := af["danceability"]; ok {
		features = append(features, renderFeature("💃 Danceability", val))
	}
	if val, ok := af["energy"]; ok {
		features = append(features, renderFeature("⚡ Energy", val))
	}
	if val, ok := af["valence"]; ok {
		features = append(features, renderFeature("😊 Valence", val))
	}
	if val, ok := af["acousticness"]; ok {
		features = append(features, renderFeature("🎸 Acousticness", val))
	}
	if val, ok := af["tempo"]; ok {
		features = append(features, renderTempoFeature("🥁 Tempo", val))
	}

	return ContainerStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left, features...),
	)
}

// renderFeature génère l'affichage d'une caractéristique (0-1)
func renderFeature(name string, value float64) string {
	percentage := int(value * 100)
	bar := renderProgressBar(value, 20)
	return fmt.Sprintf("%s: %s %d%%", name, bar, percentage)
}

// renderTempoFeature génère l'affichage du tempo
func renderTempoFeature(name string, value float64) string {
	return fmt.Sprintf("%s: %.0f BPM", name, value)
}

// renderProgressBar génère une barre de progression
func renderProgressBar(value float64, width int) string {
	filled := int(value * float64(width))
	bar := ""

	for i := 0; i < width; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}

	return bar
}

// GetScreenDimensions retourne les dimensions recommandées
func GetScreenDimensions() (int, int) {
	return 100, 30 // Largeur, Hauteur recommandées
}

package export

import (
	"context"
	"fmt"
	"songbattle/internal/models"
	"songbattle/internal/spotify"
	"songbattle/internal/store"
	"time"
)

type PlaylistExporter struct {
	db            *store.DB
	spotifyClient *spotify.Client
	ctx           context.Context
}

// NewPlaylistExporter crée une nouvelle instance d'exporteur de playlist
func NewPlaylistExporter(db *store.DB, spotifyClient *spotify.Client, ctx context.Context) *PlaylistExporter {
	return &PlaylistExporter{
		db:            db,
		spotifyClient: spotifyClient,
		ctx:           ctx,
	}
}

// ExportTopTracks exporte les N meilleurs tracks vers une playlist Spotify
func (pe *PlaylistExporter) ExportTopTracks(limit int) (*PlaylistInfo, error) {
	// Récupérer les top tracks
	topTracks, err := pe.db.GetTopTracks(limit)
	if err != nil {
		return nil, fmt.Errorf("erreur récupération top tracks: %w", err)
	}

	if len(topTracks) == 0 {
		return nil, fmt.Errorf("aucun track trouvé")
	}

	// Récupérer l'utilisateur actuel
	user, err := pe.spotifyClient.GetCurrentUser()
	if err != nil {
		return nil, fmt.Errorf("erreur récupération utilisateur: %w", err)
	}

	// Créer la playlist
	playlistName := fmt.Sprintf("Song Battle Top %d", len(topTracks))
	playlistDescription := fmt.Sprintf("Top %d des meilleures chansons selon Song Battle - Créée le %s",
		len(topTracks), time.Now().Format("02/01/2006"))

	playlist, err := pe.spotifyClient.CreatePlaylist(
		string(user.ID),
		playlistName,
		playlistDescription,
	)
	if err != nil {
		return nil, fmt.Errorf("erreur création playlist: %w", err)
	}

	// Préparer les URIs des tracks
	trackURIs := make([]string, 0, len(topTracks))
	for _, track := range topTracks {
		trackURIs = append(trackURIs, track.Track.SpotifyURI)
	}

	// Ajouter les tracks à la playlist (par batches de 100)
	batchSize := 100
	for i := 0; i < len(trackURIs); i += batchSize {
		end := i + batchSize
		if end > len(trackURIs) {
			end = len(trackURIs)
		}

		batch := trackURIs[i:end]
		if err := pe.spotifyClient.AddTracksToPlaylist(string(playlist.ID), batch); err != nil {
			return nil, fmt.Errorf("erreur ajout tracks playlist: %w", err)
		}
	}

	// Retourner les informations de la playlist créée
	return &PlaylistInfo{
		ID:          string(playlist.ID),
		Name:        playlist.Name,
		Description: playlist.Description,
		URL:         playlist.ExternalURLs["spotify"],
		TrackCount:  len(topTracks),
		CreatedAt:   time.Now(),
		Tracks:      topTracks,
	}, nil
}

// ExportCustomPlaylist exporte une sélection personnalisée de tracks
func (pe *PlaylistExporter) ExportCustomPlaylist(trackIDs []int64, name, description string) (*PlaylistInfo, error) {
	if len(trackIDs) == 0 {
		return nil, fmt.Errorf("aucun track spécifié")
	}

	// Récupérer les tracks avec leurs ratings
	tracks := make([]models.TrackWithRating, 0, len(trackIDs))
	for _, trackID := range trackIDs {
		track, err := pe.db.GetTrackWithRating(trackID)
		if err != nil {
			continue // Ignorer les tracks introuvables
		}
		tracks = append(tracks, *track)
	}

	if len(tracks) == 0 {
		return nil, fmt.Errorf("aucun track valide trouvé")
	}

	// Récupérer l'utilisateur actuel
	user, err := pe.spotifyClient.GetCurrentUser()
	if err != nil {
		return nil, fmt.Errorf("erreur récupération utilisateur: %w", err)
	}

	// Créer la playlist
	if name == "" {
		name = "Song Battle Custom Playlist"
	}
	if description == "" {
		description = fmt.Sprintf("Playlist personnalisée Song Battle - %d chansons - Créée le %s",
			len(tracks), time.Now().Format("02/01/2006"))
	}

	playlist, err := pe.spotifyClient.CreatePlaylist(
		string(user.ID),
		name,
		description,
	)
	if err != nil {
		return nil, fmt.Errorf("erreur création playlist: %w", err)
	}

	// Préparer les URIs des tracks
	trackURIs := make([]string, 0, len(tracks))
	for _, track := range tracks {
		trackURIs = append(trackURIs, track.Track.SpotifyURI)
	}

	// Ajouter les tracks à la playlist
	if err := pe.spotifyClient.AddTracksToPlaylist(string(playlist.ID), trackURIs); err != nil {
		return nil, fmt.Errorf("erreur ajout tracks playlist: %w", err)
	}

	return &PlaylistInfo{
		ID:          string(playlist.ID),
		Name:        playlist.Name,
		Description: playlist.Description,
		URL:         playlist.ExternalURLs["spotify"],
		TrackCount:  len(tracks),
		CreatedAt:   time.Now(),
		Tracks:      tracks,
	}, nil
}

// ExportByEloRange exporte les tracks dans une plage d'Elo spécifique
func (pe *PlaylistExporter) ExportByEloRange(minElo, maxElo int, name string) (*PlaylistInfo, error) {
	// Récupérer tous les tracks et filtrer par Elo
	allTracks, err := pe.db.GetAllTracksWithRatings()
	if err != nil {
		return nil, fmt.Errorf("erreur récupération tracks: %w", err)
	}

	filteredTracks := make([]models.TrackWithRating, 0)
	for _, track := range allTracks {
		if track.Rating.Elo >= minElo && track.Rating.Elo <= maxElo {
			filteredTracks = append(filteredTracks, track)
		}
	}

	if len(filteredTracks) == 0 {
		return nil, fmt.Errorf("aucun track trouvé dans la plage Elo %d-%d", minElo, maxElo)
	}

	// Extraire les IDs
	trackIDs := make([]int64, len(filteredTracks))
	for i, track := range filteredTracks {
		trackIDs[i] = track.Track.ID
	}

	// Utiliser l'export personnalisé
	if name == "" {
		name = fmt.Sprintf("Song Battle Elo %d-%d", minElo, maxElo)
	}
	description := fmt.Sprintf("Chansons avec un Elo entre %d et %d - %d chansons - Créée le %s",
		minElo, maxElo, len(filteredTracks), time.Now().Format("02/01/2006"))

	return pe.ExportCustomPlaylist(trackIDs, name, description)
}

// GetExportHistory récupère l'historique des exports (simulé pour l'instant)
func (pe *PlaylistExporter) GetExportHistory() ([]PlaylistInfo, error) {
	// Pour l'instant, on retourne une liste vide
	// Dans une vraie implémentation, on stockerait l'historique en base
	return []PlaylistInfo{}, nil
}

// PlaylistInfo contient les informations d'une playlist exportée
type PlaylistInfo struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	URL         string                   `json:"url"`
	TrackCount  int                      `json:"track_count"`
	CreatedAt   time.Time                `json:"created_at"`
	Tracks      []models.TrackWithRating `json:"tracks,omitempty"`
}

// GetSummary retourne un résumé de la playlist
func (pi *PlaylistInfo) GetSummary() string {
	return fmt.Sprintf("🎵 %s\n📊 %d chansons\n🔗 %s\n📅 Créée le %s",
		pi.Name, pi.TrackCount, pi.URL, pi.CreatedAt.Format("02/01/2006"))
}

// ValidateExportParams valide les paramètres d'export
func ValidateExportParams(limit int) error {
	if limit <= 0 {
		return fmt.Errorf("la limite doit être positive")
	}
	if limit > 1000 {
		return fmt.Errorf("la limite ne peut pas dépasser 1000 tracks")
	}
	return nil
}

// GetRecommendedLimits retourne les limites recommandées pour l'export
func GetRecommendedLimits() map[string]int {
	return map[string]int{
		"small":  25,  // Playlist courte
		"medium": 50,  // Playlist moyenne
		"large":  100, // Playlist complète
		"max":    500, // Maximum recommandé
	}
}

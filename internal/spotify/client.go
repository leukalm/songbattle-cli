package spotify

import (
	"context"
	"fmt"
	"songbattle/internal/models"
	"strconv"
	"strings"
	"time"

	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
)

// Client wraps the Spotify API client
type Client struct {
	client   *spotify.Client
	context  context.Context
	clientID string
}

// NewClient crée un nouveau client Spotify
func NewClient(ctx context.Context, token *oauth2.Token, clientID string) *Client {
	auth := spotifyauth.New(spotifyauth.WithClientID(clientID))
	client := spotify.New(auth.Client(ctx, token))

	return &Client{
		client:   client,
		context:  ctx,
		clientID: clientID,
	}
}

// GetCurrentUser récupère l'utilisateur actuel
func (c *Client) GetCurrentUser() (*spotify.PrivateUser, error) {
	user, err := c.client.CurrentUser(c.context)
	return user, err
}

// GetUserTopTracks récupère les top tracks de l'utilisateur
func (c *Client) GetUserTopTracks(limit int, timeRange spotify.Range) ([]*models.Track, error) {
	topTracks, err := c.client.CurrentUsersTopTracks(c.context, spotify.Limit(limit), spotify.Timerange(timeRange))
	if err != nil {
		return nil, err
	}

	tracks := make([]*models.Track, 0, len(topTracks.Tracks))
	for _, item := range topTracks.Tracks {
		modelTrack := c.convertFullTrack(&item)
		tracks = append(tracks, modelTrack)
	}

	return tracks, nil
}

// GetRecommendations récupère des recommandations
func (c *Client) GetRecommendations(seedTracks, seedArtists, seedGenres []string, limit int) ([]*models.Track, error) {
	seeds := spotify.Seeds{}
	
	// Convertir les IDs en format Spotify
	for _, id := range seedTracks {
		seeds.Tracks = append(seeds.Tracks, spotify.ID(id))
	}
	for _, id := range seedArtists {
		seeds.Artists = append(seeds.Artists, spotify.ID(id))
	}
	for _, genre := range seedGenres {
		seeds.Genres = append(seeds.Genres, genre)
	}

	recommendations, err := c.client.GetRecommendations(c.context, seeds, nil, spotify.Limit(limit))
	if err != nil {
		return nil, err
	}

	tracks := make([]*models.Track, 0, len(recommendations.Tracks))
	for _, track := range recommendations.Tracks {
		modelTrack := c.convertSimpleTrack(&track)
		tracks = append(tracks, modelTrack)
	}

	return tracks, nil
}

// GetAudioFeatures récupère les caractéristiques audio d'un track
func (c *Client) GetAudioFeatures(trackID string) (*models.AudioFeatures, error) {
	af, err := c.client.GetAudioFeatures(c.context, spotify.ID(trackID))
	if err != nil {
		return nil, err
	}

	if len(af) == 0 {
		return nil, fmt.Errorf("aucune caractéristique audio trouvée")
	}

	features := af[0]
	return &models.AudioFeatures{
		Danceability:     float64(features.Danceability),
		Energy:           float64(features.Energy),
		Key:              int(features.Key),
		Loudness:         float64(features.Loudness),
		Mode:             int(features.Mode),
		Speechiness:      float64(features.Speechiness),
		Acousticness:     float64(features.Acousticness),
		Instrumentalness: float64(features.Instrumentalness),
		Liveness:         float64(features.Liveness),
		Valence:          float64(features.Valence),
		Tempo:            float64(features.Tempo),
		TimeSignature:    int(features.TimeSignature),
	}, nil
}

// PlayTrack joue un track sur l'appareil actif
func (c *Client) PlayTrack(uri string) error {
	uris := []spotify.URI{spotify.URI(uri)}
	
	playOptions := &spotify.PlayOptions{
		URIs: uris,
	}
	
	return c.client.PlayOpt(c.context, playOptions)
}

// CreatePlaylist crée une nouvelle playlist
func (c *Client) CreatePlaylist(userID, name, description string) (*spotify.FullPlaylist, error) {
	public := false
	playlist, err := c.client.CreatePlaylistForUser(c.context, userID, name, description, public, false)
	return playlist, err
}

// AddTracksToPlaylist ajoute des tracks à une playlist
func (c *Client) AddTracksToPlaylist(playlistID string, trackURIs []string) error {
	uris := make([]spotify.ID, len(trackURIs))
	for i, uri := range trackURIs {
		// Convertir spotify:track:ID en ID
		if strings.HasPrefix(uri, "spotify:track:") {
			uris[i] = spotify.ID(uri[14:])
		} else {
			uris[i] = spotify.ID(uri)
		}
	}

	_, err := c.client.AddTracksToPlaylist(c.context, spotify.ID(playlistID), uris...)
	return err
}

// EnrichTrackWithAudioFeatures enrichit un track avec ses caractéristiques audio
func (c *Client) EnrichTrackWithAudioFeatures(track *models.Track) error {
	features, err := c.GetAudioFeatures(track.SpotifyID)
	if err != nil {
		// Ne pas échouer si les audio features ne sont pas disponibles
		return nil
	}
	
	track.AudioFeaturesJSON = *features
	return nil
}

// Fonctions de conversion

// convertFullTrack convertit un FullTrack Spotify en model Track
func (c *Client) convertFullTrack(track *spotify.FullTrack) *models.Track {
	modelTrack := &models.Track{
		SpotifyID:  string(track.ID),
		Name:       track.Name,
		Artist:     c.joinArtists(track.Artists),
		Album:      track.Album.Name,
		SpotifyURI: string(track.URI),
		CreatedAt:  time.Now(),
	}

	// Preview URL
	if track.PreviewURL != "" {
		modelTrack.PreviewURL = &track.PreviewURL
	}

	// Année de sortie
	if track.Album.ReleaseDate != "" {
		if year, err := c.parseYear(track.Album.ReleaseDate); err == nil {
			modelTrack.Year = year
		}
	}

	// Genres (généralement vides pour les tracks, disponibles pour les artistes)
	modelTrack.GenresJSON = make(models.Genres, 0)

	return modelTrack
}

// convertSimpleTrack convertit un SimpleTrack Spotify en model Track
func (c *Client) convertSimpleTrack(track *spotify.SimpleTrack) *models.Track {
	modelTrack := &models.Track{
		SpotifyID:  string(track.ID),
		Name:       track.Name,
		Artist:     c.joinArtists(track.Artists),
		SpotifyURI: string(track.URI),
		CreatedAt:  time.Now(),
	}

	// Preview URL
	if track.PreviewURL != "" {
		modelTrack.PreviewURL = &track.PreviewURL
	}

	// Genres
	modelTrack.GenresJSON = make(models.Genres, 0)

	return modelTrack
}

// joinArtists joint les noms des artistes
func (c *Client) joinArtists(artists []spotify.SimpleArtist) string {
	names := make([]string, len(artists))
	for i, artist := range artists {
		names[i] = artist.Name
	}
	return strings.Join(names, ", ")
}

// parseYear parse l'année depuis une date de sortie
func (c *Client) parseYear(releaseDate string) (int, error) {
	// Format peut être YYYY, YYYY-MM, ou YYYY-MM-DD
	parts := strings.Split(releaseDate, "-")
	if len(parts) == 0 {
		return 0, fmt.Errorf("format de date invalide")
	}
	
	return strconv.Atoi(parts[0])
}
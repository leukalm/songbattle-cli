package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// Track représente une chanson avec toutes ses métadonnées
type Track struct {
	ID               int64     `json:"id" db:"id"`
	SpotifyID        string    `json:"spotify_id" db:"spotify_id"`
	Name             string    `json:"name" db:"name"`
	Artist           string    `json:"artist" db:"artist"`
	Album            string    `json:"album" db:"album"`
	Year             int       `json:"year" db:"year"`
	GenresJSON       Genres    `json:"genres" db:"genres_json"`
	SpotifyURI       string    `json:"spotify_uri" db:"spotify_uri"`
	PreviewURL       *string   `json:"preview_url" db:"preview_url"`
	AudioFeaturesJSON AudioFeatures `json:"audio_features" db:"audio_features_json"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
}

// Rating contient les statistiques Elo d'une chanson
type Rating struct {
	TrackID    int64     `json:"track_id" db:"track_id"`
	Elo        int       `json:"elo" db:"elo"`
	Wins       int       `json:"wins" db:"wins"`
	Losses     int       `json:"losses" db:"losses"`
	Draws      int       `json:"draws" db:"draws"`
	LastSeenAt time.Time `json:"last_seen_at" db:"last_seen_at"`
}

// Duel représente un combat entre deux chansons
type Duel struct {
	ID            int64     `json:"id" db:"id"`
	LeftTrackID   int64     `json:"left_track_id" db:"left_track_id"`
	RightTrackID  int64     `json:"right_track_id" db:"right_track_id"`
	WinnerTrackID *int64    `json:"winner_track_id" db:"winner_track_id"` // NULL si draw/skip
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// Meta stocke les métadonnées de l'application
type Meta struct {
	Key   string `json:"key" db:"key"`
	Value string `json:"value" db:"value"`
}

// Genres est un type personnalisé pour stocker la liste des genres en JSON
type Genres []string

// AudioFeatures contient les caractéristiques audio Spotify
type AudioFeatures struct {
	Danceability     float64 `json:"danceability"`
	Energy           float64 `json:"energy"`
	Key              int     `json:"key"`
	Loudness         float64 `json:"loudness"`
	Mode             int     `json:"mode"`
	Speechiness      float64 `json:"speechiness"`
	Acousticness     float64 `json:"acousticness"`
	Instrumentalness float64 `json:"instrumentalness"`
	Liveness         float64 `json:"liveness"`
	Valence          float64 `json:"valence"`
	Tempo            float64 `json:"tempo"`
	TimeSignature    int     `json:"time_signature"`
}

// Implémentation de l'interface sql.Scanner et driver.Valuer pour Genres
func (g *Genres) Scan(value interface{}) error {
	if value == nil {
		*g = make(Genres, 0)
		return nil
	}
	
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	
	return json.Unmarshal(bytes, g)
}

func (g Genres) Value() (driver.Value, error) {
	if g == nil {
		return "[]", nil
	}
	return json.Marshal(g)
}

// Implémentation de l'interface sql.Scanner et driver.Valuer pour AudioFeatures
func (af *AudioFeatures) Scan(value interface{}) error {
	if value == nil {
		*af = AudioFeatures{}
		return nil
	}
	
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	
	return json.Unmarshal(bytes, af)
}

func (af AudioFeatures) Value() (driver.Value, error) {
	return json.Marshal(af)
}

// TrackWithRating combine Track et Rating pour l'affichage
type TrackWithRating struct {
	Track  Track  `json:"track"`
	Rating Rating `json:"rating"`
}

// DuelResult représente le résultat d'un duel
type DuelResult struct {
	Winner string `json:"winner"` // "left", "right", "draw", "skip"
}

// Constants pour les résultats de duel
const (
	WinnerLeft  = "left"
	WinnerRight = "right"
	WinnerDraw  = "draw"
	WinnerSkip  = "skip"
)

// Constants pour les métadonnées
const (
	MetaKeyAccessToken  = "access_token"
	MetaKeyRefreshToken = "refresh_token"
	MetaKeyTokenExpiry  = "token_expiry"
	MetaKeyDeviceID     = "device_id"
	MetaKeyAppVersion   = "app_version"
)

// GetTotalBattles retourne le nombre total de duels d'un track
func (r *Rating) GetTotalBattles() int {
	return r.Wins + r.Losses + r.Draws
}

// GetWinRate retourne le taux de victoire en pourcentage
func (r *Rating) GetWinRate() float64 {
	total := r.GetTotalBattles()
	if total == 0 {
		return 0
	}
	return float64(r.Wins) / float64(total) * 100
}
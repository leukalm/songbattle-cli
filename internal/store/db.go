package store

import (
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"songbattle/internal/models"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

// NewDB initialise la connexion à la base de données et lance les migrations
func NewDB(dbPath string) (*DB, error) {
	// Créer le dossier parent si nécessaire
	dir := filepath.Dir(dbPath)
	if dir != "." {
		// Utilisation de mkdir pour créer le dossier (si nécessaire on peut importer os)
		// os.MkdirAll(dir, 0755)
	}

	db, err := sql.Open("sqlite", dbPath+"?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("erreur ouverture base de données: %w", err)
	}

	// Test de la connexion
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("erreur ping base de données: %w", err)
	}

	store := &DB{db}

	// Lancer les migrations
	if err := store.migrate(); err != nil {
		return nil, fmt.Errorf("erreur migration: %w", err)
	}

	log.Println("Base de données initialisée avec succès")
	return store, nil
}

// migrate crée les tables si elles n'existent pas
func (db *DB) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS tracks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			spotify_id TEXT UNIQUE NOT NULL,
			name TEXT NOT NULL,
			artist TEXT NOT NULL,
			album TEXT NOT NULL,
			year INTEGER DEFAULT 0,
			genres_json TEXT DEFAULT '[]',
			spotify_uri TEXT NOT NULL,
			preview_url TEXT,
			audio_features_json TEXT DEFAULT '{}',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS ratings (
			track_id INTEGER PRIMARY KEY,
			elo INTEGER DEFAULT 1200,
			wins INTEGER DEFAULT 0,
			losses INTEGER DEFAULT 0,
			draws INTEGER DEFAULT 0,
			last_seen_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (track_id) REFERENCES tracks(id) ON DELETE CASCADE
		)`,

		`CREATE TABLE IF NOT EXISTS duels (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			left_track_id INTEGER NOT NULL,
			right_track_id INTEGER NOT NULL,
			winner_track_id INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (left_track_id) REFERENCES tracks(id) ON DELETE CASCADE,
			FOREIGN KEY (right_track_id) REFERENCES tracks(id) ON DELETE CASCADE,
			FOREIGN KEY (winner_track_id) REFERENCES tracks(id) ON DELETE SET NULL
		)`,

		`CREATE TABLE IF NOT EXISTS meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,

		`CREATE INDEX IF NOT EXISTS idx_tracks_spotify_id ON tracks(spotify_id)`,
		`CREATE INDEX IF NOT EXISTS idx_ratings_elo ON ratings(elo DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_duels_created_at ON duels(created_at DESC)`,
	}

	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			return fmt.Errorf("erreur exécution migration: %w", err)
		}
	}

	return nil
}

// === TRACKS ===

// CreateTrack insère un nouveau track et son rating initial
func (db *DB) CreateTrack(track *models.Track) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insérer le track
	result, err := tx.Exec(`
		INSERT INTO tracks (spotify_id, name, artist, album, year, genres_json, spotify_uri, preview_url, audio_features_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		track.SpotifyID, track.Name, track.Artist, track.Album, track.Year,
		track.GenresJSON, track.SpotifyURI, track.PreviewURL, track.AudioFeaturesJSON)
	if err != nil {
		return err
	}

	trackID, err := result.LastInsertId()
	if err != nil {
		return err
	}
	track.ID = trackID

	// Créer le rating initial
	_, err = tx.Exec(`
		INSERT INTO ratings (track_id, elo, wins, losses, draws, last_seen_at)
		VALUES (?, 1200, 0, 0, 0, ?)`,
		trackID, time.Now())
	if err != nil {
		return err
	}

	return tx.Commit()
}

// GetTrackBySpotifyID récupère un track par son ID Spotify
func (db *DB) GetTrackBySpotifyID(spotifyID string) (*models.Track, error) {
	var track models.Track
	err := db.QueryRow(`
		SELECT id, spotify_id, name, artist, album, year, genres_json, spotify_uri, preview_url, audio_features_json, created_at
		FROM tracks WHERE spotify_id = ?`, spotifyID).Scan(
		&track.ID, &track.SpotifyID, &track.Name, &track.Artist, &track.Album, &track.Year,
		&track.GenresJSON, &track.SpotifyURI, &track.PreviewURL, &track.AudioFeaturesJSON, &track.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &track, nil
}

// GetTrackWithRating récupère un track avec son rating
func (db *DB) GetTrackWithRating(trackID int64) (*models.TrackWithRating, error) {
	var track models.Track
	var rating models.Rating

	err := db.QueryRow(`
		SELECT t.id, t.spotify_id, t.name, t.artist, t.album, t.year, t.genres_json, t.spotify_uri, t.preview_url, t.audio_features_json, t.created_at,
		       r.track_id, r.elo, r.wins, r.losses, r.draws, r.last_seen_at
		FROM tracks t
		JOIN ratings r ON t.id = r.track_id
		WHERE t.id = ?`, trackID).Scan(
		&track.ID, &track.SpotifyID, &track.Name, &track.Artist, &track.Album, &track.Year,
		&track.GenresJSON, &track.SpotifyURI, &track.PreviewURL, &track.AudioFeaturesJSON, &track.CreatedAt,
		&rating.TrackID, &rating.Elo, &rating.Wins, &rating.Losses, &rating.Draws, &rating.LastSeenAt)
	if err != nil {
		return nil, err
	}

	return &models.TrackWithRating{Track: track, Rating: rating}, nil
}

// GetAllTracksWithRatings récupère tous les tracks avec leurs ratings
func (db *DB) GetAllTracksWithRatings() ([]models.TrackWithRating, error) {
	rows, err := db.Query(`
		SELECT t.id, t.spotify_id, t.name, t.artist, t.album, t.year, t.genres_json, t.spotify_uri, t.preview_url, t.audio_features_json, t.created_at,
		       r.track_id, r.elo, r.wins, r.losses, r.draws, r.last_seen_at
		FROM tracks t
		JOIN ratings r ON t.id = r.track_id
		ORDER BY r.elo DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []models.TrackWithRating
	for rows.Next() {
		var track models.Track
		var rating models.Rating

		err := rows.Scan(
			&track.ID, &track.SpotifyID, &track.Name, &track.Artist, &track.Album, &track.Year,
			&track.GenresJSON, &track.SpotifyURI, &track.PreviewURL, &track.AudioFeaturesJSON, &track.CreatedAt,
			&rating.TrackID, &rating.Elo, &rating.Wins, &rating.Losses, &rating.Draws, &rating.LastSeenAt)
		if err != nil {
			return nil, err
		}

		tracks = append(tracks, models.TrackWithRating{Track: track, Rating: rating})
	}

	return tracks, nil
}

// === RATINGS ===

// UpdateRating met à jour les statistiques d'un track
func (db *DB) UpdateRating(rating *models.Rating) error {
	_, err := db.Exec(`
		UPDATE ratings SET elo = ?, wins = ?, losses = ?, draws = ?, last_seen_at = ?
		WHERE track_id = ?`,
		rating.Elo, rating.Wins, rating.Losses, rating.Draws, rating.LastSeenAt, rating.TrackID)
	return err
}

// GetRating récupère le rating d'un track
func (db *DB) GetRating(trackID int64) (*models.Rating, error) {
	var rating models.Rating
	err := db.QueryRow(`
		SELECT track_id, elo, wins, losses, draws, last_seen_at
		FROM ratings WHERE track_id = ?`, trackID).Scan(
		&rating.TrackID, &rating.Elo, &rating.Wins, &rating.Losses, &rating.Draws, &rating.LastSeenAt)
	if err != nil {
		return nil, err
	}
	return &rating, nil
}

// GetTopTracks récupère les N meilleurs tracks par Elo
func (db *DB) GetTopTracks(limit int) ([]models.TrackWithRating, error) {
	rows, err := db.Query(`
		SELECT t.id, t.spotify_id, t.name, t.artist, t.album, t.year, t.genres_json, t.spotify_uri, t.preview_url, t.audio_features_json, t.created_at,
		       r.track_id, r.elo, r.wins, r.losses, r.draws, r.last_seen_at
		FROM tracks t
		JOIN ratings r ON t.id = r.track_id
		ORDER BY r.elo DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []models.TrackWithRating
	for rows.Next() {
		var track models.Track
		var rating models.Rating

		err := rows.Scan(
			&track.ID, &track.SpotifyID, &track.Name, &track.Artist, &track.Album, &track.Year,
			&track.GenresJSON, &track.SpotifyURI, &track.PreviewURL, &track.AudioFeaturesJSON, &track.CreatedAt,
			&rating.TrackID, &rating.Elo, &rating.Wins, &rating.Losses, &rating.Draws, &rating.LastSeenAt)
		if err != nil {
			return nil, err
		}

		tracks = append(tracks, models.TrackWithRating{Track: track, Rating: rating})
	}

	return tracks, nil
}

// === DUELS ===

// CreateDuel enregistre un nouveau duel
func (db *DB) CreateDuel(duel *models.Duel) error {
	result, err := db.Exec(`
		INSERT INTO duels (left_track_id, right_track_id, winner_track_id, created_at)
		VALUES (?, ?, ?, ?)`,
		duel.LeftTrackID, duel.RightTrackID, duel.WinnerTrackID, duel.CreatedAt)
	if err != nil {
		return err
	}

	duelID, err := result.LastInsertId()
	if err != nil {
		return err
	}
	duel.ID = duelID

	return nil
}

// GetDuelHistory récupère l'historique des duels
func (db *DB) GetDuelHistory(limit int) ([]models.Duel, error) {
	rows, err := db.Query(`
		SELECT id, left_track_id, right_track_id, winner_track_id, created_at
		FROM duels
		ORDER BY created_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var duels []models.Duel
	for rows.Next() {
		var duel models.Duel
		err := rows.Scan(&duel.ID, &duel.LeftTrackID, &duel.RightTrackID, &duel.WinnerTrackID, &duel.CreatedAt)
		if err != nil {
			return nil, err
		}
		duels = append(duels, duel)
	}

	return duels, nil
}

// === META ===

// SetMeta sauvegarde une métadonnée
func (db *DB) SetMeta(key, value string) error {
	_, err := db.Exec(`
		INSERT OR REPLACE INTO meta (key, value) VALUES (?, ?)`, key, value)
	return err
}

// GetMeta récupère une métadonnée
func (db *DB) GetMeta(key string) (string, error) {
	var value string
	err := db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&value)
	return value, err
}

// DeleteMeta supprime une métadonnée
func (db *DB) DeleteMeta(key string) error {
	_, err := db.Exec(`DELETE FROM meta WHERE key = ?`, key)
	return err
}

// Close ferme la connexion à la base de données
func (db *DB) Close() error {
	return db.DB.Close()
}

package elo

import (
	"math"
	"songbattle/internal/models"
	"songbattle/internal/store"
	"time"
)

const (
	// Constantes Elo
	InitialElo = 1200
	MaxK       = 32 // Pour les nouveaux tracks
	MidK       = 24 // Pour les tracks avec quelques duels
	MinK       = 16 // Pour les tracks expérimentés

	// Seuils pour ajuster K
	NewPlayerThreshold         = 10 // Moins de 10 duels = nouveau
	ExperiencedPlayerThreshold = 30 // Plus de 30 duels = expérimenté
)

type EloSystem struct {
	db *store.DB
}

// NewEloSystem crée une nouvelle instance du système Elo
func NewEloSystem(db *store.DB) *EloSystem {
	return &EloSystem{db: db}
}

// CalculateExpectedScore calcule le score attendu pour le joueur A contre B
// E_A = 1 / (1 + 10^((Elo_B - Elo_A)/400))
func CalculateExpectedScore(eloA, eloB int) float64 {
	return 1.0 / (1.0 + math.Pow(10, float64(eloB-eloA)/400.0))
}

// GetKFactor calcule le facteur K basé sur l'expérience du joueur
func GetKFactor(totalBattles int) int {
	if totalBattles < NewPlayerThreshold {
		return MaxK
	} else if totalBattles < ExperiencedPlayerThreshold {
		return MidK
	}
	return MinK
}

// CalculateNewElo calcule le nouveau Elo après un duel
// Elo_new = Elo_old + K * (Score - Expected)
func CalculateNewElo(oldElo int, actualScore float64, expectedScore float64, kFactor int) int {
	newElo := float64(oldElo) + float64(kFactor)*(actualScore-expectedScore)
	return int(math.Round(newElo))
}

// ProcessDuel traite le résultat d'un duel et met à jour les Elos
func (es *EloSystem) ProcessDuel(leftTrackID, rightTrackID int64, result string) error {
	// Récupérer les ratings actuels
	leftRating, err := es.db.GetRating(leftTrackID)
	if err != nil {
		return err
	}

	rightRating, err := es.db.GetRating(rightTrackID)
	if err != nil {
		return err
	}

	// Déterminer les scores
	var leftScore, rightScore float64
	switch result {
	case models.WinnerLeft:
		leftScore, rightScore = 1.0, 0.0
	case models.WinnerRight:
		leftScore, rightScore = 0.0, 1.0
	case models.WinnerDraw:
		leftScore, rightScore = 0.5, 0.5
	case models.WinnerSkip:
		// Pas de changement d'Elo pour un skip
		return es.recordDuelWithoutEloChange(leftTrackID, rightTrackID, nil)
	default:
		return nil // Résultat invalide
	}

	// Calculer les scores attendus
	leftExpected := CalculateExpectedScore(leftRating.Elo, rightRating.Elo)
	rightExpected := CalculateExpectedScore(rightRating.Elo, leftRating.Elo)

	// Calculer les facteurs K
	leftK := GetKFactor(leftRating.GetTotalBattles())
	rightK := GetKFactor(rightRating.GetTotalBattles())

	// Calculer les nouveaux Elos
	newLeftElo := CalculateNewElo(leftRating.Elo, leftScore, leftExpected, leftK)
	newRightElo := CalculateNewElo(rightRating.Elo, rightScore, rightExpected, rightK)

	// Mettre à jour les statistiques
	leftRating.Elo = newLeftElo
	rightRating.Elo = newRightElo
	leftRating.LastSeenAt = time.Now()
	rightRating.LastSeenAt = time.Now()

	// Mettre à jour les compteurs de victoires/défaites
	if result == models.WinnerLeft {
		leftRating.Wins++
		rightRating.Losses++
	} else if result == models.WinnerRight {
		leftRating.Losses++
		rightRating.Wins++
	} else if result == models.WinnerDraw {
		leftRating.Draws++
		rightRating.Draws++
	}

	// Sauvegarder en base
	if err := es.db.UpdateRating(leftRating); err != nil {
		return err
	}
	if err := es.db.UpdateRating(rightRating); err != nil {
		return err
	}

	// Enregistrer le duel
	var winnerID *int64
	if result == models.WinnerLeft {
		winnerID = &leftTrackID
	} else if result == models.WinnerRight {
		winnerID = &rightTrackID
	}

	return es.recordDuelWithoutEloChange(leftTrackID, rightTrackID, winnerID)
}

// recordDuelWithoutEloChange enregistre juste le duel sans changer les Elos
func (es *EloSystem) recordDuelWithoutEloChange(leftTrackID, rightTrackID int64, winnerID *int64) error {
	duel := &models.Duel{
		LeftTrackID:   leftTrackID,
		RightTrackID:  rightTrackID,
		WinnerTrackID: winnerID,
		CreatedAt:     time.Now(),
	}

	return es.db.CreateDuel(duel)
}

// GetEloRanking retourne les tracks classés par Elo
func (es *EloSystem) GetEloRanking(limit int) ([]models.TrackWithRating, error) {
	return es.db.GetTopTracks(limit)
}

// EloChange représente un changement d'Elo pour l'affichage
type EloChange struct {
	TrackID int64
	OldElo  int
	NewElo  int
	Change  int
	Result  string
}

// SimulateDuel simule un duel pour prévoir les changements d'Elo
func (es *EloSystem) SimulateDuel(leftTrackID, rightTrackID int64, result string) ([]EloChange, error) {
	// Récupérer les ratings actuels
	leftRating, err := es.db.GetRating(leftTrackID)
	if err != nil {
		return nil, err
	}

	rightRating, err := es.db.GetRating(rightTrackID)
	if err != nil {
		return nil, err
	}

	// Déterminer les scores
	var leftScore, rightScore float64
	switch result {
	case models.WinnerLeft:
		leftScore, rightScore = 1.0, 0.0
	case models.WinnerRight:
		leftScore, rightScore = 0.0, 1.0
	case models.WinnerDraw:
		leftScore, rightScore = 0.5, 0.5
	case models.WinnerSkip:
		// Pas de changement pour un skip
		return []EloChange{
			{TrackID: leftTrackID, OldElo: leftRating.Elo, NewElo: leftRating.Elo, Change: 0, Result: result},
			{TrackID: rightTrackID, OldElo: rightRating.Elo, NewElo: rightRating.Elo, Change: 0, Result: result},
		}, nil
	default:
		return nil, nil
	}

	// Calculer les scores attendus
	leftExpected := CalculateExpectedScore(leftRating.Elo, rightRating.Elo)
	rightExpected := CalculateExpectedScore(rightRating.Elo, leftRating.Elo)

	// Calculer les facteurs K
	leftK := GetKFactor(leftRating.GetTotalBattles())
	rightK := GetKFactor(rightRating.GetTotalBattles())

	// Calculer les nouveaux Elos
	newLeftElo := CalculateNewElo(leftRating.Elo, leftScore, leftExpected, leftK)
	newRightElo := CalculateNewElo(rightRating.Elo, rightScore, rightExpected, rightK)

	return []EloChange{
		{
			TrackID: leftTrackID,
			OldElo:  leftRating.Elo,
			NewElo:  newLeftElo,
			Change:  newLeftElo - leftRating.Elo,
			Result:  result,
		},
		{
			TrackID: rightTrackID,
			OldElo:  rightRating.Elo,
			NewElo:  newRightElo,
			Change:  newRightElo - rightRating.Elo,
			Result:  result,
		},
	}, nil
}

// GetEloStats retourne des statistiques globales sur les Elos
func (es *EloSystem) GetEloStats() (map[string]interface{}, error) {
	tracks, err := es.db.GetAllTracksWithRatings()
	if err != nil {
		return nil, err
	}

	if len(tracks) == 0 {
		return map[string]interface{}{
			"total_tracks": 0,
			"average_elo":  0,
			"min_elo":      0,
			"max_elo":      0,
		}, nil
	}

	var totalElo, minElo, maxElo int
	minElo = tracks[0].Rating.Elo
	maxElo = tracks[0].Rating.Elo

	for _, track := range tracks {
		elo := track.Rating.Elo
		totalElo += elo
		if elo < minElo {
			minElo = elo
		}
		if elo > maxElo {
			maxElo = elo
		}
	}

	return map[string]interface{}{
		"total_tracks": len(tracks),
		"average_elo":  totalElo / len(tracks),
		"min_elo":      minElo,
		"max_elo":      maxElo,
	}, nil
}

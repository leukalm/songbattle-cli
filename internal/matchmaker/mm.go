package matchmaker

import (
	"fmt"
	"math/rand"
	"songbattle/internal/models"
	"songbattle/internal/store"
	"time"
)

const (
	// Paramètres du matchmaking
	EloRange             = 100  // Différence d'Elo acceptable pour un match équilibré
	ExplorationRate      = 0.15 // 15% des duels incluent un morceau peu joué
	MinBattlesForBalance = 5    // Minimum de duels avant d'utiliser le matchmaking équilibré
)

type Matchmaker struct {
	db   *store.DB
	rand *rand.Rand
}

// NewMatchmaker crée une nouvelle instance du matchmaker
func NewMatchmaker(db *store.DB) *Matchmaker {
	return &Matchmaker{
		db:   db,
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GetNextMatch sélectionne la prochaine paire de tracks pour un duel
func (mm *Matchmaker) GetNextMatch() (*models.TrackWithRating, *models.TrackWithRating, error) {
	// Récupérer tous les tracks avec leurs ratings
	allTracks, err := mm.db.GetAllTracksWithRatings()
	if err != nil {
		return nil, nil, fmt.Errorf("erreur récupération tracks: %w", err)
	}

	if len(allTracks) < 2 {
		return nil, nil, fmt.Errorf("besoin d'au moins 2 tracks pour un duel")
	}

	// Déterminer si on fait de l'exploration ou du matchmaking équilibré
	shouldExplore := mm.shouldExplore(allTracks)

	var leftTrack, rightTrack *models.TrackWithRating

	if shouldExplore {
		leftTrack, rightTrack = mm.explorationMatch(allTracks)
	} else {
		leftTrack, rightTrack = mm.balancedMatch(allTracks)
	}

	// Si le match équilibré échoue, faire un match aléatoire
	if leftTrack == nil || rightTrack == nil {
		leftTrack, rightTrack = mm.randomMatch(allTracks)
	}

	return leftTrack, rightTrack, nil
}

// shouldExplore détermine si on devrait faire un match d'exploration
func (mm *Matchmaker) shouldExplore(tracks []models.TrackWithRating) bool {
	// Calculer le nombre de tracks peu joués
	underplayedTracks := 0
	for _, track := range tracks {
		if track.Rating.GetTotalBattles() < MinBattlesForBalance {
			underplayedTracks++
		}
	}

	// Si plus de la moitié des tracks sont peu joués, toujours faire de l'exploration
	if float64(underplayedTracks)/float64(len(tracks)) > 0.5 {
		return true
	}

	// Sinon, utiliser le taux d'exploration
	return mm.rand.Float64() < ExplorationRate
}

// explorationMatch sélectionne un match incluant au moins un track peu joué
func (mm *Matchmaker) explorationMatch(tracks []models.TrackWithRating) (*models.TrackWithRating, *models.TrackWithRating) {
	// Séparer les tracks peu joués des autres
	underplayed := make([]models.TrackWithRating, 0)
	experienced := make([]models.TrackWithRating, 0)

	for _, track := range tracks {
		if track.Rating.GetTotalBattles() < MinBattlesForBalance {
			underplayed = append(underplayed, track)
		} else {
			experienced = append(experienced, track)
		}
	}

	if len(underplayed) == 0 {
		// Pas de tracks peu joués, faire un match normal
		return mm.balancedMatch(tracks)
	}

	// Sélectionner un track peu joué
	leftIdx := mm.rand.Intn(len(underplayed))
	leftTrack := &underplayed[leftIdx]

	// Sélectionner un adversaire (peut être peu joué ou expérimenté)
	allOthers := make([]models.TrackWithRating, 0)
	for i, track := range tracks {
		if int64(i) != leftTrack.Track.ID { // Éviter le même track
			allOthers = append(allOthers, track)
		}
	}

	if len(allOthers) == 0 {
		return nil, nil
	}

	rightIdx := mm.rand.Intn(len(allOthers))
	rightTrack := &allOthers[rightIdx]

	return leftTrack, rightTrack
}

// balancedMatch sélectionne un match équilibré basé sur l'Elo
func (mm *Matchmaker) balancedMatch(tracks []models.TrackWithRating) (*models.TrackWithRating, *models.TrackWithRating) {
	// Filtrer les tracks avec assez de duels pour un match équilibré
	experienced := make([]models.TrackWithRating, 0)
	for _, track := range tracks {
		if track.Rating.GetTotalBattles() >= MinBattlesForBalance {
			experienced = append(experienced, track)
		}
	}

	if len(experienced) < 2 {
		// Pas assez de tracks expérimentés, faire un match aléatoire
		return mm.randomMatch(tracks)
	}

	// Sélectionner le premier track aléatoirement
	leftIdx := mm.rand.Intn(len(experienced))
	leftTrack := &experienced[leftIdx]

	// Trouver un adversaire avec un Elo proche
	bestOpponent := mm.findBestOpponent(leftTrack, experienced)

	return leftTrack, bestOpponent
}

// findBestOpponent trouve le meilleur adversaire basé sur l'Elo
func (mm *Matchmaker) findBestOpponent(target *models.TrackWithRating, candidates []models.TrackWithRating) *models.TrackWithRating {
	var bestOpponent *models.TrackWithRating
	bestDifference := int(^uint(0) >> 1) // Max int

	for i := range candidates {
		candidate := &candidates[i]

		// Éviter le même track
		if candidate.Track.ID == target.Track.ID {
			continue
		}

		// Calculer la différence d'Elo
		eloDiff := abs(candidate.Rating.Elo - target.Rating.Elo)

		// Si dans la plage acceptable et meilleur que le précédent
		if eloDiff <= EloRange && eloDiff < bestDifference {
			bestOpponent = candidate
			bestDifference = eloDiff
		}
	}

	// Si aucun adversaire dans la plage, prendre le plus proche
	if bestOpponent == nil {
		for i := range candidates {
			candidate := &candidates[i]

			if candidate.Track.ID == target.Track.ID {
				continue
			}

			eloDiff := abs(candidate.Rating.Elo - target.Rating.Elo)
			if eloDiff < bestDifference {
				bestOpponent = candidate
				bestDifference = eloDiff
			}
		}
	}

	return bestOpponent
}

// randomMatch sélectionne deux tracks complètement au hasard
func (mm *Matchmaker) randomMatch(tracks []models.TrackWithRating) (*models.TrackWithRating, *models.TrackWithRating) {
	if len(tracks) < 2 {
		return nil, nil
	}

	// Sélectionner deux indices différents
	leftIdx := mm.rand.Intn(len(tracks))
	rightIdx := mm.rand.Intn(len(tracks))

	// S'assurer qu'ils sont différents
	for rightIdx == leftIdx {
		rightIdx = mm.rand.Intn(len(tracks))
	}

	return &tracks[leftIdx], &tracks[rightIdx]
}

// GetMatchQuality évalue la qualité d'un match potentiel
func (mm *Matchmaker) GetMatchQuality(left, right *models.TrackWithRating) string {
	eloDiff := abs(left.Rating.Elo - right.Rating.Elo)

	leftBattles := left.Rating.GetTotalBattles()
	rightBattles := right.Rating.GetTotalBattles()

	// Si l'un des deux est nouveau
	if leftBattles < MinBattlesForBalance || rightBattles < MinBattlesForBalance {
		return "Exploration"
	}

	// Basé sur la différence d'Elo
	if eloDiff <= 25 {
		return "Parfait"
	} else if eloDiff <= 50 {
		return "Excellent"
	} else if eloDiff <= EloRange {
		return "Bon"
	} else if eloDiff <= 200 {
		return "Moyen"
	} else {
		return "Déséquilibré"
	}
}

// GetRecentOpponents récupère les adversaires récents d'un track
func (mm *Matchmaker) GetRecentOpponents(trackID int64, limit int) ([]int64, error) {
	duels, err := mm.db.GetDuelHistory(limit * 2) // Plus large pour filtrer
	if err != nil {
		return nil, err
	}

	opponents := make([]int64, 0)
	seen := make(map[int64]bool)

	for _, duel := range duels {
		var opponentID int64

		if duel.LeftTrackID == trackID {
			opponentID = duel.RightTrackID
		} else if duel.RightTrackID == trackID {
			opponentID = duel.LeftTrackID
		} else {
			continue // Ce duel ne concerne pas ce track
		}

		if !seen[opponentID] {
			opponents = append(opponents, opponentID)
			seen[opponentID] = true

			if len(opponents) >= limit {
				break
			}
		}
	}

	return opponents, nil
}

// AvoidRecentOpponent modifie la sélection pour éviter les adversaires récents
func (mm *Matchmaker) AvoidRecentOpponent(target *models.TrackWithRating, candidates []models.TrackWithRating) *models.TrackWithRating {
	recentOpponents, err := mm.GetRecentOpponents(target.Track.ID, 3)
	if err != nil {
		// En cas d'erreur, faire un match normal
		return mm.findBestOpponent(target, candidates)
	}

	// Créer un map des adversaires récents pour un accès rapide
	recentMap := make(map[int64]bool)
	for _, opponentID := range recentOpponents {
		recentMap[opponentID] = true
	}

	// Filtrer les candidats pour éviter les adversaires récents
	filtered := make([]models.TrackWithRating, 0)
	for _, candidate := range candidates {
		if candidate.Track.ID != target.Track.ID && !recentMap[candidate.Track.ID] {
			filtered = append(filtered, candidate)
		}
	}

	// Si pas de candidats après filtrage, utiliser tous les candidats
	if len(filtered) == 0 {
		return mm.findBestOpponent(target, candidates)
	}

	return mm.findBestOpponent(target, filtered)
}

// abs retourne la valeur absolue d'un entier
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// GetMatchmakingStats retourne des statistiques sur le matchmaking
func (mm *Matchmaker) GetMatchmakingStats() (map[string]interface{}, error) {
	tracks, err := mm.db.GetAllTracksWithRatings()
	if err != nil {
		return nil, err
	}

	newTracks := 0
	experiencedTracks := 0

	for _, track := range tracks {
		if track.Rating.GetTotalBattles() < MinBattlesForBalance {
			newTracks++
		} else {
			experiencedTracks++
		}
	}

	return map[string]interface{}{
		"total_tracks":       len(tracks),
		"new_tracks":         newTracks,
		"experienced_tracks": experiencedTracks,
		"exploration_rate":   ExplorationRate,
		"elo_range":          EloRange,
	}, nil
}

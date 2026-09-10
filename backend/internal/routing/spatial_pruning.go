package routing

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

const nauticalMilesPerDegree = 60.0

type spatialCell struct {
	east  int64
	north int64
}

// PruneCandidatesSpatially keeps only the candidate with the greatest progress
// in each local geographic cell. The cell size is expressed in nautical miles.
func PruneCandidatesSpatially(
	candidates []CandidateProgress,
	cellSize navigation.NauticalMiles,
) ([]CandidateProgress, error) {
	cellSizeValue := float64(cellSize)
	if math.IsNaN(cellSizeValue) ||
		math.IsInf(cellSizeValue, 0) ||
		cellSize <= 0 {
		return nil, errors.New(
			"routing: la taille des cellules doit être un nombre fini strictement positif",
		)
	}
	if len(candidates) == 0 {
		return nil, errors.New(
			"routing: au moins un candidat est requis pour l'élagage spatial",
		)
	}

	origin := candidates[0].Candidate.Step.Position
	instant := candidates[0].Candidate.Step.Instant
	duration := candidates[0].Candidate.Step.Duration
	initialDistance := candidates[0].InitialDistance

	selected := make([]CandidateProgress, 0, len(candidates))
	cellIndexes := make(map[spatialCell]int, len(candidates))

	for index, candidate := range candidates {
		if err := validateSpatialCandidate(
			index,
			candidate,
			origin,
			instant,
			duration,
			initialDistance,
		); err != nil {
			return nil, err
		}

		cell, err := spatialCellFor(
			origin,
			candidate.Candidate.Step.NextPosition,
			cellSizeValue,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"routing: cellule du candidat %d impossible: %w",
				index+1,
				err,
			)
		}

		selectedIndex, exists := cellIndexes[cell]
		if !exists {
			cellIndexes[cell] = len(selected)
			selected = append(selected, candidate)
			continue
		}
		if candidate.Progress > selected[selectedIndex].Progress {
			selected[selectedIndex] = candidate
		}
	}

	sort.SliceStable(selected, func(left, right int) bool {
		return selected[left].Progress > selected[right].Progress
	})
	return selected, nil
}

// PruneSpatially returns a copy of the layer with spatially pruned candidates.
func (layer IsochroneLayer) PruneSpatially(
	cellSize navigation.NauticalMiles,
) (IsochroneLayer, error) {
	candidates, err := PruneCandidatesSpatially(
		layer.Candidates,
		cellSize,
	)
	if err != nil {
		return IsochroneLayer{}, fmt.Errorf(
			"routing: élagage de la couche impossible: %w",
			err,
		)
	}

	result := layer
	result.Candidates = candidates
	return result, nil
}

func validateSpatialCandidate(
	index int,
	candidate CandidateProgress,
	origin navigation.Position,
	instant time.Time,
	duration time.Duration,
	initialDistance navigation.NauticalMiles,
) error {
	if err := validateCandidateForRanking(
		index,
		candidate.Candidate,
		origin,
		instant,
		duration,
	); err != nil {
		return err
	}
	if err := candidate.InitialDistance.Validate(); err != nil {
		return fmt.Errorf(
			"routing: distance initiale du candidat %d invalide: %w",
			index+1,
			err,
		)
	}
	if candidate.InitialDistance != initialDistance {
		return fmt.Errorf(
			"routing: distance initiale incohérente pour le candidat %d",
			index+1,
		)
	}
	if err := candidate.RemainingDistance.Validate(); err != nil {
		return fmt.Errorf(
			"routing: distance restante du candidat %d invalide: %w",
			index+1,
			err,
		)
	}

	progress := float64(candidate.Progress)
	if math.IsNaN(progress) || math.IsInf(progress, 0) {
		return fmt.Errorf(
			"routing: progression du candidat %d invalide",
			index+1,
		)
	}
	expectedProgress := float64(
		candidate.InitialDistance - candidate.RemainingDistance,
	)
	if math.Abs(progress-expectedProgress) > 1e-9 {
		return fmt.Errorf(
			"routing: progression incohérente pour le candidat %d",
			index+1,
		)
	}
	return nil
}

func spatialCellFor(
	origin navigation.Position,
	position navigation.Position,
	cellSize float64,
) (spatialCell, error) {
	if err := origin.Validate(); err != nil {
		return spatialCell{}, fmt.Errorf(
			"origine invalide: %w",
			err,
		)
	}
	if err := position.Validate(); err != nil {
		return spatialCell{}, fmt.Errorf(
			"position invalide: %w",
			err,
		)
	}

	longitudeDelta, err := navigation.ShortestLongitudeDelta(
		origin.Longitude,
		position.Longitude,
	)
	if err != nil {
		return spatialCell{}, fmt.Errorf(
			"écart de longitude impossible: %w",
			err,
		)
	}

	northNauticalMiles := float64(
		position.Latitude-origin.Latitude,
	) * nauticalMilesPerDegree
	eastNauticalMiles := longitudeDelta *
		nauticalMilesPerDegree *
		math.Cos(float64(origin.Latitude)*math.Pi/180)

	east, err := spatialCellCoordinate(eastNauticalMiles, cellSize)
	if err != nil {
		return spatialCell{}, fmt.Errorf(
			"indice est impossible: %w",
			err,
		)
	}
	north, err := spatialCellCoordinate(northNauticalMiles, cellSize)
	if err != nil {
		return spatialCell{}, fmt.Errorf(
			"indice nord impossible: %w",
			err,
		)
	}
	return spatialCell{east: east, north: north}, nil
}

func spatialCellCoordinate(
	distance float64,
	cellSize float64,
) (int64, error) {
	const largestExactInteger = float64(1 << 53)

	coordinate := math.Floor(distance / cellSize)
	if math.IsNaN(coordinate) ||
		math.IsInf(coordinate, 0) ||
		math.Abs(coordinate) > largestExactInteger {
		return 0, errors.New(
			"taille de cellule trop petite pour calculer un indice sûr",
		)
	}
	return int64(coordinate), nil
}

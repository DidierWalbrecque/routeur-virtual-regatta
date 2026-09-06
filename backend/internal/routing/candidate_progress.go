package routing

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

// DistanceProgress is a signed progress toward a destination in nautical miles.
// A positive value means that the candidate moved closer to the destination.
type DistanceProgress float64

// CandidateProgress adds destination-relative distances to a heading candidate.
type CandidateProgress struct {
	Candidate         HeadingCandidate
	InitialDistance   navigation.NauticalMiles
	RemainingDistance navigation.NauticalMiles
	Progress          DistanceProgress
}

// RankCandidatesByProgress returns a new slice sorted from the greatest
// progress toward the destination to the smallest. Equal candidates retain
// their original order.
func RankCandidatesByProgress(
	destination navigation.Position,
	candidates []HeadingCandidate,
) ([]CandidateProgress, error) {
	if err := destination.Validate(); err != nil {
		return nil, fmt.Errorf(
			"routing: destination invalide: %w",
			err,
		)
	}
	if len(candidates) == 0 {
		return nil, errors.New(
			"routing: au moins un candidat est requis pour le classement",
		)
	}

	origin := candidates[0].Step.Position
	instant := candidates[0].Step.Instant
	duration := candidates[0].Step.Duration

	initialDistance, err := navigation.DistanceBetween(origin, destination)
	if err != nil {
		return nil, fmt.Errorf(
			"routing: calcul de la distance initiale impossible: %w",
			err,
		)
	}

	ranked := make([]CandidateProgress, 0, len(candidates))
	for index, candidate := range candidates {
		if err := validateCandidateForRanking(
			index,
			candidate,
			origin,
			instant,
			duration,
		); err != nil {
			return nil, err
		}

		remainingDistance, err := navigation.DistanceBetween(
			candidate.Step.NextPosition,
			destination,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"routing: calcul de la distance restante du candidat %d impossible: %w",
				index+1,
				err,
			)
		}

		ranked = append(ranked, CandidateProgress{
			Candidate:         candidate,
			InitialDistance:   initialDistance,
			RemainingDistance: remainingDistance,
			Progress: DistanceProgress(
				float64(initialDistance - remainingDistance),
			),
		})
	}

	sort.SliceStable(ranked, func(left, right int) bool {
		return ranked[left].Progress > ranked[right].Progress
	})
	return ranked, nil
}

func validateCandidateForRanking(
	index int,
	candidate HeadingCandidate,
	origin navigation.Position,
	instant time.Time,
	duration time.Duration,
) error {
	if err := candidate.Heading.Validate(); err != nil {
		return fmt.Errorf(
			"routing: cap du candidat %d invalide: %w",
			index+1,
			err,
		)
	}
	if candidate.Step.Heading != candidate.Heading {
		return fmt.Errorf(
			"routing: le cap simulé du candidat %d ne correspond pas au cap demandé",
			index+1,
		)
	}
	if candidate.Step.Position != origin ||
		!candidate.Step.Instant.Equal(instant) ||
		candidate.Step.Duration != duration {
		return fmt.Errorf(
			"routing: le candidat %d ne partage pas le même état initial",
			index+1,
		)
	}
	if err := candidate.Step.NextPosition.Validate(); err != nil {
		return fmt.Errorf(
			"routing: position finale du candidat %d invalide: %w",
			index+1,
			err,
		)
	}
	return nil
}

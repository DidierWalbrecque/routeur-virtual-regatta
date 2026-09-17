package routing

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/equipment"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/weather"
)

// PracticalHeadingInput describes several constant-heading simulations over the
// same horizon. Weather and boat performance are recalculated at every step.
type PracticalHeadingInput struct {
	Position     navigation.Position
	Destination  navigation.Position
	Instant      time.Time
	Duration     time.Duration
	StepDuration time.Duration
	Owned        equipment.EquipmentSet
	FixedSailID  polar.SailID
	Headings     []navigation.Heading
}

// PracticalHeadingCandidate contains one constant-heading simulation and its
// geographic progress toward the destination.
type PracticalHeadingCandidate struct {
	Heading           navigation.Heading
	Simulation        simulation.Result
	InitialDistance   navigation.NauticalMiles
	RemainingDistance navigation.NauticalMiles
	Progress          DistanceProgress
}

// EvaluatePracticalHeadingCandidates simulates every supplied heading for the
// complete horizon and sorts the results from greatest destination progress to
// smallest. Equal candidates retain their input order.
func EvaluatePracticalHeadingCandidates(
	ctx context.Context,
	weatherProvider weather.WeatherProvider,
	boatPolar *polar.Polar,
	input PracticalHeadingInput,
) ([]PracticalHeadingCandidate, error) {
	if ctx == nil {
		return nil, errors.New(
			"routing: évaluation des caps pratiques impossible: contexte requis",
		)
	}
	if err := input.Position.Validate(); err != nil {
		return nil, fmt.Errorf(
			"routing: position pratique initiale invalide: %w",
			err,
		)
	}
	if err := input.Destination.Validate(); err != nil {
		return nil, fmt.Errorf(
			"routing: destination pratique invalide: %w",
			err,
		)
	}
	if input.Instant.IsZero() {
		return nil, errors.New(
			"routing: instant initial pratique requis",
		)
	}
	if input.Duration <= 0 {
		return nil, errors.New(
			"routing: durée pratique strictement positive requise",
		)
	}
	if input.StepDuration <= 0 {
		return nil, errors.New(
			"routing: durée de pas pratique strictement positive requise",
		)
	}
	if err := input.Owned.Validate(); err != nil {
		return nil, fmt.Errorf(
			"routing: équipements pratiques invalides: %w",
			err,
		)
	}
	if len(input.Headings) == 0 {
		return nil, errors.New(
			"routing: au moins un cap pratique est requis",
		)
	}

	seen := make(map[navigation.Heading]struct{}, len(input.Headings))
	for index, heading := range input.Headings {
		if err := heading.Validate(); err != nil {
			return nil, fmt.Errorf(
				"routing: cap pratique %d invalide: %w",
				index+1,
				err,
			)
		}
		if _, exists := seen[heading]; exists {
			return nil, fmt.Errorf(
				"routing: cap pratique %d dupliqué: %.6f°",
				index+1,
				heading,
			)
		}
		seen[heading] = struct{}{}
	}

	initialDistance, err := navigation.DistanceBetween(
		input.Position,
		input.Destination,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"routing: distance pratique initiale impossible: %w",
			err,
		)
	}

	candidates := make(
		[]PracticalHeadingCandidate,
		0,
		len(input.Headings),
	)
	for index, heading := range input.Headings {
		result, err := simulation.Run(
			ctx,
			weatherProvider,
			boatPolar,
			simulation.Input{
				Position:     input.Position,
				Instant:      input.Instant,
				Heading:      heading,
				Duration:     input.Duration,
				StepDuration: input.StepDuration,
				Owned:        input.Owned,
				FixedSailID:  input.FixedSailID,
			},
		)
		if err != nil {
			return nil, fmt.Errorf(
				"routing: simulation du cap pratique %d à %.6f° impossible: %w",
				index+1,
				heading,
				err,
			)
		}
		remainingDistance, err := navigation.DistanceBetween(
			result.NextPosition,
			input.Destination,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"routing: distance restante du cap pratique %d impossible: %w",
				index+1,
				err,
			)
		}
		candidates = append(candidates, PracticalHeadingCandidate{
			Heading:           heading,
			Simulation:        result,
			InitialDistance:   initialDistance,
			RemainingDistance: remainingDistance,
			Progress: DistanceProgress(
				float64(initialDistance - remainingDistance),
			),
		})
	}

	sort.SliceStable(candidates, func(left, right int) bool {
		return candidates[left].Progress > candidates[right].Progress
	})
	return candidates, nil
}

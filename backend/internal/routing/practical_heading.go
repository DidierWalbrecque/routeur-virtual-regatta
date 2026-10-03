package routing

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"sync"
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
	Position                navigation.Position
	Destination             navigation.Position
	Instant                 time.Time
	Duration                time.Duration
	StepDuration            time.Duration
	Owned                   equipment.EquipmentSet
	FixedSailID             polar.SailID
	Headings                []navigation.Heading
	WeatherVariations       []PracticalWeatherVariation
	MaxScenarioProgressLoss navigation.NauticalMiles
	RequireRobustScenarios  bool
}

// PracticalHeadingCandidate contains one constant-heading simulation and its
// geographic progress toward the destination.
type PracticalHeadingCandidate struct {
	Heading              navigation.Heading
	Simulation           simulation.Result
	InitialDistance      navigation.NauticalMiles
	RemainingDistance    navigation.NauticalMiles
	Progress             DistanceProgress
	StabilityScore       float64
	WeatherScenarios     []PracticalHeadingWeatherScenario
	WorstCaseProgress    DistanceProgress
	ScenarioProgressLoss navigation.NauticalMiles
	Robust               bool
}

type practicalHeadingResult struct {
	candidate PracticalHeadingCandidate
	err       error
}

// PracticalHeadingWeatherScenario contains the result obtained by applying a
// weather variation to one practical heading.
type PracticalHeadingWeatherScenario struct {
	Variation         PracticalWeatherVariation
	Simulation        simulation.Result
	RemainingDistance navigation.NauticalMiles
	Progress          DistanceProgress
	StabilityScore    float64
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
	preparedEffects, err := equipment.ResolveEffects(boatPolar, input.Owned)
	if err != nil {
		return nil, fmt.Errorf(
			"routing: résolution des équipements pratiques impossible: %w",
			err,
		)
	}
	if err := input.MaxScenarioProgressLoss.Validate(); err != nil {
		return nil, fmt.Errorf(
			"routing: perte maximale de progression pratique invalide: %w",
			err,
		)
	}
	if len(input.Headings) == 0 {
		return nil, errors.New(
			"routing: au moins un cap pratique est requis",
		)
	}
	for index, variation := range input.WeatherVariations {
		if err := variation.Validate(); err != nil {
			return nil, fmt.Errorf(
				"routing: variation météo pratique %d invalide: %w",
				index+1,
				err,
			)
		}
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

	weatherCache := newConcurrentWeatherCache(weatherProvider)
	results := make([]practicalHeadingResult, len(input.Headings))
	jobs := make(chan int, len(input.Headings))
	for index := range input.Headings {
		jobs <- index
	}
	close(jobs)
	workerCount := runtime.GOMAXPROCS(0)
	if workerCount < 1 {
		workerCount = 1
	}
	if workerCount > len(input.Headings) {
		workerCount = len(input.Headings)
	}
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for worker := 0; worker < workerCount; worker++ {
		go func() {
			defer workers.Done()
			for index := range jobs {
				if err := ctx.Err(); err != nil {
					results[index].err = err
					continue
				}
				candidate, err := evaluatePracticalHeading(
					ctx,
					weatherCache,
					boatPolar,
					input,
					preparedEffects,
					input.Headings[index],
					initialDistance,
				)
				results[index] = practicalHeadingResult{
					candidate: candidate,
					err:       err,
				}
			}
		}()
	}
	workers.Wait()

	candidates := make([]PracticalHeadingCandidate, 0, len(input.Headings))
	for index, result := range results {
		if result.err != nil {
			return nil, fmt.Errorf(
				"routing: simulation du cap pratique %d à %.6f° impossible: %w",
				index+1,
				input.Headings[index],
				result.err,
			)
		}
		if input.RequireRobustScenarios && !result.candidate.Robust {
			continue
		}
		candidates = append(candidates, result.candidate)
	}
	if input.RequireRobustScenarios && len(candidates) == 0 {
		return nil, errors.New(
			"routing: aucun cap pratique robuste ne respecte la perte maximale de progression",
		)
	}

	sort.SliceStable(candidates, func(left, right int) bool {
		return candidates[left].Progress > candidates[right].Progress
	})
	return candidates, nil
}

func evaluatePracticalHeading(
	ctx context.Context,
	weatherProvider weather.WeatherProvider,
	boatPolar *polar.Polar,
	input PracticalHeadingInput,
	effects equipment.Effects,
	heading navigation.Heading,
	initialDistance navigation.NauticalMiles,
) (PracticalHeadingCandidate, error) {
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
			Effects:      &effects,
		},
	)
	if err != nil {
		return PracticalHeadingCandidate{}, err
	}
	remainingDistance, err := navigation.DistanceBetween(
		result.NextPosition,
		input.Destination,
	)
	if err != nil {
		return PracticalHeadingCandidate{}, err
	}
	candidate := PracticalHeadingCandidate{
		Heading:           heading,
		Simulation:        result,
		InitialDistance:   initialDistance,
		RemainingDistance: remainingDistance,
		Progress: DistanceProgress(
			float64(initialDistance - remainingDistance),
		),
		StabilityScore: practicalStabilityScore(result),
	}
	for _, variation := range input.WeatherVariations {
		variedResult, err := simulation.Run(
			ctx,
			practicalWeatherVariationProvider{
				provider:  weatherProvider,
				variation: variation,
			},
			boatPolar,
			simulation.Input{
				Position:     input.Position,
				Instant:      input.Instant,
				Heading:      heading,
				Duration:     input.Duration,
				StepDuration: input.StepDuration,
				Owned:        input.Owned,
				FixedSailID:  input.FixedSailID,
				Effects:      &effects,
			},
		)
		if err != nil {
			return PracticalHeadingCandidate{}, err
		}
		variedRemainingDistance, err := navigation.DistanceBetween(
			variedResult.NextPosition,
			input.Destination,
		)
		if err != nil {
			return PracticalHeadingCandidate{}, err
		}
		candidate.WeatherScenarios = append(
			candidate.WeatherScenarios,
			PracticalHeadingWeatherScenario{
				Variation:         variation,
				Simulation:        variedResult,
				RemainingDistance: variedRemainingDistance,
				Progress: DistanceProgress(
					float64(initialDistance - variedRemainingDistance),
				),
				StabilityScore: practicalStabilityScore(variedResult),
			},
		)
	}
	candidate.WorstCaseProgress,
		candidate.ScenarioProgressLoss,
		candidate.Robust = practicalRobustness(
		candidate.Progress,
		candidate.WeatherScenarios,
		input.MaxScenarioProgressLoss,
	)
	return candidate, nil
}

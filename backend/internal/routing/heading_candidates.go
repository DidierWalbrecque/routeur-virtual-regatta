// Package routing explores navigation choices and builds routes from
// simulation results.
package routing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/equipment"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/weather"
)

// HeadingCandidatesInput describes the common state used to evaluate several
// headings independently.
type HeadingCandidatesInput struct {
	Position navigation.Position
	Instant  time.Time
	Duration time.Duration
	Owned    equipment.EquipmentSet
	Headings []navigation.Heading
}

// HeadingCandidate associates a requested heading with its simulated step.
type HeadingCandidate struct {
	Heading navigation.Heading
	Step    simulation.StepResult
}

type headingWeatherCache struct {
	provider weather.WeatherProvider
	position navigation.Position
	instant  time.Time
	loaded   bool
	wind     navigation.Wind
	err      error
}

func (cache *headingWeatherCache) WindAt(
	ctx context.Context,
	position navigation.Position,
	instant time.Time,
) (navigation.Wind, error) {
	if cache.provider == nil {
		return navigation.Wind{}, errors.New("routing: fournisseur météo requis")
	}
	if cache.loaded && position == cache.position && instant.Equal(cache.instant) {
		return cache.wind, cache.err
	}
	wind, err := cache.provider.WindAt(ctx, position, instant)
	if position == cache.position && instant.Equal(cache.instant) {
		cache.loaded = true
		cache.wind = wind
		cache.err = err
	}
	return wind, err
}

// EvaluateHeadingCandidates simulates each heading from the exact same initial
// state and preserves the order supplied by the caller.
func EvaluateHeadingCandidates(
	ctx context.Context,
	weatherProvider weather.WeatherProvider,
	boatPolar *polar.Polar,
	input HeadingCandidatesInput,
) ([]HeadingCandidate, error) {
	if len(input.Headings) == 0 {
		return nil, errors.New(
			"routing: au moins un cap candidat est requis",
		)
	}

	seen := make(map[navigation.Heading]struct{}, len(input.Headings))
	for index, heading := range input.Headings {
		if err := heading.Validate(); err != nil {
			return nil, fmt.Errorf(
				"routing: cap du candidat %d invalide: %w",
				index+1,
				err,
			)
		}
		if _, exists := seen[heading]; exists {
			return nil, fmt.Errorf(
				"routing: cap dupliqué pour le candidat %d: %.6f°",
				index+1,
				heading,
			)
		}
		seen[heading] = struct{}{}
	}

	candidates := make([]HeadingCandidate, 0, len(input.Headings))
	cachedWeather := &headingWeatherCache{
		provider: weatherProvider,
		position: input.Position,
		instant:  input.Instant,
	}
	for index, heading := range input.Headings {
		step, err := simulation.EvaluateStep(
			ctx,
			cachedWeather,
			boatPolar,
			simulation.StepInput{
				Position: input.Position,
				Instant:  input.Instant,
				Heading:  heading,
				Duration: input.Duration,
				Owned:    input.Owned,
			},
		)
		if err != nil {
			return nil, fmt.Errorf(
				"routing: évaluation du candidat %d au cap %.6f° impossible: %w",
				index+1,
				heading,
				err,
			)
		}
		candidates = append(candidates, HeadingCandidate{
			Heading: heading,
			Step:    step,
		})
	}

	return candidates, nil
}

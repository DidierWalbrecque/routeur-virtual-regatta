package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/equipment"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/weather"
)

type raceSimulationInput struct {
	Position    navigation.Position
	Instant     time.Time
	Heading     navigation.Heading
	Minutes     float64
	StepMinutes float64
	Owned       equipment.EquipmentSet
}

type raceSimulationResult struct {
	Steps        []raceStepResult
	Distance     navigation.NauticalMiles
	NextPosition navigation.Position
	NextInstant  time.Time
}

func simulateRace(
	ctx context.Context,
	weatherProvider weather.WeatherProvider,
	boatPolar *polar.Polar,
	input raceSimulationInput,
) (raceSimulationResult, error) {
	if math.IsNaN(input.Minutes) ||
		math.IsInf(input.Minutes, 0) ||
		input.Minutes <= 0 {
		return raceSimulationResult{}, errors.New(
			"racecheck: la durée totale doit être strictement positive",
		)
	}
	if math.IsNaN(input.StepMinutes) ||
		math.IsInf(input.StepMinutes, 0) ||
		input.StepMinutes <= 0 {
		return raceSimulationResult{}, errors.New(
			"racecheck: la durée d'un pas doit être strictement positive",
		)
	}

	result := raceSimulationResult{
		NextPosition: input.Position,
		NextInstant:  input.Instant,
	}
	remainingMinutes := input.Minutes

	for remainingMinutes > 0 {
		currentMinutes := math.Min(input.StepMinutes, remainingMinutes)
		step, err := evaluateRaceStep(
			ctx,
			weatherProvider,
			boatPolar,
			raceStepInput{
				Position: result.NextPosition,
				Instant:  result.NextInstant,
				Heading:  input.Heading,
				Minutes:  currentMinutes,
				Owned:    input.Owned,
			},
		)
		if err != nil {
			return raceSimulationResult{}, fmt.Errorf(
				"racecheck: pas %d impossible: %w",
				len(result.Steps)+1,
				err,
			)
		}

		result.Steps = append(result.Steps, step)
		result.Distance += step.Distance
		result.NextPosition = step.NextPosition
		result.NextInstant = step.NextInstant

		remainingMinutes -= currentMinutes
		if remainingMinutes < 1e-9 {
			remainingMinutes = 0
		}
	}

	return result, nil
}

// Package simulation combines weather, polar, equipment and navigation
// calculations to advance a boat state through time.
package simulation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/equipment"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/weather"
)

// StepInput describes one constant-heading movement.
type StepInput struct {
	Position navigation.Position
	Instant  time.Time
	Heading  navigation.Heading
	Duration time.Duration
	Owned    equipment.EquipmentSet
}

// StepResult contains the weather and performance used for one movement.
type StepResult struct {
	Position     navigation.Position
	Instant      time.Time
	Heading      navigation.Heading
	Duration     time.Duration
	Wind         navigation.Wind
	WindKnots    navigation.Knots
	WindAngle    navigation.TrueWindAngle
	Tack         navigation.Tack
	BestSail     polar.SailPerformance
	Distance     navigation.NauticalMiles
	NextPosition navigation.Position
	NextInstant  time.Time
}

// EvaluateStep advances the boat once at a constant heading.
func EvaluateStep(
	ctx context.Context,
	weatherProvider weather.WeatherProvider,
	boatPolar *polar.Polar,
	input StepInput,
) (StepResult, error) {
	if weatherProvider == nil {
		return StepResult{}, errors.New(
			"simulation: fournisseur météo requis",
		)
	}
	if boatPolar == nil {
		return StepResult{}, errors.New("simulation: polaire requise")
	}
	if err := input.Position.Validate(); err != nil {
		return StepResult{}, fmt.Errorf(
			"simulation: position invalide: %w",
			err,
		)
	}
	if input.Instant.IsZero() {
		return StepResult{}, errors.New(
			"simulation: instant initial requis",
		)
	}
	if err := input.Heading.Validate(); err != nil {
		return StepResult{}, fmt.Errorf(
			"simulation: cap invalide: %w",
			err,
		)
	}
	if input.Duration <= 0 {
		return StepResult{}, errors.New(
			"simulation: la durée doit être strictement positive",
		)
	}
	if err := input.Owned.Validate(); err != nil {
		return StepResult{}, fmt.Errorf(
			"simulation: équipements invalides: %w",
			err,
		)
	}

	wind, err := weatherProvider.WindAt(
		ctx,
		input.Position,
		input.Instant,
	)
	if err != nil {
		return StepResult{}, fmt.Errorf(
			"simulation: calcul du vent impossible: %w",
			err,
		)
	}
	windKnots, err := navigation.KnotsFromMetersPerSecond(wind.Speed)
	if err != nil {
		return StepResult{}, fmt.Errorf(
			"simulation: conversion du vent impossible: %w",
			err,
		)
	}
	windAngle, err := navigation.TrueWindAngleFromHeading(
		input.Heading,
		wind.Direction,
	)
	if err != nil {
		return StepResult{}, fmt.Errorf(
			"simulation: calcul du TWA impossible: %w",
			err,
		)
	}
	tack, err := navigation.TackFromHeading(
		input.Heading,
		wind.Direction,
	)
	if err != nil {
		return StepResult{}, fmt.Errorf(
			"simulation: calcul de l'amure impossible: %w",
			err,
		)
	}

	effects, err := equipment.ResolveEffects(boatPolar, input.Owned)
	if err != nil {
		return StepResult{}, fmt.Errorf(
			"simulation: résolution des équipements impossible: %w",
			err,
		)
	}
	bestSail, err := boatPolar.BestEffectiveSailAt(
		polar.WindSpeed(windKnots),
		polar.TrueWindAngle(windAngle),
		effects.AvailableSails,
		effects.SpeedOptions,
	)
	if err != nil {
		return StepResult{}, fmt.Errorf(
			"simulation: calcul de la vitesse impossible: %w",
			err,
		)
	}

	distance, err := navigation.DistanceAtConstantSpeed(
		navigation.Knots(bestSail.Speed),
		navigation.Seconds(input.Duration.Seconds()),
	)
	if err != nil {
		return StepResult{}, fmt.Errorf(
			"simulation: calcul de la distance impossible: %w",
			err,
		)
	}
	nextPosition, err := navigation.DestinationPosition(
		input.Position,
		input.Heading,
		distance,
	)
	if err != nil {
		return StepResult{}, fmt.Errorf(
			"simulation: calcul de la position suivante impossible: %w",
			err,
		)
	}

	return StepResult{
		Position:     input.Position,
		Instant:      input.Instant,
		Heading:      input.Heading,
		Duration:     input.Duration,
		Wind:         wind,
		WindKnots:    windKnots,
		WindAngle:    windAngle,
		Tack:         tack,
		BestSail:     bestSail,
		Distance:     distance,
		NextPosition: nextPosition,
		NextInstant:  input.Instant.Add(input.Duration),
	}, nil
}

// Input describes a constant-heading simulation split into bounded steps.
type Input struct {
	Position     navigation.Position
	Instant      time.Time
	Heading      navigation.Heading
	Duration     time.Duration
	StepDuration time.Duration
	Owned        equipment.EquipmentSet
}

// Result contains every evaluated step and the final boat state.
type Result struct {
	Steps        []StepResult
	Distance     navigation.NauticalMiles
	NextPosition navigation.Position
	NextInstant  time.Time
}

// Run simulates a constant heading and recalculates weather and performance at
// the beginning of every step.
func Run(
	ctx context.Context,
	weatherProvider weather.WeatherProvider,
	boatPolar *polar.Polar,
	input Input,
) (Result, error) {
	if input.Duration <= 0 {
		return Result{}, errors.New(
			"simulation: la durée totale doit être strictement positive",
		)
	}
	if input.StepDuration <= 0 {
		return Result{}, errors.New(
			"simulation: la durée d'un pas doit être strictement positive",
		)
	}

	result := Result{
		NextPosition: input.Position,
		NextInstant:  input.Instant,
	}
	remaining := input.Duration

	for remaining > 0 {
		currentDuration := min(input.StepDuration, remaining)
		step, err := EvaluateStep(
			ctx,
			weatherProvider,
			boatPolar,
			StepInput{
				Position: result.NextPosition,
				Instant:  result.NextInstant,
				Heading:  input.Heading,
				Duration: currentDuration,
				Owned:    input.Owned,
			},
		)
		if err != nil {
			return Result{}, fmt.Errorf(
				"simulation: pas %d impossible: %w",
				len(result.Steps)+1,
				err,
			)
		}

		result.Steps = append(result.Steps, step)
		result.Distance += step.Distance
		result.NextPosition = step.NextPosition
		result.NextInstant = step.NextInstant
		remaining -= currentDuration
	}

	return result, nil
}

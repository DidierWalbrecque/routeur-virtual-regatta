package navigation

import (
	"fmt"
	"math"
)

const (
	metersPerNauticalMile = 1852.0
	secondsPerHour        = 3600.0
)

// Knots représente une vitesse en milles nautiques par heure.
type Knots float64

// MetersPerSecond représente une vitesse en mètres par seconde.
type MetersPerSecond float64

// Seconds représente une durée en secondes.
type Seconds float64

// Validate vérifie que la vitesse en nœuds est finie et positive ou nulle.
func (speed Knots) Validate() error {
	value := float64(speed)
	if math.IsNaN(value) || math.IsInf(value, 0) || speed < 0 {
		return fmt.Errorf(
			"navigation: la vitesse en nœuds doit être un nombre fini positif ou nul",
		)
	}
	return nil
}

// Validate vérifie que la vitesse en mètres par seconde est finie et positive
// ou nulle.
func (speed MetersPerSecond) Validate() error {
	value := float64(speed)
	if math.IsNaN(value) || math.IsInf(value, 0) || speed < 0 {
		return fmt.Errorf(
			"navigation: la vitesse en mètres par seconde doit être un nombre fini positif ou nul",
		)
	}
	return nil
}

// Validate vérifie que la durée est finie et positive ou nulle.
func (duration Seconds) Validate() error {
	value := float64(duration)
	if math.IsNaN(value) || math.IsInf(value, 0) || duration < 0 {
		return fmt.Errorf(
			"navigation: la durée en secondes doit être un nombre fini positif ou nul",
		)
	}
	return nil
}

// MetersPerSecondFromKnots convertit une vitesse exprimée en nœuds.
func MetersPerSecondFromKnots(speed Knots) (MetersPerSecond, error) {
	if err := speed.Validate(); err != nil {
		return 0, fmt.Errorf(
			"navigation: conversion des nœuds impossible: %w",
			err,
		)
	}
	return MetersPerSecond(
		float64(speed) * metersPerNauticalMile / secondsPerHour,
	), nil
}

// KnotsFromMetersPerSecond convertit une vitesse exprimée en mètres par
// seconde.
func KnotsFromMetersPerSecond(speed MetersPerSecond) (Knots, error) {
	if err := speed.Validate(); err != nil {
		return 0, fmt.Errorf(
			"navigation: conversion des mètres par seconde impossible: %w",
			err,
		)
	}
	return Knots(
		float64(speed) * secondsPerHour / metersPerNauticalMile,
	), nil
}

// DistanceAtConstantSpeed calcule la distance parcourue à vitesse constante.
// Les nœuds étant des milles nautiques par heure, la durée est convertie de
// secondes en heures.
func DistanceAtConstantSpeed(
	speed Knots,
	duration Seconds,
) (NauticalMiles, error) {
	if err := speed.Validate(); err != nil {
		return 0, fmt.Errorf(
			"navigation: calcul de distance impossible: %w",
			err,
		)
	}
	if err := duration.Validate(); err != nil {
		return 0, fmt.Errorf(
			"navigation: calcul de distance impossible: %w",
			err,
		)
	}

	distance := NauticalMiles(
		float64(speed) * float64(duration) / secondsPerHour,
	)
	if err := distance.Validate(); err != nil {
		return 0, fmt.Errorf(
			"navigation: distance calculée invalide: %w",
			err,
		)
	}
	return distance, nil
}

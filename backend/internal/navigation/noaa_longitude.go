package navigation

import (
	"fmt"
	"math"
)

// NOAALongitude représente une longitude de grille NOAA en degrés, comprise
// entre 0 inclus et 360 exclus.
type NOAALongitude float64

// Validate vérifie que la longitude NOAA est finie et appartient à [0, 360[.
func (longitude NOAALongitude) Validate() error {
	value := float64(longitude)
	if math.IsNaN(value) || math.IsInf(value, 0) ||
		longitude < 0 || longitude >= 360 {
		return fmt.Errorf(
			"navigation: la longitude NOAA doit être un nombre fini compris entre 0 inclus et 360 degrés exclus",
		)
	}
	return nil
}

// LongitudeFromNOAA convertit une longitude NOAA [0, 360[ en longitude
// géographique canonique [-180, 180[.
func LongitudeFromNOAA(longitude NOAALongitude) (Longitude, error) {
	if err := longitude.Validate(); err != nil {
		return 0, fmt.Errorf(
			"navigation: conversion de la longitude NOAA impossible: %w",
			err,
		)
	}

	converted, err := NormalizeLongitude(float64(longitude))
	if err != nil {
		return 0, fmt.Errorf(
			"navigation: conversion de la longitude NOAA impossible: %w",
			err,
		)
	}
	return converted, nil
}

// NOAALongitudeFromLongitude convertit une longitude géographique [-180, 180]
// en longitude NOAA [0, 360[.
func NOAALongitudeFromLongitude(longitude Longitude) (NOAALongitude, error) {
	if err := longitude.Validate(); err != nil {
		return 0, fmt.Errorf(
			"navigation: conversion vers la longitude NOAA impossible: %w",
			err,
		)
	}

	value := float64(longitude)
	if value < 0 {
		value += 360
	}

	converted := NOAALongitude(value)
	if err := converted.Validate(); err != nil {
		return 0, fmt.Errorf(
			"navigation: longitude NOAA convertie invalide: %w",
			err,
		)
	}
	return converted, nil
}

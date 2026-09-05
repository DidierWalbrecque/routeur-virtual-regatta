package weather

import (
	"fmt"
	"math"
	"sort"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

const interpolationTolerance = 1e-9

// ComponentsAt interpole bilinéairement les composantes U et V à la position
// demandée. La grille est parcourue du nord au sud et d'ouest en est.
//
// Une grille mondiale est considérée comme cyclique en longitude : une
// position proche de 360 degrés est interpolée entre la dernière colonne et
// la première.
func (grid GRIB2WindGrid) ComponentsAt(
	position navigation.Position,
) (navigation.WindComponent, navigation.WindComponent, error) {
	if err := position.Validate(); err != nil {
		return 0, 0, fmt.Errorf(
			"weather: interpolation spatiale impossible: position invalide: %w",
			err,
		)
	}
	if err := grid.validateInterpolationShape(); err != nil {
		return 0, 0, fmt.Errorf(
			"weather: interpolation spatiale impossible: %w",
			err,
		)
	}

	north, south, latitudeRatio, err := grid.latitudeBounds(
		float64(position.Latitude),
	)
	if err != nil {
		return 0, 0, fmt.Errorf(
			"weather: interpolation spatiale impossible: %w",
			err,
		)
	}

	longitude := grid.longitudeForPosition(position.Longitude)
	west, east, longitudeRatio, err := grid.longitudeBounds(longitude)
	if err != nil {
		return 0, 0, fmt.Errorf(
			"weather: interpolation spatiale impossible: %w",
			err,
		)
	}

	u := grid.interpolateComponent(
		grid.U,
		north,
		south,
		west,
		east,
		latitudeRatio,
		longitudeRatio,
	)
	v := grid.interpolateComponent(
		grid.V,
		north,
		south,
		west,
		east,
		latitudeRatio,
		longitudeRatio,
	)

	return navigation.WindComponent(u), navigation.WindComponent(v), nil
}

// WindAtPosition retourne la force et la direction météorologique du vent à
// une position pour l'échéance de cette grille.
func (grid GRIB2WindGrid) WindAtPosition(
	position navigation.Position,
) (navigation.Wind, error) {
	u, v, err := grid.ComponentsAt(position)
	if err != nil {
		return navigation.Wind{}, err
	}

	wind, err := navigation.WindFromComponents(u, v)
	if err != nil {
		return navigation.Wind{}, fmt.Errorf(
			"weather: conversion des composantes interpolées impossible: %w",
			err,
		)
	}
	return wind, nil
}

func (grid GRIB2WindGrid) validateInterpolationShape() error {
	if grid.Width <= 0 || grid.Height <= 0 {
		return fmt.Errorf(
			"dimensions de grille invalides: %dx%d",
			grid.Width,
			grid.Height,
		)
	}
	maxInt := int(^uint(0) >> 1)
	if grid.Width > maxInt/grid.Height {
		return fmt.Errorf("dimensions de grille trop grandes")
	}

	expected := grid.Width * grid.Height
	if len(grid.Latitudes) != expected ||
		len(grid.Longitudes) != expected ||
		len(grid.U) != expected ||
		len(grid.V) != expected {
		return fmt.Errorf(
			"dimensions et tableaux de grille incompatibles",
		)
	}

	for row := 1; row < grid.Height; row++ {
		previous := grid.Latitudes[(row-1)*grid.Width]
		current := grid.Latitudes[row*grid.Width]
		if !finite(previous) || !finite(current) || current >= previous {
			return fmt.Errorf(
				"axe des latitudes non strictement décroissant à la ligne %d",
				row,
			)
		}
	}
	for column := 1; column < grid.Width; column++ {
		previous := grid.unwrappedLongitudeAt(column - 1)
		current := grid.unwrappedLongitudeAt(column)
		if !finite(previous) || !finite(current) || current <= previous {
			return fmt.Errorf(
				"axe des longitudes non strictement croissant à la colonne %d",
				column,
			)
		}
	}

	return nil
}

func (grid GRIB2WindGrid) latitudeBounds(
	latitude float64,
) (int, int, float64, error) {
	northernmost := grid.Latitudes[0]
	southernmost := grid.Latitudes[(grid.Height-1)*grid.Width]
	if latitude > northernmost+interpolationTolerance ||
		latitude < southernmost-interpolationTolerance {
		return 0, 0, 0, fmt.Errorf(
			"latitude %.6f hors de la grille [%.6f, %.6f]",
			latitude,
			southernmost,
			northernmost,
		)
	}

	south := sort.Search(grid.Height, func(row int) bool {
		return grid.Latitudes[row*grid.Width] <= latitude
	})
	if south == grid.Height {
		south = grid.Height - 1
	}
	southLatitude := grid.Latitudes[south*grid.Width]
	if nearlyEqual(southLatitude, latitude) || south == 0 {
		return south, south, 0, nil
	}

	north := south - 1
	northLatitude := grid.Latitudes[north*grid.Width]
	ratio := (northLatitude - latitude) / (northLatitude - southLatitude)
	return north, south, ratio, nil
}

func (grid GRIB2WindGrid) longitudeForPosition(
	longitude navigation.Longitude,
) float64 {
	return unwrapLongitude(float64(longitude), grid.Longitudes[0])
}

func (grid GRIB2WindGrid) unwrappedLongitudeAt(column int) float64 {
	return unwrapLongitude(
		grid.Longitudes[column],
		grid.Longitudes[0],
	)
}

func unwrapLongitude(longitude, axisStart float64) float64 {
	for longitude < axisStart-interpolationTolerance {
		longitude += 360
	}
	for longitude >= axisStart+360-interpolationTolerance {
		longitude -= 360
	}
	return longitude
}

func (grid GRIB2WindGrid) longitudeBounds(
	longitude float64,
) (int, int, float64, error) {
	first := grid.unwrappedLongitudeAt(0)
	last := grid.unwrappedLongitudeAt(grid.Width - 1)

	if nearlyEqual(longitude, first) {
		return 0, 0, 0, nil
	}
	if longitude >= first-interpolationTolerance &&
		longitude <= last+interpolationTolerance {
		east := sort.Search(grid.Width, func(column int) bool {
			return grid.unwrappedLongitudeAt(column) >= longitude
		})
		if east == grid.Width {
			east = grid.Width - 1
		}
		if nearlyEqual(grid.unwrappedLongitudeAt(east), longitude) || east == 0 {
			return east, east, 0, nil
		}

		west := east - 1
		westLongitude := grid.unwrappedLongitudeAt(west)
		eastLongitude := grid.unwrappedLongitudeAt(east)
		ratio := (longitude - westLongitude) /
			(eastLongitude - westLongitude)
		return west, east, ratio, nil
	}

	if !grid.longitudeIsCyclic() {
		return 0, 0, 0, fmt.Errorf(
			"longitude %.6f hors de la grille [%.6f, %.6f]",
			longitude,
			first,
			last,
		)
	}

	wrappedLongitude := longitude
	for wrappedLongitude < first {
		wrappedLongitude += 360
	}
	for wrappedLongitude >= first+360 {
		wrappedLongitude -= 360
	}

	west := grid.Width - 1
	east := 0
	eastLongitude := first + 360
	ratio := (wrappedLongitude - last) / (eastLongitude - last)
	return west, east, ratio, nil
}

func (grid GRIB2WindGrid) longitudeIsCyclic() bool {
	if grid.Width < 2 {
		return false
	}
	first := grid.unwrappedLongitudeAt(0)
	second := grid.unwrappedLongitudeAt(1)
	last := grid.unwrappedLongitudeAt(grid.Width - 1)
	step := second - first
	return step > 0 &&
		nearlyEqual((last-first)+step, 360)
}

func (grid GRIB2WindGrid) interpolateComponent(
	values []float64,
	north,
	south,
	west,
	east int,
	latitudeRatio,
	longitudeRatio float64,
) float64 {
	northWest := values[north*grid.Width+west]
	northEast := values[north*grid.Width+east]
	southWest := values[south*grid.Width+west]
	southEast := values[south*grid.Width+east]

	northValue := linearInterpolation(
		northWest,
		northEast,
		longitudeRatio,
	)
	southValue := linearInterpolation(
		southWest,
		southEast,
		longitudeRatio,
	)
	return linearInterpolation(northValue, southValue, latitudeRatio)
}

func linearInterpolation(start, end, ratio float64) float64 {
	return start + (end-start)*ratio
}

func nearlyEqual(left, right float64) bool {
	return math.Abs(left-right) <= interpolationTolerance
}

package weather

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

// GRIB2WeatherProvider fournit un vent interpolé dans l'espace et dans le temps
// à partir d'une série d'échéances GRIB2 déjà décodées.
type GRIB2WeatherProvider struct {
	grids []GRIB2WindGrid
}

var _ WeatherProvider = (*GRIB2WeatherProvider)(nil)

// NewGRIB2WeatherProvider construit un fournisseur à partir d'une ou plusieurs
// grilles. Les grilles sont copiées puis triées par échéance ; deux grilles ne
// peuvent pas partager la même échéance.
func NewGRIB2WeatherProvider(
	grids ...GRIB2WindGrid,
) (*GRIB2WeatherProvider, error) {
	if len(grids) == 0 {
		return nil, fmt.Errorf(
			"weather: au moins une grille GRIB2 est requise",
		)
	}

	ordered := append([]GRIB2WindGrid(nil), grids...)
	for index := range ordered {
		if err := ordered[index].Validate(); err != nil {
			return nil, fmt.Errorf(
				"weather: grille GRIB2 %d invalide: %w",
				index,
				err,
			)
		}
		if err := ordered[index].validateInterpolationShape(); err != nil {
			return nil, fmt.Errorf(
				"weather: grille GRIB2 %d non interpolable: %w",
				index,
				err,
			)
		}
	}

	sort.Slice(ordered, func(left, right int) bool {
		return ordered[left].ValidAt.Before(ordered[right].ValidAt)
	})
	for index := 1; index < len(ordered); index++ {
		if ordered[index].ValidAt.Equal(ordered[index-1].ValidAt) {
			return nil, fmt.Errorf(
				"weather: plusieurs grilles GRIB2 pour l'échéance %s",
				ordered[index].ValidAt.Format(time.RFC3339),
			)
		}
	}

	return &GRIB2WeatherProvider{grids: ordered}, nil
}

// LoadGRIB2WeatherProvider charge une série de fichiers contenant chacun les
// composantes U et V à 10 mètres.
func LoadGRIB2WeatherProvider(
	paths ...string,
) (*GRIB2WeatherProvider, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf(
			"weather: au moins un fichier GRIB2 est requis",
		)
	}

	grids := make([]GRIB2WindGrid, 0, len(paths))
	for _, path := range paths {
		grid, err := LoadGRIB2WindGrid(path)
		if err != nil {
			return nil, fmt.Errorf(
				"weather: chargement de %q impossible: %w",
				path,
				err,
			)
		}
		grids = append(grids, grid)
	}

	return NewGRIB2WeatherProvider(grids...)
}

// WindAt retourne le vent à la position et à l'instant demandés.
//
// Lorsque l'instant est situé entre deux échéances, U et V sont d'abord
// interpolés spatialement dans chaque grille, puis linéairement dans le temps.
// La vitesse et la direction sont calculées seulement après ces interpolations.
func (provider *GRIB2WeatherProvider) WindAt(
	ctx context.Context,
	position navigation.Position,
	instant time.Time,
) (navigation.Wind, error) {
	if provider == nil || len(provider.grids) == 0 {
		return navigation.Wind{}, fmt.Errorf(
			"weather: fournisseur GRIB2 non initialisé",
		)
	}
	if ctx == nil {
		return navigation.Wind{}, fmt.Errorf(
			"weather: contexte de calcul requis",
		)
	}
	if err := ctx.Err(); err != nil {
		return navigation.Wind{}, fmt.Errorf(
			"weather: calcul du vent annulé: %w",
			err,
		)
	}
	if err := position.Validate(); err != nil {
		return navigation.Wind{}, fmt.Errorf(
			"weather: position invalide: %w",
			err,
		)
	}
	if instant.IsZero() {
		return navigation.Wind{}, fmt.Errorf(
			"weather: l'instant demandé ne doit pas être nul",
		)
	}

	upper := sort.Search(len(provider.grids), func(index int) bool {
		return !provider.grids[index].ValidAt.Before(instant)
	})

	var u, v navigation.WindComponent
	if upper < len(provider.grids) &&
		provider.grids[upper].ValidAt.Equal(instant) {
		var err error
		u, v, err = provider.grids[upper].ComponentsAt(position)
		if err != nil {
			return navigation.Wind{}, fmt.Errorf(
				"weather: interpolation à l'échéance %s impossible: %w",
				instant.Format(time.RFC3339),
				err,
			)
		}
	} else {
		if upper == 0 || upper == len(provider.grids) {
			first := provider.grids[0].ValidAt
			last := provider.grids[len(provider.grids)-1].ValidAt
			return navigation.Wind{}, fmt.Errorf(
				"weather: instant %s hors de la période couverte [%s, %s]",
				instant.Format(time.RFC3339),
				first.Format(time.RFC3339),
				last.Format(time.RFC3339),
			)
		}

		lower := upper - 1
		before := provider.grids[lower]
		after := provider.grids[upper]

		beforeU, beforeV, err := before.ComponentsAt(position)
		if err != nil {
			return navigation.Wind{}, fmt.Errorf(
				"weather: interpolation à l'échéance %s impossible: %w",
				before.ValidAt.Format(time.RFC3339),
				err,
			)
		}
		if err := ctx.Err(); err != nil {
			return navigation.Wind{}, fmt.Errorf(
				"weather: calcul du vent annulé: %w",
				err,
			)
		}

		afterU, afterV, err := after.ComponentsAt(position)
		if err != nil {
			return navigation.Wind{}, fmt.Errorf(
				"weather: interpolation à l'échéance %s impossible: %w",
				after.ValidAt.Format(time.RFC3339),
				err,
			)
		}

		duration := after.ValidAt.Sub(before.ValidAt)
		elapsed := instant.Sub(before.ValidAt)
		ratio := float64(elapsed) / float64(duration)
		u = navigation.WindComponent(linearInterpolation(
			float64(beforeU),
			float64(afterU),
			ratio,
		))
		v = navigation.WindComponent(linearInterpolation(
			float64(beforeV),
			float64(afterV),
			ratio,
		))
	}

	if err := ctx.Err(); err != nil {
		return navigation.Wind{}, fmt.Errorf(
			"weather: calcul du vent annulé: %w",
			err,
		)
	}

	wind, err := navigation.WindFromComponents(u, v)
	if err != nil {
		return navigation.Wind{}, fmt.Errorf(
			"weather: conversion du vent interpolé impossible: %w",
			err,
		)
	}
	return wind, nil
}

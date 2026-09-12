package main

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/routing"
)

type initialNavigationOptions struct {
	Heading  navigation.Heading
	SailName string
}

func parseInitialNavigationOptions(
	currentHeading float64,
	currentSail string,
) (*initialNavigationOptions, error) {
	sailName := strings.TrimSpace(currentSail)
	hasHeading := !math.IsNaN(currentHeading)
	hasSail := sailName != ""

	if !hasHeading && !hasSail {
		return nil, nil
	}
	if hasHeading != hasSail {
		return nil, errors.New(
			"routecheck: -current-heading et -current-sail doivent être fournis ensemble",
		)
	}

	heading := navigation.Heading(currentHeading)
	if err := heading.Validate(); err != nil {
		return nil, fmt.Errorf(
			"routecheck: -current-heading invalide: %w",
			err,
		)
	}
	return &initialNavigationOptions{
		Heading:  heading,
		SailName: sailName,
	}, nil
}

func resolveInitialNavigationState(
	boatPolar *polar.Polar,
	requested *initialNavigationOptions,
) (*routing.InitialNavigationState, error) {
	if requested == nil {
		return nil, nil
	}
	if boatPolar == nil {
		return nil, errors.New(
			"routecheck: résolution de l'état initial impossible: polaire requise",
		)
	}

	for _, sail := range boatPolar.Sails {
		if strings.EqualFold(sail.Name, requested.SailName) {
			return &routing.InitialNavigationState{
				Heading: requested.Heading,
				SailID:  sail.ID,
			}, nil
		}
	}

	names := make([]string, 0, len(boatPolar.Sails))
	for _, sail := range boatPolar.Sails {
		names = append(names, sail.Name)
	}
	sort.Strings(names)
	return nil, fmt.Errorf(
		"routecheck: voile actuelle %q absente de la polaire; voiles disponibles: %s",
		requested.SailName,
		strings.Join(names, ", "),
	)
}

package main

import (
	"fmt"
	"strings"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/land"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/routing"
)

func loadLandConstraints(path string) ([]routing.Constraint, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}

	constraint, err := land.LoadGeoJSONFile(path)
	if err != nil {
		return nil, fmt.Errorf(
			"routecheck: chargement des terres impossible: %w",
			err,
		)
	}
	return []routing.Constraint{constraint}, nil
}

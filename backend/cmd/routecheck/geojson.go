package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/routing"
)

func writeRouteGeoJSON(
	path string,
	route routing.IsochroneRoute,
) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}

	data, err := route.MarshalGeoJSON()
	if err != nil {
		return fmt.Errorf(
			"routecheck: préparation du GeoJSON impossible: %w",
			err,
		)
	}

	parent := filepath.Dir(path)
	if parent != "." {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return fmt.Errorf(
				"routecheck: création du dossier GeoJSON %q impossible: %w",
				parent,
				err,
			)
		}
	}

	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf(
			"routecheck: écriture du GeoJSON %q impossible: %w",
			path,
			err,
		)
	}
	return nil
}

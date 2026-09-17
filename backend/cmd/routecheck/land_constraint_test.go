package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/land"
)

func TestLoadLandConstraintsDisabled(t *testing.T) {
	constraints, err := loadLandConstraints("  ", 0)
	if err != nil {
		t.Fatalf("chargement refusé: %v", err)
	}
	if constraints != nil {
		t.Fatalf("contraintes = %v, attendues nil", constraints)
	}
}

func TestLoadLandConstraints(t *testing.T) {
	path := filepath.Join(t.TempDir(), "land.geojson")
	err := os.WriteFile(
		path,
		[]byte(`{
			"type":"FeatureCollection",
			"features":[{
				"type":"Feature",
				"geometry":{
					"type":"Polygon",
					"coordinates":[[[0,0],[1,0],[1,1],[0,1],[0,0]]]
				}
			}]
		}`),
		0o600,
	)
	if err != nil {
		t.Fatalf("fixture impossible: %v", err)
	}

	constraints, err := loadLandConstraints(path, 1)
	if err != nil {
		t.Fatalf("chargement refusé: %v", err)
	}
	if len(constraints) != 1 {
		t.Fatalf("contraintes = %d, attendue 1", len(constraints))
	}
	constraint, ok := constraints[0].(*land.GeoJSONConstraint)
	if !ok {
		t.Fatalf("type de contrainte = %T", constraints[0])
	}
	if constraint.CoastalMargin() != 1 {
		t.Fatalf(
			"marge = %.3f NM, attendue 1",
			constraint.CoastalMargin(),
		)
	}
}

func TestLoadLandConstraintsReportsFileError(t *testing.T) {
	_, err := loadLandConstraints(filepath.Join(t.TempDir(), "missing.geojson"), 0)
	if err == nil || !strings.Contains(err.Error(), "chargement des terres") {
		t.Fatalf("erreur = %v, chargement des terres attendu", err)
	}
}

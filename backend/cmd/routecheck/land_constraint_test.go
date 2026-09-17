package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadLandConstraintsDisabled(t *testing.T) {
	constraints, err := loadLandConstraints("  ")
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

	constraints, err := loadLandConstraints(path)
	if err != nil {
		t.Fatalf("chargement refusé: %v", err)
	}
	if len(constraints) != 1 {
		t.Fatalf("contraintes = %d, attendue 1", len(constraints))
	}
}

func TestLoadLandConstraintsReportsFileError(t *testing.T) {
	_, err := loadLandConstraints(filepath.Join(t.TempDir(), "missing.geojson"))
	if err == nil || !strings.Contains(err.Error(), "chargement des terres") {
		t.Fatalf("erreur = %v, chargement des terres attendu", err)
	}
}

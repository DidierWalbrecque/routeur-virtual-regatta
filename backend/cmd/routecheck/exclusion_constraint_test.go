package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/routing"
)

func TestLoadExclusionConstraintsDisabled(t *testing.T) {
	constraints, err := loadExclusionConstraints(" ")
	if err != nil {
		t.Fatalf("chargement refusé: %v", err)
	}
	if constraints != nil {
		t.Fatalf("contraintes = %v, attendues nil", constraints)
	}
}

func TestLoadExclusionConstraints(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exclusions.geojson")
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

	constraints, err := loadExclusionConstraints(path)
	if err != nil {
		t.Fatalf("chargement refusé: %v", err)
	}
	if len(constraints) != 1 {
		t.Fatalf("contraintes = %d, attendue 1", len(constraints))
	}

	allowed, err := constraints[0].Allows(
		context.Background(),
		routing.RouteSegment{
			From:       navigation.Position{Latitude: 0.5, Longitude: -1},
			To:         navigation.Position{Latitude: 0.5, Longitude: 2},
			StartedAt:  time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
			FinishedAt: time.Date(2026, 9, 5, 7, 40, 0, 0, time.UTC),
		},
	)
	if err != nil {
		t.Fatalf("vérification refusée: %v", err)
	}
	if allowed {
		t.Fatal("un segment traversant la zone doit être rejeté")
	}
}

func TestLoadExclusionConstraintsReportsFileError(t *testing.T) {
	_, err := loadExclusionConstraints(
		filepath.Join(t.TempDir(), "missing.geojson"),
	)
	if err == nil ||
		!strings.Contains(err.Error(), "zones d'exclusion") {
		t.Fatalf(
			"erreur = %v, chargement des zones d'exclusion attendu",
			err,
		)
	}
}

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/routing"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
)

func TestWriteRouteGeoJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routes", "route.geojson")
	route := testGeoJSONRoute()

	if err := writeRouteGeoJSON(path, route); err != nil {
		t.Fatalf("écriture refusée: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("lecture impossible: %v", err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatal("le fichier doit se terminer par un retour à la ligne")
	}

	var document struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("GeoJSON invalide: %v", err)
	}
	if document.Type != "FeatureCollection" {
		t.Errorf("type = %q, attendu FeatureCollection", document.Type)
	}
}

func TestWriteRouteGeoJSONSkipsEmptyPath(t *testing.T) {
	if err := writeRouteGeoJSON("", routing.IsochroneRoute{}); err != nil {
		t.Fatalf("chemin vide refusé: %v", err)
	}
}

func TestWriteRouteGeoJSONRejectsInvalidRoute(t *testing.T) {
	path := filepath.Join(t.TempDir(), "route.geojson")
	err := writeRouteGeoJSON(path, routing.IsochroneRoute{})
	if err == nil {
		t.Fatal("erreur attendue")
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("fichier créé malgré l'erreur: %v", statErr)
	}
}

func testGeoJSONRoute() routing.IsochroneRoute {
	instant := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)
	origin := navigation.Position{Latitude: 13, Longitude: 121}
	next := navigation.Position{Latitude: 13.1, Longitude: 120.9}
	destination := navigation.Position{Latitude: 14, Longitude: 120}

	return routing.IsochroneRoute{
		Origin:      origin,
		Destination: destination,
		StartedAt:   instant,
		FinishedAt:  instant.Add(10 * time.Minute),
		Steps: []routing.IsochroneRouteStep{
			{
				FrontierIndex: 0,
				NodeIndex:     0,
				Node: routing.IsochroneNode{
					ParentIndex: -1,
					Candidate: routing.HeadingCandidate{
						Heading: 270,
						Step: simulation.StepResult{
							Position:     origin,
							Instant:      instant,
							Heading:      270,
							Duration:     10 * time.Minute,
							Wind:         navigation.Wind{Speed: 2, Direction: 330},
							WindKnots:    3.88769,
							WindAngle:    60,
							Tack:         navigation.TackStarboard,
							BestSail:     polar.SailPerformance{SailName: "Code0", Speed: 8},
							NextPosition: next,
							NextInstant:  instant.Add(10 * time.Minute),
						},
					},
					RemainingDistance: 50,
					Progress:          1,
				},
			},
		},
	}
}

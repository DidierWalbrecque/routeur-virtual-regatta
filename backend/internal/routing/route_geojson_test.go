package routing

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

func TestIsochroneRouteMarshalGeoJSON(t *testing.T) {
	instant := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)
	search := buildRouteReconstructionSearch(t, instant, 2)
	route, err := search.ReconstructBestRoute()
	if err != nil {
		t.Fatalf("reconstruction refusée: %v", err)
	}

	data, err := route.MarshalGeoJSON()
	if err != nil {
		t.Fatalf("export refusé: %v", err)
	}

	var collection struct {
		Type     string `json:"type"`
		Features []struct {
			Type       string         `json:"type"`
			Properties map[string]any `json:"properties"`
			Geometry   struct {
				Type        string          `json:"type"`
				Coordinates json.RawMessage `json:"coordinates"`
			} `json:"geometry"`
		} `json:"features"`
	}
	if err := json.Unmarshal(data, &collection); err != nil {
		t.Fatalf("GeoJSON invalide: %v\n%s", err, data)
	}
	if collection.Type != "FeatureCollection" {
		t.Errorf("type = %q, attendu FeatureCollection", collection.Type)
	}
	if len(collection.Features) != len(route.Steps)+3 {
		t.Fatalf(
			"features = %d, attendues %d",
			len(collection.Features),
			len(route.Steps)+3,
		)
	}

	line := collection.Features[0]
	if line.Type != "Feature" || line.Geometry.Type != "LineString" {
		t.Fatalf(
			"feature route = type %q, géométrie %q",
			line.Type,
			line.Geometry.Type,
		)
	}
	if line.Properties["kind"] != "route" {
		t.Errorf("kind de la ligne = %v, attendu route", line.Properties["kind"])
	}
	if line.Properties["startedAt"] != instant.Format(time.RFC3339) {
		t.Errorf("startedAt = %v", line.Properties["startedAt"])
	}
	if line.Properties["finishedAt"] != route.FinishedAt.Format(time.RFC3339) {
		t.Errorf("finishedAt = %v", line.Properties["finishedAt"])
	}

	var lineCoordinates [][]float64
	if err := json.Unmarshal(line.Geometry.Coordinates, &lineCoordinates); err != nil {
		t.Fatalf("coordonnées de ligne invalides: %v", err)
	}
	if len(lineCoordinates) != len(route.Steps)+1 {
		t.Fatalf(
			"coordonnées = %d, attendues %d",
			len(lineCoordinates),
			len(route.Steps)+1,
		)
	}
	assertGeoJSONPosition(t, lineCoordinates[0], route.Origin)
	for index, step := range route.Steps {
		assertGeoJSONPosition(
			t,
			lineCoordinates[index+1],
			step.Node.Candidate.Step.NextPosition,
		)
	}

	if collection.Features[1].Properties["kind"] != "start" {
		t.Errorf(
			"deuxième feature = %v, attendue start",
			collection.Features[1].Properties["kind"],
		)
	}
	if collection.Features[2].Properties["kind"] != "destination" {
		t.Errorf(
			"troisième feature = %v, attendue destination",
			collection.Features[2].Properties["kind"],
		)
	}

	firstStepFeature := collection.Features[3]
	firstStep := route.Steps[0]
	step := firstStep.Node.Candidate.Step
	if firstStepFeature.Properties["kind"] != "step" {
		t.Errorf(
			"kind du premier pas = %v, attendu step",
			firstStepFeature.Properties["kind"],
		)
	}
	if firstStepFeature.Properties["index"] != float64(1) {
		t.Errorf("index du premier pas = %v", firstStepFeature.Properties["index"])
	}
	if firstStepFeature.Properties["heading"] != float64(step.Heading) {
		t.Errorf("cap = %v, attendu %.3f", firstStepFeature.Properties["heading"], step.Heading)
	}
	if firstStepFeature.Properties["sail"] != step.BestSail.SailName {
		t.Errorf("voile = %v, attendue %s", firstStepFeature.Properties["sail"], step.BestSail.SailName)
	}
	if firstStepFeature.Properties["tack"] != string(step.Tack) {
		t.Errorf("amure = %v, attendue %s", firstStepFeature.Properties["tack"], step.Tack)
	}
	if firstStepFeature.Geometry.Type != "Point" {
		t.Errorf("géométrie du pas = %q, attendue Point", firstStepFeature.Geometry.Type)
	}
	var pointCoordinates []float64
	if err := json.Unmarshal(
		firstStepFeature.Geometry.Coordinates,
		&pointCoordinates,
	); err != nil {
		t.Fatalf("coordonnées du pas invalides: %v", err)
	}
	assertGeoJSONPosition(t, pointCoordinates, step.NextPosition)
}

func TestIsochroneRouteMarshalGeoJSONRejectsInvalidRoute(t *testing.T) {
	instant := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)
	search := buildRouteReconstructionSearch(t, instant, 2)
	valid, err := search.ReconstructBestRoute()
	if err != nil {
		t.Fatalf("reconstruction refusée: %v", err)
	}

	tests := []struct {
		name    string
		mutate  func(*IsochroneRoute)
		wantErr string
	}{
		{
			name: "origine invalide",
			mutate: func(route *IsochroneRoute) {
				route.Origin.Latitude = 91
			},
			wantErr: "origine invalide",
		},
		{
			name: "instant initial absent",
			mutate: func(route *IsochroneRoute) {
				route.StartedAt = time.Time{}
			},
			wantErr: "instant initial",
		},
		{
			name: "route vide",
			mutate: func(route *IsochroneRoute) {
				route.Steps = nil
			},
			wantErr: "route vide",
		},
		{
			name: "liaison rompue",
			mutate: func(route *IsochroneRoute) {
				route.Steps[1].Node.Candidate.Step.Position =
					navigation.Position{Latitude: 1}
			},
			wantErr: "position initiale du pas 2",
		},
		{
			name: "instant final incohérent",
			mutate: func(route *IsochroneRoute) {
				route.FinishedAt = route.FinishedAt.Add(time.Minute)
			},
			wantErr: "instant final de la route",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			route := valid
			route.Steps = append(
				[]IsochroneRouteStep(nil),
				valid.Steps...,
			)
			test.mutate(&route)

			_, err := route.MarshalGeoJSON()
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %v, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

func assertGeoJSONPosition(
	t *testing.T,
	coordinates []float64,
	position navigation.Position,
) {
	t.Helper()
	if len(coordinates) != 2 {
		t.Fatalf("coordonnées = %v, deux valeurs attendues", coordinates)
	}
	if coordinates[0] != float64(position.Longitude) ||
		coordinates[1] != float64(position.Latitude) {
		t.Errorf(
			"coordonnées = %v, attendues [%.6f %.6f]",
			coordinates,
			position.Longitude,
			position.Latitude,
		)
	}
}

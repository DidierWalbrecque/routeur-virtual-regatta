package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/land"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/routing"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/weather"
)

func square(name string, south, west, north, east float64) exclusionZoneRequest {
	return exclusionZoneRequest{Name: name, Points: []positionRequest{
		{Latitude: south, Longitude: west},
		{Latitude: south, Longitude: east},
		{Latitude: north, Longitude: east},
		{Latitude: north, Longitude: west},
	}}
}

func TestExclusionConstraintBlocksSegmentsCrossingAZone(t *testing.T) {
	constraint, err := exclusionConstraintFromZones([]exclusionZoneRequest{square("île", 0, 0, 1, 1)})
	if err != nil || constraint == nil {
		t.Fatalf("contrainte attendue, obtenu %v, %v", constraint, err)
	}
	segment := func(fromLat, fromLon, toLat, toLon float64) routing.RouteSegment {
		return routing.RouteSegment{
			From:       navigation.Position{Latitude: navigation.Latitude(fromLat), Longitude: navigation.Longitude(fromLon)},
			To:         navigation.Position{Latitude: navigation.Latitude(toLat), Longitude: navigation.Longitude(toLon)},
			StartedAt:  time.Date(2026, 9, 5, 7, 0, 0, 0, time.UTC),
			FinishedAt: time.Date(2026, 9, 5, 7, 10, 0, 0, time.UTC),
		}
	}
	cases := []struct {
		name    string
		segment routing.RouteSegment
		allowed bool
	}{
		{"traverse la zone", segment(0.5, -1, 0.5, 2), false},
		{"entre dans la zone", segment(0.5, -1, 0.5, 0.5), false},
		{"passe au nord", segment(2, -1, 2, 2), true},
		{"reste loin", segment(-3, -3, -2, -2), true},
	}
	for _, testCase := range cases {
		allowed, allowErr := constraint.Allows(context.Background(), testCase.segment)
		if allowErr != nil {
			t.Fatalf("%s : %v", testCase.name, allowErr)
		}
		if allowed != testCase.allowed {
			t.Errorf("%s : autorisé = %v, attendu %v", testCase.name, allowed, testCase.allowed)
		}
	}
}

func TestExclusionConstraintRejectsInvalidZones(t *testing.T) {
	tooMany := make([]exclusionZoneRequest, maxExclusionZones+1)
	for index := range tooMany {
		tooMany[index] = square("z", 0, 0, 1, 1)
	}
	cases := map[string][]exclusionZoneRequest{
		"deux points":       {{Name: "a", Points: []positionRequest{{Latitude: 0, Longitude: 0}, {Latitude: 1, Longitude: 1}}}},
		"points confondus":  {{Name: "b", Points: []positionRequest{{Latitude: 1, Longitude: 1}, {Latitude: 1, Longitude: 1}, {Latitude: 1, Longitude: 1}}}},
		"latitude invalide": {{Name: "c", Points: []positionRequest{{Latitude: 95, Longitude: 0}, {Latitude: 1, Longitude: 1}, {Latitude: 2, Longitude: 0}}}},
		"trop de zones":     tooMany,
	}
	for name, zones := range cases {
		if _, err := exclusionConstraintFromZones(zones); err == nil {
			t.Errorf("%s : une erreur était attendue", name)
		}
	}
	if constraint, err := exclusionConstraintFromZones(nil); constraint != nil || err != nil {
		t.Fatalf("aucune zone : nil attendu, obtenu %v, %v", constraint, err)
	}
}

func TestPracticalHeadingsAppliesExclusionZones(t *testing.T) {
	originalWeatherLoader := loadWeatherProviderForRequest
	originalPolarLoader := loadPolarForRequest
	originalLandLoader := loadLandForRequest
	originalEvaluate := evaluatePracticalHeadings
	originalBuild := buildIsochroneSearch
	t.Cleanup(func() {
		loadWeatherProviderForRequest = originalWeatherLoader
		loadPolarForRequest = originalPolarLoader
		loadLandForRequest = originalLandLoader
		evaluatePracticalHeadings = originalEvaluate
		buildIsochroneSearch = originalBuild
	})
	loadWeatherProviderForRequest = func(...string) (weather.WeatherProvider, cacheLoadStats, error) {
		return nil, cacheLoadStats{}, nil
	}
	loadPolarForRequest = func(string) (*polar.Polar, bool, error) {
		return &polar.Polar{MaxSpeed: 10}, false, nil
	}
	loadLandForRequest = func(string) (*land.GeoJSONConstraint, bool, error) {
		constraint, err := land.DecodeGeoJSON(strings.NewReader(testLandGeoJSON))
		return constraint, false, err
	}
	evaluatePracticalHeadings = func(context.Context, weather.WeatherProvider, *polar.Polar, routing.PracticalHeadingInput) ([]routing.PracticalHeadingCandidate, error) {
		return nil, nil
	}
	var captured routing.IsochroneSearchInput
	buildIsochroneSearch = func(_ context.Context, _ weather.WeatherProvider, _ *polar.Polar, input routing.IsochroneSearchInput) (routing.IsochroneSearch, error) {
		captured = input
		instant := input.Instant.Add(input.StepDuration)
		return routing.IsochroneSearch{
			Origin: input.Position, Destination: input.Destination, StartedAt: input.Instant,
			StepDuration: input.StepDuration, Frontiers: []routing.IsochroneFrontier{
				streamTestFrontier(input.Position, instant, 1, -1),
			}, ArrivalRadius: input.ArrivalRadius,
		}, nil
	}

	post := func(zones string) *httptest.ResponseRecorder {
		body := `{"weatherFiles":["fake"],"polarPath":"fake","position":{"latitude":-10,"longitude":-10},"destination":{"latitude":-10,"longitude":-9.9},"instant":"2026-09-05T07:30:00Z","durationMinutes":10,"stepMinutes":10,"headings":[0],"mode":"quick","exclusionZones":` + zones + `}`
		request := httptest.NewRequest(http.MethodPost, "/api/practical-headings", bytes.NewBufferString(body))
		response := httptest.NewRecorder()
		NewHandler().ServeHTTP(response, request)
		return response
	}

	const crossing = `[{"name":"à éviter","points":[{"latitude":-10.05,"longitude":-9.97},{"latitude":-10.05,"longitude":-9.93},{"latitude":-9.95,"longitude":-9.93},{"latitude":-9.95,"longitude":-9.97}]}]`
	if response := post(crossing); response.Code != http.StatusOK {
		t.Fatalf("calcul avec zone : statut %d, corps %s", response.Code, response.Body.String())
	}
	if len(captured.Constraints) != 2 || len(captured.ArrivalConstraints) != 2 {
		t.Fatalf("la zone doit s'ajouter aux contraintes de trajet et d'arrivée : %d et %d", len(captured.Constraints), len(captured.ArrivalConstraints))
	}
	blocked := routing.RouteSegment{
		From:       navigation.Position{Latitude: -10, Longitude: -10},
		To:         navigation.Position{Latitude: -10, Longitude: -9.9},
		StartedAt:  time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
		FinishedAt: time.Date(2026, 9, 5, 7, 40, 0, 0, time.UTC),
	}
	allowed, err := captured.Constraints[1].Allows(context.Background(), blocked)
	if err != nil || allowed {
		t.Fatalf("le segment qui traverse la zone doit être refusé (autorisé = %v, erreur = %v)", allowed, err)
	}

	captured = routing.IsochroneSearchInput{}
	if response := post(`[]`); response.Code != http.StatusOK || len(captured.Constraints) != 1 {
		t.Fatalf("sans zone, une seule contrainte attendue : statut %d, %d contrainte(s)", response.Code, len(captured.Constraints))
	}

	insideStart := `[{"name":"départ","points":[{"latitude":-10.1,"longitude":-10.1},{"latitude":-10.1,"longitude":-9.95},{"latitude":-9.95,"longitude":-9.95},{"latitude":-9.95,"longitude":-10.1}]}]`
	response := post(insideStart)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "zone interdite") {
		t.Fatalf("départ dans une zone : 400 attendu, obtenu %d : %s", response.Code, response.Body.String())
	}
	if response := post(`[{"name":"x","points":[{"latitude":0,"longitude":0}]}]`); response.Code != http.StatusBadRequest {
		t.Fatalf("zone invalide : 400 attendu, obtenu %d", response.Code)
	}
}

func TestPracticalHeadingsLimitsTheHorizonByMode(t *testing.T) {
	post := func(mode string, minutes string) string {
		body := `{"mode":"` + mode + `","instant":"2026-09-05T07:30:00Z","durationMinutes":` + minutes + `}`
		request := httptest.NewRequest(http.MethodPost, "/api/practical-headings", bytes.NewBufferString(body))
		response := httptest.NewRecorder()
		NewHandler().ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s %s min : 400 attendu, obtenu %d", mode, minutes, response.Code)
		}
		return response.Body.String()
	}
	if message := post("precise", "5761"); !strings.Contains(message, "trop longue") || !strings.Contains(message, "4 jours") {
		t.Fatalf("le mode précis doit refuser plus de 4 jours : %s", message)
	}
	if message := post("quick", "14401"); !strings.Contains(message, "trop longue") || !strings.Contains(message, "10 jours") {
		t.Fatalf("le mode rapide doit refuser plus de 10 jours : %s", message)
	}
	// À la limite, la durée est acceptée : l'erreur suivante est celle des fichiers manquants.
	if message := post("precise", "5760"); strings.Contains(message, "trop longue") {
		t.Fatalf("4 jours exactement doivent être acceptés en mode précis : %s", message)
	}
	if message := post("quick", "14400"); strings.Contains(message, "trop longue") {
		t.Fatalf("10 jours exactement doivent être acceptés en mode rapide : %s", message)
	}
}

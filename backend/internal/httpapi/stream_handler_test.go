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
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/weather"
)

func TestPracticalHeadingsStreamEndpointEmitsProgressBeforeComplete(t *testing.T) {
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
	evaluatePracticalHeadings = func(
		context.Context,
		weather.WeatherProvider,
		*polar.Polar,
		routing.PracticalHeadingInput,
	) ([]routing.PracticalHeadingCandidate, error) {
		return nil, nil
	}
	buildIsochroneSearch = func(
		ctx context.Context,
		_ weather.WeatherProvider,
		_ *polar.Polar,
		input routing.IsochroneSearchInput,
	) (routing.IsochroneSearch, error) {
		firstInstant := input.Instant.Add(input.StepDuration)
		secondInstant := firstInstant.Add(input.StepDuration)
		frontiers := []routing.IsochroneFrontier{
			streamTestFrontier(input.Position, firstInstant, 1, -1),
			streamTestFrontier(input.Position, secondInstant, 2, 0),
		}
		for index, frontier := range frontiers {
			if input.Progress != nil {
				input.Progress(routing.IsochroneProgress{
					CompletedLayers: index + 1,
					TotalLayers:     len(frontiers),
					Instant:         frontier.Instant,
					CandidateCount:  len(frontier.Nodes),
					CellSize:        1,
					Frontier:        frontier,
				})
			}
		}
		return routing.IsochroneSearch{
			Origin:        input.Position,
			Destination:   input.Destination,
			StartedAt:     input.Instant,
			StepDuration:  input.StepDuration,
			Frontiers:     frontiers,
			ArrivalRadius: input.ArrivalRadius,
		}, nil
	}

	payload := []byte(`{"weatherFiles":["fake"],"polarPath":"fake","position":{"latitude":-10,"longitude":-10},"destination":{"latitude":-10,"longitude":-9.9},"instant":"2026-09-05T07:30:00Z","durationMinutes":20,"stepMinutes":10,"headings":[0],"mode":"quick"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/practical-headings/stream", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	NewHandler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("statut = %d, corps = %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	startIndex := strings.Index(body, "event: start")
	frontierIndex := strings.Index(body, "event: frontier")
	completeIndex := strings.Index(body, "event: complete")
	if startIndex < 0 || frontierIndex < 0 || completeIndex < 0 {
		t.Fatalf("événements SSE absents: %q", body)
	}
	if !(startIndex < frontierIndex && frontierIndex < completeIndex) {
		t.Fatalf("ordre SSE inattendu: %q", body)
	}
	if !strings.Contains(body, `"completedLayers":1`) || !strings.Contains(body, `"completedLayers":2`) {
		t.Fatalf("progression absente: %q", body)
	}
}

func streamTestFrontier(origin navigation.Position, instant time.Time, index, parentIndex int) routing.IsochroneFrontier {
	positions := []navigation.Position{
		{Latitude: origin.Latitude + navigation.Latitude(0.01*float64(index)), Longitude: origin.Longitude - 0.01},
		{Latitude: origin.Latitude + navigation.Latitude(0.02*float64(index)), Longitude: origin.Longitude},
		{Latitude: origin.Latitude + navigation.Latitude(0.01*float64(index)), Longitude: origin.Longitude + 0.01},
	}
	nodes := make([]routing.IsochroneNode, 0, len(positions))
	for _, position := range positions {
		nodes = append(nodes, routing.IsochroneNode{
			ParentIndex: parentIndex,
			Candidate: routing.HeadingCandidate{
				Heading: 0,
				Step: simulation.StepResult{
					NextPosition: position,
					NextInstant:  instant,
					BestSail:     polar.SailPerformance{SailName: "test"},
				},
			},
		})
	}
	return routing.IsochroneFrontier{Index: index, Instant: instant, CellSize: 1, Nodes: nodes}
}

package routing

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

func TestEvaluatePracticalHeadingCandidatesEvaluatesWeatherVariations(t *testing.T) {
	provider := &staticWeatherProvider{
		wind: navigation.Wind{Speed: 5, Direction: 180},
	}
	variations := []PracticalWeatherVariation{
		{SpeedOffset: 2, DirectionOffset: 30},
		{SpeedOffset: -10, DirectionOffset: 190},
	}

	candidates, err := EvaluatePracticalHeadingCandidates(
		context.Background(),
		provider,
		testPolar(),
		PracticalHeadingInput{
			Position:          navigation.Position{},
			Destination:       navigation.Position{Longitude: 10},
			Instant:           time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
			Duration:          time.Hour,
			StepDuration:      time.Hour,
			Headings:          []navigation.Heading{90},
			WeatherVariations: variations,
		},
	)
	if err != nil {
		t.Fatalf("évaluation refusée: %v", err)
	}
	if provider.calls != 1 {
		t.Fatalf("appels météo = %d, attendus 1 après déduplication", provider.calls)
	}
	if len(candidates[0].WeatherScenarios) != len(variations) {
		t.Fatalf(
			"scénarios météo = %d, attendus %d",
			len(candidates[0].WeatherScenarios),
			len(variations),
		)
	}
	firstWind := candidates[0].WeatherScenarios[0].Simulation.Steps[0].Wind
	if firstWind.Speed != 7 || firstWind.Direction != 210 {
		t.Errorf("premier vent varié = %+v, attendu 7 m/s depuis 210°", firstWind)
	}
	secondWind := candidates[0].WeatherScenarios[1].Simulation.Steps[0].Wind
	if secondWind.Speed != 0 || secondWind.Direction != 10 {
		t.Errorf("second vent varié = %+v, attendu 0 m/s depuis 10°", secondWind)
	}
}

func TestEvaluatePracticalHeadingCandidatesRejectsInvalidWeatherVariation(t *testing.T) {
	_, err := EvaluatePracticalHeadingCandidates(
		context.Background(),
		&staticWeatherProvider{wind: navigation.Wind{Speed: 5, Direction: 180}},
		testPolar(),
		PracticalHeadingInput{
			Position:          navigation.Position{},
			Destination:       navigation.Position{Longitude: 10},
			Instant:           time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
			Duration:          time.Hour,
			StepDuration:      time.Hour,
			Headings:          []navigation.Heading{90},
			WeatherVariations: []PracticalWeatherVariation{{SpeedOffset: math.NaN()}},
		},
	)
	if err == nil || !strings.Contains(err.Error(), "variation météo pratique 1") {
		t.Fatalf("erreur = %v, doit identifier la variation invalide", err)
	}
}

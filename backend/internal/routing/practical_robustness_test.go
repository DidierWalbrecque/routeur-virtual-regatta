package routing

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
)

func TestPracticalRobustness(t *testing.T) {
	worst, loss, robust := practicalRobustness(
		10,
		[]PracticalHeadingWeatherScenario{{Progress: 8}, {Progress: 12}},
		2,
	)
	if worst != 8 || loss != 2 || !robust {
		t.Fatalf("robustesse = pire %.1f, perte %.1f, robuste %t", worst, loss, robust)
	}

	_, loss, robust = practicalRobustness(10, nil, 0)
	if loss != 0 || !robust {
		t.Fatalf("sans scénario : perte %.1f, robuste %t", loss, robust)
	}
}

func TestEvaluatePracticalHeadingCandidatesCanRequireRobustScenarios(t *testing.T) {
	base := PracticalHeadingInput{
		Position:     navigation.Position{},
		Destination:  navigation.Position{Longitude: 10},
		Instant:      time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
		Duration:     time.Hour,
		StepDuration: time.Hour,
		Headings:     []navigation.Heading{90},
		WeatherVariations: []PracticalWeatherVariation{
			{SpeedOffset: -5},
		},
		MaxScenarioProgressLoss: 1,
	}
	provider := func() *staticWeatherProvider {
		return &staticWeatherProvider{
			wind: navigation.Wind{Speed: 5, Direction: 180},
		}
	}

	candidates, err := EvaluatePracticalHeadingCandidates(
		context.Background(),
		provider(),
		windSensitivePracticalPolar(),
		base,
	)
	if err != nil {
		t.Fatalf("évaluation refusée: %v", err)
	}
	if candidates[0].Robust || candidates[0].ScenarioProgressLoss <= 1 ||
		candidates[0].WorstCaseProgress >= candidates[0].Progress {
		t.Fatalf("robustesse inattendue: %+v", candidates[0])
	}

	strict := base
	strict.RequireRobustScenarios = true
	_, err = EvaluatePracticalHeadingCandidates(
		context.Background(), provider(), windSensitivePracticalPolar(), strict,
	)
	if err == nil || !strings.Contains(err.Error(), "aucun cap pratique robuste") {
		t.Fatalf("erreur = %v, robuste absent attendu", err)
	}

	accepted := base
	accepted.RequireRobustScenarios = true
	accepted.MaxScenarioProgressLoss = candidates[0].ScenarioProgressLoss
	filtered, err := EvaluatePracticalHeadingCandidates(
		context.Background(), provider(), windSensitivePracticalPolar(), accepted,
	)
	if err != nil || len(filtered) != 1 || !filtered[0].Robust {
		t.Fatalf("candidats robustes = %+v, erreur = %v", filtered, err)
	}
}

func TestEvaluatePracticalHeadingCandidatesRejectsInvalidScenarioLoss(t *testing.T) {
	_, err := EvaluatePracticalHeadingCandidates(
		context.Background(),
		&staticWeatherProvider{wind: navigation.Wind{Speed: 5, Direction: 180}},
		testPolar(),
		PracticalHeadingInput{
			Position:                navigation.Position{},
			Destination:             navigation.Position{Longitude: 10},
			Instant:                 time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
			Duration:                time.Hour,
			StepDuration:            time.Hour,
			Headings:                []navigation.Heading{90},
			MaxScenarioProgressLoss: -1,
		},
	)
	if err == nil || !strings.Contains(err.Error(), "perte maximale") {
		t.Fatalf("erreur = %v, perte maximale invalide attendue", err)
	}
}

func windSensitivePracticalPolar() *polar.Polar {
	return &polar.Polar{
		Label:            "Vent sensible",
		GlobalSpeedRatio: 1,
		WindSpeeds:       []polar.WindSpeed{0, 20},
		WindAngles:       []polar.TrueWindAngle{0, 180},
		Sails: []polar.Sail{{
			ID:   1,
			Name: "Jib",
			Speeds: polar.SpeedGrid{Values: [][]polar.BoatSpeed{
				{0, 20}, {0, 20},
			}},
		}, {
			ID:   2,
			Name: "Spi",
			Speeds: polar.SpeedGrid{Values: [][]polar.BoatSpeed{
				{0, 20}, {0, 20},
			}},
		}},
	}
}

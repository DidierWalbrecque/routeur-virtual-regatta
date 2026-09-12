package routing

import (
	"context"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
)

func TestBuildIsochroneSearchEvaluatesOptionalTransitions(t *testing.T) {
	instant := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)
	initialNavigation := InitialNavigationState{
		Heading: 120,
		SailID:  1,
	}
	evaluator := testRoutingTransitionEvaluator()

	search, err := BuildIsochroneSearch(
		context.Background(),
		&staticWeatherProvider{
			wind: navigation.Wind{
				Speed:     5,
				Direction: 180,
			},
		},
		testPolar(),
		IsochroneSearchInput{
			Position:            navigation.Position{},
			Destination:         navigation.Position{Longitude: 10},
			Instant:             instant,
			StepDuration:        time.Hour,
			LayerCount:          1,
			HeadingSpacing:      120,
			CellSize:            0.01,
			InitialStamina:      simulation.StaminaState{Level: 100},
			InitialNavigation:   &initialNavigation,
			TransitionEvaluator: &evaluator,
		},
	)
	if err != nil {
		t.Fatalf("construction refusée: %v", err)
	}

	var tackNode *IsochroneNode
	for index := range search.Frontiers[0].Nodes {
		node := &search.Frontiers[0].Nodes[index]
		if node.Transition != nil &&
			node.Transition.Maneuver.Kind == simulation.ManeuverTack {
			tackNode = node
			break
		}
	}
	if tackNode == nil {
		t.Fatal("aucune transition de virement évaluée")
	}
	if tackNode.Maneuver == nil ||
		tackNode.Maneuver.Kind != simulation.ManeuverTack {
		t.Errorf("manœuvre du nœud = %+v", tackNode.Maneuver)
	}
	if tackNode.Transition.TotalConsumption != 10 {
		t.Errorf(
			"consommation = %.1f, attendue 10",
			tackNode.Transition.TotalConsumption,
		)
	}
	if tackNode.Transition.NauticalPenalty == nil {
		t.Fatal("pénalité nautique absente")
	}
	if tackNode.State.Stamina.Level != 90 {
		t.Errorf(
			"stamina du nœud = %.1f, attendue 90",
			tackNode.State.Stamina.Level,
		)
	}
	if tackNode.State.Stamina != tackNode.Transition.NextStamina {
		t.Errorf(
			"stamina portée = %+v, stamina évaluée = %+v",
			tackNode.State.Stamina,
			tackNode.Transition.NextStamina,
		)
	}
}

func TestBuildIsochroneSearchKeepsLegacyBehaviorWithoutTransitionEvaluator(
	t *testing.T,
) {
	initialNavigation := InitialNavigationState{
		Heading: 120,
		SailID:  1,
	}
	search, err := BuildIsochroneSearch(
		context.Background(),
		&staticWeatherProvider{
			wind: navigation.Wind{
				Speed:     5,
				Direction: 180,
			},
		},
		testPolar(),
		IsochroneSearchInput{
			Position:          navigation.Position{},
			Destination:       navigation.Position{Longitude: 10},
			Instant:           time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
			StepDuration:      time.Hour,
			LayerCount:        1,
			HeadingSpacing:    120,
			CellSize:          0.01,
			InitialStamina:    simulation.StaminaState{Level: 80},
			InitialNavigation: &initialNavigation,
		},
	)
	if err != nil {
		t.Fatalf("construction refusée: %v", err)
	}

	maneuverCount := 0
	for _, node := range search.Frontiers[0].Nodes {
		if node.Maneuver != nil {
			maneuverCount++
		}
		if node.Transition != nil {
			t.Errorf("transition inattendue: %+v", node.Transition)
		}
		if node.State.Stamina.Level != 80 {
			t.Errorf(
				"stamina = %.1f, attendue 80",
				node.State.Stamina.Level,
			)
		}
	}
	if maneuverCount == 0 {
		t.Fatal("détection de manœuvre absente sans évaluateur")
	}
}

func testRoutingTransitionEvaluator() simulation.TransitionEvaluator {
	penalty := &polar.WindDependentPenalty{
		LowWind: polar.ManeuverPenalty{
			SpeedRatio: 0.5,
			Duration:   time.Minute,
		},
		HighWind: polar.ManeuverPenalty{
			SpeedRatio: 0.5,
			Duration:   time.Minute,
		},
	}
	profile := polar.ManeuverProfile{
		Standard:     penalty,
		Professional: penalty,
	}
	return simulation.TransitionEvaluator{
		WinchProfile: polar.WinchProfile{
			LowWindThreshold:  5,
			HighWindThreshold: 15,
			Tack:              profile,
			Gybe:              profile,
			SailChange:        profile,
		},
		ConsumptionRules: simulation.DefaultStaminaConsumptionRules(),
		WindCurve: simulation.StaminaWindCurve{
			Points: []simulation.StaminaWindPoint{
				{WindSpeed: 0, Factor: 1},
			},
		},
		DurationCurve: simulation.StaminaDurationCurve{
			Points: []simulation.StaminaDurationPoint{
				{Stamina: 0, Factor: 1},
			},
		},
		CombineConsumption: func(
			consumption simulation.StaminaConsumption,
		) (simulation.StaminaLevel, error) {
			return consumption.Nautical + consumption.SailChange, nil
		},
	}
}

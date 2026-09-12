package routing

import (
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
)

func TestEvaluateCandidateTransitionAppliesTriggeredPenalty(t *testing.T) {
	parent, candidate := penaltyTestParentAndCandidate()
	evaluator := penaltyTestTransitionEvaluator()

	maneuver, transition, application, err :=
		evaluateCandidateTransitionAndPenalty(
			parent,
			&candidate,
			nil,
			&evaluator,
			nil,
		)
	if err != nil {
		t.Fatalf("évaluation refusée: %v", err)
	}
	if maneuver == nil || maneuver.Kind != simulation.ManeuverTack {
		t.Fatalf("manœuvre = %+v, virement attendu", maneuver)
	}
	if transition == nil {
		t.Fatal("transition absente")
	}
	if application == nil {
		t.Fatal("application de pénalité absente")
	}
	if application.PenalizedDuration != 10*time.Minute {
		t.Errorf(
			"durée pénalisée = %s, attendue 10m",
			application.PenalizedDuration,
		)
	}
	if application.RemainingPenalty == nil ||
		application.RemainingPenalty.Duration != 5*time.Minute {
		t.Errorf(
			"pénalité restante = %+v, durée de 5m attendue",
			application.RemainingPenalty,
		)
	}
	if candidate.Step.Distance != 1 {
		t.Errorf(
			"distance pénalisée = %.6f NM, attendue 1",
			candidate.Step.Distance,
		)
	}
}

func TestEvaluateCandidateTransitionCarriesActivePenalty(t *testing.T) {
	parent, candidate := penaltyTestParentAndCandidate()
	parent.BoatState.Tack = candidate.Step.Tack
	parent.WindAngle = candidate.Step.WindAngle
	parent.BoatState.ActiveSpeedPenalty = &polar.ManeuverPenalty{
		SpeedRatio: 0.5,
		Duration:   15 * time.Minute,
	}

	_, transition, application, err :=
		evaluateCandidateTransitionAndPenalty(
			parent,
			&candidate,
			nil,
			nil,
			nil,
		)
	if err != nil {
		t.Fatalf("évaluation refusée: %v", err)
	}
	if transition != nil {
		t.Errorf("transition inattendue: %+v", transition)
	}
	if application == nil || application.RemainingPenalty == nil {
		t.Fatal("pénalité active non reportée")
	}
	if application.RemainingPenalty.Duration != 5*time.Minute {
		t.Errorf(
			"durée restante = %s, attendue 5m",
			application.RemainingPenalty.Duration,
		)
	}
	if candidate.Step.Distance != 1 {
		t.Errorf(
			"distance pénalisée = %.6f NM, attendue 1",
			candidate.Step.Distance,
		)
	}
}

func TestEvaluateCandidateTransitionRequiresRuleForOverlappingPenalties(
	t *testing.T,
) {
	parent, candidate := penaltyTestParentAndCandidate()
	parent.BoatState.ActiveSpeedPenalty = &polar.ManeuverPenalty{
		SpeedRatio: 0.8,
		Duration:   5 * time.Minute,
	}
	evaluator := penaltyTestTransitionEvaluator()

	_, _, _, err := evaluateCandidateTransitionAndPenalty(
		parent,
		&candidate,
		nil,
		&evaluator,
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "règle explicite") {
		t.Fatalf(
			"erreur = %v, doit signaler la règle de superposition absente",
			err,
		)
	}
}

func TestPruningSeparatesDifferentActiveSpeedPenalties(t *testing.T) {
	origin := navigation.Position{}
	position := navigation.Position{Longitude: 0.001}
	first := penaltyPruningNode(position, 5*time.Minute)
	second := penaltyPruningNode(position, 10*time.Minute)

	selected, err := pruneIsochroneNodes(
		origin,
		[]IsochroneNode{first, second},
		1,
	)
	if err != nil {
		t.Fatalf("élagage refusé: %v", err)
	}
	if len(selected) != 2 {
		t.Fatalf(
			"positions conservées = %d, attendues 2",
			len(selected),
		)
	}
}

func penaltyTestParentAndCandidate() (
	isochroneParentState,
	HeadingCandidate,
) {
	at := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)
	position := navigation.Position{Latitude: 13, Longitude: 121}
	parent := isochroneParentState{
		Position: position,
		Stamina:  simulation.StaminaState{Level: 100},
		BoatState: simulation.BoatState{
			Position: position,
			Instant:  at,
			Heading:  60,
			SailID:   1,
			Tack:     navigation.TackStarboard,
			Stamina:  simulation.StaminaState{Level: 100},
		},
		WindAngle:    60,
		HasBoatState: true,
	}
	candidate := HeadingCandidate{
		Heading: 90,
		Step: simulation.StepResult{
			Position:  position,
			Instant:   at,
			Heading:   90,
			Duration:  10 * time.Minute,
			WindKnots: 20,
			WindAngle: 50,
			Tack:      navigation.TackPort,
			BestSail: polar.SailPerformance{
				SailID:   1,
				SailName: "Jib",
				Speed:    12,
			},
			Distance:     2,
			NextPosition: navigation.Position{Latitude: 14, Longitude: 122},
			NextInstant:  at.Add(10 * time.Minute),
		},
	}
	return parent, candidate
}

func penaltyTestTransitionEvaluator() simulation.TransitionEvaluator {
	penalty := &polar.WindDependentPenalty{
		LowWind: polar.ManeuverPenalty{
			SpeedRatio: 0.5,
			Duration:   15 * time.Minute,
		},
		HighWind: polar.ManeuverPenalty{
			SpeedRatio: 0.5,
			Duration:   15 * time.Minute,
		},
	}
	return simulation.TransitionEvaluator{
		WinchProfile: polar.WinchProfile{
			LowWindThreshold:  10,
			HighWindThreshold: 30,
			Tack: polar.ManeuverProfile{
				Standard:     penalty,
				Professional: penalty,
			},
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
	}
}

func penaltyPruningNode(
	position navigation.Position,
	duration time.Duration,
) IsochroneNode {
	at := time.Date(2026, 9, 5, 7, 40, 0, 0, time.UTC)
	return IsochroneNode{
		Candidate: HeadingCandidate{
			Heading: 90,
			Step: simulation.StepResult{
				NextPosition: position,
			},
		},
		Progress: 1,
		State: simulation.BoatState{
			Position: position,
			Instant:  at,
			Heading:  90,
			SailID:   1,
			Tack:     navigation.TackPort,
			Stamina:  simulation.StaminaState{Level: 90},
			ActiveSpeedPenalty: &polar.ManeuverPenalty{
				SpeedRatio: 0.5,
				Duration:   duration,
			},
		},
	}
}

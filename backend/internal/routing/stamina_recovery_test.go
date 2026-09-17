package routing

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
)

func TestNextStaminaForStepRecoversDuringCalmStep(t *testing.T) {
	got, err := nextStaminaForStep(
		simulation.StaminaState{Level: 40},
		&simulation.ManeuverState{Kind: simulation.ManeuverNone},
		nil,
		30*time.Minute,
		&simulation.StaminaRecoveryRules{PointsPerHour: 12},
	)
	if err != nil {
		t.Fatalf("récupération refusée: %v", err)
	}
	if math.Abs(float64(got.Level-46)) > 1e-12 {
		t.Errorf("stamina = %.12f, attendue 46", got.Level)
	}
}

func TestNextStaminaForStepDoesNotRecoverDuringConsumingManeuver(t *testing.T) {
	transition := &simulation.TransitionEvaluation{
		TotalConsumption: 10,
		NextStamina:      simulation.StaminaState{Level: 30},
	}
	got, err := nextStaminaForStep(
		simulation.StaminaState{Level: 40},
		&simulation.ManeuverState{Kind: simulation.ManeuverTack},
		transition,
		30*time.Minute,
		&simulation.StaminaRecoveryRules{PointsPerHour: 12},
	)
	if err != nil {
		t.Fatalf("calcul refusé: %v", err)
	}
	if got.Level != 30 {
		t.Errorf("stamina = %.3f, attendue 30", got.Level)
	}
}

func TestNextStaminaForStepDoesNotRecoverUnknownManeuverCost(t *testing.T) {
	initial := simulation.StaminaState{Level: 40}
	got, err := nextStaminaForStep(
		initial,
		&simulation.ManeuverState{
			Kind:        simulation.ManeuverNone,
			SailChanged: true,
		},
		nil,
		30*time.Minute,
		&simulation.StaminaRecoveryRules{PointsPerHour: 12},
	)
	if err != nil {
		t.Fatalf("calcul refusé: %v", err)
	}
	if got != initial {
		t.Errorf("stamina = %+v, attendue %+v", got, initial)
	}
}

func TestNextStaminaForStepPreservesStateWhenManeuverIsUnknown(t *testing.T) {
	initial := simulation.StaminaState{Level: 40}
	got, err := nextStaminaForStep(
		initial,
		nil,
		nil,
		30*time.Minute,
		&simulation.StaminaRecoveryRules{PointsPerHour: 12},
	)
	if err != nil {
		t.Fatalf("calcul refusé: %v", err)
	}
	if got != initial {
		t.Errorf("stamina = %+v, attendue %+v", got, initial)
	}
}

func TestNextStaminaForStepRejectsInvalidRecovery(t *testing.T) {
	_, err := nextStaminaForStep(
		simulation.StaminaState{Level: 40},
		&simulation.ManeuverState{Kind: simulation.ManeuverNone},
		nil,
		-time.Minute,
		&simulation.StaminaRecoveryRules{PointsPerHour: 12},
	)
	if err == nil || !strings.Contains(err.Error(), "récupération") {
		t.Fatalf("erreur = %v, récupération invalide attendue", err)
	}
}

package simulation

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
)

func TestTransitionEvaluatorEvaluatesTack(t *testing.T) {
	evaluator := newTestTransitionEvaluator()
	result, err := evaluator.Evaluate(TransitionEvaluationRequest{
		Transition: ManeuverTransition{
			PreviousTack:      navigation.TackStarboard,
			NextTack:          navigation.TackPort,
			PreviousWindAngle: 60,
			NextWindAngle:     50,
			PreviousSailID:    1,
			NextSailID:        1,
		},
		WindSpeed: 20,
		Stamina:   StaminaState{Level: 100},
	})
	if err != nil {
		t.Fatalf("évaluation refusée: %v", err)
	}

	if result.Maneuver != (ManeuverState{Kind: ManeuverTack}) {
		t.Errorf("manœuvre = %+v", result.Maneuver)
	}
	if result.NauticalPenalty == nil {
		t.Fatal("pénalité nautique absente")
	}
	if result.NauticalPenalty.SpeedRatio != 0.5 {
		t.Errorf(
			"ratio = %.6f, attendu 0.5",
			result.NauticalPenalty.SpeedRatio,
		)
	}
	if result.NauticalPenalty.Duration != 4*time.Minute {
		t.Errorf(
			"durée = %s, attendue 4m",
			result.NauticalPenalty.Duration,
		)
	}
	if result.SailChangePenalty != nil {
		t.Errorf(
			"pénalité de changement de voile inattendue: %+v",
			result.SailChangePenalty,
		)
	}
	if result.Consumption != (StaminaConsumption{Nautical: 10}) {
		t.Errorf("consommation = %+v", result.Consumption)
	}
	if result.TotalConsumption != 10 {
		t.Errorf(
			"consommation totale = %.6f, attendue 10",
			result.TotalConsumption,
		)
	}
	if result.NextStamina.Level != 90 {
		t.Errorf(
			"stamina suivante = %.6f, attendue 90",
			result.NextStamina.Level,
		)
	}
}

func TestTransitionEvaluatorEvaluatesProfessionalGybe(t *testing.T) {
	evaluator := newTestTransitionEvaluator()
	evaluator.DurationCurve = StaminaDurationCurve{
		Points: []StaminaDurationPoint{
			{Stamina: 0, Factor: 2},
		},
	}
	result, err := evaluator.Evaluate(TransitionEvaluationRequest{
		Transition: ManeuverTransition{
			PreviousTack:      navigation.TackPort,
			NextTack:          navigation.TackStarboard,
			PreviousWindAngle: 120,
			NextWindAngle:     100,
			PreviousSailID:    1,
			NextSailID:        1,
		},
		WindSpeed:           20,
		Stamina:             StaminaState{Level: 50},
		ProfessionalWinches: true,
	})
	if err != nil {
		t.Fatalf("évaluation refusée: %v", err)
	}

	if result.Maneuver.Kind != ManeuverGybe {
		t.Errorf("manœuvre = %q, attendue %q", result.Maneuver.Kind, ManeuverGybe)
	}
	if result.NauticalPenalty == nil {
		t.Fatal("pénalité nautique absente")
	}
	if result.NauticalPenalty.SpeedRatio != 0.7 {
		t.Errorf(
			"ratio = %.6f, attendu 0.7",
			result.NauticalPenalty.SpeedRatio,
		)
	}
	if result.NauticalPenalty.Duration != 5*time.Minute {
		t.Errorf(
			"durée = %s, attendue 5m",
			result.NauticalPenalty.Duration,
		)
	}
	if result.NextStamina.Level != 40 {
		t.Errorf(
			"stamina suivante = %.6f, attendue 40",
			result.NextStamina.Level,
		)
	}
}

func TestTransitionEvaluatorEvaluatesSailChange(t *testing.T) {
	evaluator := newTestTransitionEvaluator()
	result, err := evaluator.Evaluate(TransitionEvaluationRequest{
		Transition: ManeuverTransition{
			PreviousTack:      navigation.TackPort,
			NextTack:          navigation.TackPort,
			PreviousWindAngle: 70,
			NextWindAngle:     75,
			PreviousSailID:    1,
			NextSailID:        2,
		},
		WindSpeed: 20,
		Stamina:   StaminaState{Level: 100},
	})
	if err != nil {
		t.Fatalf("évaluation refusée: %v", err)
	}

	if result.NauticalPenalty != nil {
		t.Errorf("pénalité nautique inattendue: %+v", result.NauticalPenalty)
	}
	if result.SailChangePenalty == nil {
		t.Fatal("pénalité de changement de voile absente")
	}
	if result.SailChangePenalty.SpeedRatio != 0.4 {
		t.Errorf(
			"ratio = %.6f, attendu 0.4",
			result.SailChangePenalty.SpeedRatio,
		)
	}
	if result.SailChangePenalty.Duration != 6*time.Minute {
		t.Errorf(
			"durée = %s, attendue 6m",
			result.SailChangePenalty.Duration,
		)
	}
	if result.Consumption != (StaminaConsumption{SailChange: 20}) {
		t.Errorf("consommation = %+v", result.Consumption)
	}
	if result.NextStamina.Level != 80 {
		t.Errorf(
			"stamina suivante = %.6f, attendue 80",
			result.NextStamina.Level,
		)
	}
}

func TestTransitionEvaluatorEvaluatesNoAction(t *testing.T) {
	evaluator := newTestTransitionEvaluator()
	result, err := evaluator.Evaluate(TransitionEvaluationRequest{
		Transition: ManeuverTransition{
			PreviousTack:      navigation.TackPort,
			NextTack:          navigation.TackPort,
			PreviousWindAngle: 70,
			NextWindAngle:     75,
			PreviousSailID:    1,
			NextSailID:        1,
		},
		WindSpeed: 20,
		Stamina:   StaminaState{Level: 72},
	})
	if err != nil {
		t.Fatalf("évaluation refusée: %v", err)
	}

	if result.Maneuver.Active() {
		t.Errorf("manœuvre inattendue: %+v", result.Maneuver)
	}
	if result.NauticalPenalty != nil || result.SailChangePenalty != nil {
		t.Errorf("pénalités inattendues: %+v", result)
	}
	if result.TotalConsumption != 0 {
		t.Errorf(
			"consommation totale = %.6f, attendue 0",
			result.TotalConsumption,
		)
	}
	if result.NextStamina.Level != 72 {
		t.Errorf(
			"stamina suivante = %.6f, attendue 72",
			result.NextStamina.Level,
		)
	}
}

func TestTransitionEvaluatorAppliesWindFactorToConsumption(t *testing.T) {
	evaluator := newTestTransitionEvaluator()
	evaluator.WindCurve = StaminaWindCurve{
		Points: []StaminaWindPoint{
			{WindSpeed: 0, Factor: 1.5},
		},
	}
	result, err := evaluator.Evaluate(TransitionEvaluationRequest{
		Transition: ManeuverTransition{
			PreviousTack:      navigation.TackStarboard,
			NextTack:          navigation.TackPort,
			PreviousWindAngle: 60,
			NextWindAngle:     50,
			PreviousSailID:    1,
			NextSailID:        1,
		},
		WindSpeed: 20,
		Stamina:   StaminaState{Level: 100},
	})
	if err != nil {
		t.Fatalf("évaluation refusée: %v", err)
	}
	if result.TotalConsumption != 15 {
		t.Errorf(
			"consommation totale = %.6f, attendue 15",
			result.TotalConsumption,
		)
	}
	if result.NextStamina.Level != 85 {
		t.Errorf(
			"stamina suivante = %.6f, attendue 85",
			result.NextStamina.Level,
		)
	}
}

func TestTransitionEvaluatorRequiresCombinerForSimultaneousActions(
	t *testing.T,
) {
	evaluator := newTestTransitionEvaluator()
	_, err := evaluator.Evaluate(simultaneousTransitionRequest())
	if err == nil || !strings.Contains(err.Error(), "règle de cumul") {
		t.Fatalf(
			"erreur = %v, doit signaler la règle de cumul absente",
			err,
		)
	}
}

func TestTransitionEvaluatorUsesExplicitCombinerForSimultaneousActions(
	t *testing.T,
) {
	evaluator := newTestTransitionEvaluator()
	evaluator.CombineConsumption = func(
		consumption StaminaConsumption,
	) (StaminaLevel, error) {
		return consumption.Nautical + consumption.SailChange, nil
	}

	result, err := evaluator.Evaluate(simultaneousTransitionRequest())
	if err != nil {
		t.Fatalf("évaluation refusée: %v", err)
	}
	if result.NauticalPenalty == nil {
		t.Fatal("pénalité nautique absente")
	}
	if result.SailChangePenalty == nil {
		t.Fatal("pénalité de changement de voile absente")
	}
	if result.Consumption != (StaminaConsumption{
		Nautical:   10,
		SailChange: 20,
	}) {
		t.Errorf("consommation = %+v", result.Consumption)
	}
	if result.TotalConsumption != 30 {
		t.Errorf(
			"consommation totale = %.6f, attendue 30",
			result.TotalConsumption,
		)
	}
	if result.NextStamina.Level != 70 {
		t.Errorf(
			"stamina suivante = %.6f, attendue 70",
			result.NextStamina.Level,
		)
	}
}

func TestTransitionEvaluatorRejectsCombinerError(t *testing.T) {
	evaluator := newTestTransitionEvaluator()
	evaluator.CombineConsumption = func(
		StaminaConsumption,
	) (StaminaLevel, error) {
		return 0, errors.New("règle indisponible")
	}
	_, err := evaluator.Evaluate(simultaneousTransitionRequest())
	if err == nil || !strings.Contains(err.Error(), "règle indisponible") {
		t.Fatalf("erreur = %v, doit contenir l'erreur du cumul", err)
	}
}

func TestTransitionEvaluatorRejectsInvalidCombinedConsumption(t *testing.T) {
	evaluator := newTestTransitionEvaluator()
	evaluator.CombineConsumption = func(
		StaminaConsumption,
	) (StaminaLevel, error) {
		return 101, nil
	}
	_, err := evaluator.Evaluate(simultaneousTransitionRequest())
	if err == nil || !strings.Contains(err.Error(), "cumul de stamina invalide") {
		t.Fatalf("erreur = %v, doit signaler un cumul invalide", err)
	}
}

func TestTransitionEvaluatorRejectsInvalidRequest(t *testing.T) {
	evaluator := newTestTransitionEvaluator()
	request := TransitionEvaluationRequest{
		Transition: ManeuverTransition{
			PreviousTack:      navigation.TackPort,
			NextTack:          navigation.TackPort,
			PreviousWindAngle: 70,
			NextWindAngle:     75,
			PreviousSailID:    1,
			NextSailID:        1,
		},
		WindSpeed: 20,
		Stamina:   StaminaState{Level: -1},
	}
	_, err := evaluator.Evaluate(request)
	if err == nil || !strings.Contains(err.Error(), "état de stamina invalide") {
		t.Fatalf("erreur = %v, doit signaler la stamina invalide", err)
	}
}

func simultaneousTransitionRequest() TransitionEvaluationRequest {
	return TransitionEvaluationRequest{
		Transition: ManeuverTransition{
			PreviousTack:      navigation.TackStarboard,
			NextTack:          navigation.TackPort,
			PreviousWindAngle: 60,
			NextWindAngle:     50,
			PreviousSailID:    1,
			NextSailID:        2,
		},
		WindSpeed: 20,
		Stamina:   StaminaState{Level: 100},
	}
}

func newTestTransitionEvaluator() TransitionEvaluator {
	standardTack := &polar.WindDependentPenalty{
		LowWind: polar.ManeuverPenalty{
			SpeedRatio: 0.5,
			Duration:   3 * time.Minute,
		},
		HighWind: polar.ManeuverPenalty{
			SpeedRatio: 0.5,
			Duration:   5 * time.Minute,
		},
	}
	professionalGybe := &polar.WindDependentPenalty{
		LowWind: polar.ManeuverPenalty{
			SpeedRatio: 0.7,
			Duration:   2 * time.Minute,
		},
		HighWind: polar.ManeuverPenalty{
			SpeedRatio: 0.7,
			Duration:   3 * time.Minute,
		},
	}
	standardSailChange := &polar.WindDependentPenalty{
		LowWind: polar.ManeuverPenalty{
			SpeedRatio: 0.4,
			Duration:   5 * time.Minute,
		},
		HighWind: polar.ManeuverPenalty{
			SpeedRatio: 0.4,
			Duration:   7 * time.Minute,
		},
	}

	return TransitionEvaluator{
		WinchProfile: polar.WinchProfile{
			LowWindThreshold:  10,
			HighWindThreshold: 30,
			Tack: polar.ManeuverProfile{
				Standard:     standardTack,
				Professional: standardTack,
			},
			Gybe: polar.ManeuverProfile{
				Standard:     professionalGybe,
				Professional: professionalGybe,
			},
			SailChange: polar.ManeuverProfile{
				Standard:     standardSailChange,
				Professional: standardSailChange,
			},
		},
		ConsumptionRules: DefaultStaminaConsumptionRules(),
		WindCurve: StaminaWindCurve{
			Points: []StaminaWindPoint{
				{WindSpeed: 0, Factor: 1},
			},
		},
		DurationCurve: StaminaDurationCurve{
			Points: []StaminaDurationPoint{
				{Stamina: 0, Factor: 1},
			},
		},
	}
}

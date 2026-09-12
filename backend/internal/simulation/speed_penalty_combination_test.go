package simulation

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
)

func TestCombinedSpeedPenaltyWithoutPenalty(t *testing.T) {
	combined, err := (TransitionEvaluation{}).CombinedSpeedPenalty(nil)
	if err != nil {
		t.Fatalf("combinaison refusée: %v", err)
	}
	if combined != nil {
		t.Fatalf("pénalité = %+v, attendue nil", combined)
	}
}

func TestCombinedSpeedPenaltyReturnsNauticalPenaltyCopy(t *testing.T) {
	original := polar.ManeuverPenalty{
		SpeedRatio: 0.5,
		Duration:   4 * time.Minute,
	}
	evaluation := TransitionEvaluation{NauticalPenalty: &original}

	combined, err := evaluation.CombinedSpeedPenalty(nil)
	if err != nil {
		t.Fatalf("combinaison refusée: %v", err)
	}
	if combined == nil {
		t.Fatal("pénalité combinée absente")
	}
	if *combined != original {
		t.Errorf("pénalité = %+v, attendue %+v", *combined, original)
	}
	if combined == evaluation.NauticalPenalty {
		t.Error("la pénalité retournée doit être une copie")
	}

	combined.Duration = time.Minute
	if original.Duration != 4*time.Minute {
		t.Errorf("la pénalité source a été modifiée: %+v", original)
	}
}

func TestCombinedSpeedPenaltyReturnsSailChangePenaltyCopy(t *testing.T) {
	original := polar.ManeuverPenalty{
		SpeedRatio: 0.4,
		Duration:   6 * time.Minute,
	}
	evaluation := TransitionEvaluation{SailChangePenalty: &original}

	combined, err := evaluation.CombinedSpeedPenalty(nil)
	if err != nil {
		t.Fatalf("combinaison refusée: %v", err)
	}
	if combined == nil {
		t.Fatal("pénalité combinée absente")
	}
	if *combined != original {
		t.Errorf("pénalité = %+v, attendue %+v", *combined, original)
	}
	if combined == evaluation.SailChangePenalty {
		t.Error("la pénalité retournée doit être une copie")
	}
}

func TestCombinedSpeedPenaltyRequiresExplicitRuleForSimultaneousPenalties(
	t *testing.T,
) {
	evaluation := simultaneousSpeedPenalties()

	_, err := evaluation.CombinedSpeedPenalty(nil)
	if err == nil || !strings.Contains(err.Error(), "règle explicite") {
		t.Fatalf(
			"erreur = %v, doit signaler la règle explicite absente",
			err,
		)
	}
}

func TestCombinedSpeedPenaltyUsesExplicitRule(t *testing.T) {
	evaluation := simultaneousSpeedPenalties()
	var receivedNautical polar.ManeuverPenalty
	var receivedSailChange polar.ManeuverPenalty

	combined, err := evaluation.CombinedSpeedPenalty(func(
		nautical polar.ManeuverPenalty,
		sailChange polar.ManeuverPenalty,
	) (polar.ManeuverPenalty, error) {
		receivedNautical = nautical
		receivedSailChange = sailChange
		return polar.ManeuverPenalty{
			SpeedRatio: nautical.SpeedRatio * sailChange.SpeedRatio,
			Duration:   nautical.Duration + sailChange.Duration,
		}, nil
	})
	if err != nil {
		t.Fatalf("combinaison refusée: %v", err)
	}

	if receivedNautical != *evaluation.NauticalPenalty {
		t.Errorf(
			"pénalité nautique reçue = %+v, attendue %+v",
			receivedNautical,
			*evaluation.NauticalPenalty,
		)
	}
	if receivedSailChange != *evaluation.SailChangePenalty {
		t.Errorf(
			"pénalité de voile reçue = %+v, attendue %+v",
			receivedSailChange,
			*evaluation.SailChangePenalty,
		)
	}
	if combined == nil {
		t.Fatal("pénalité combinée absente")
	}
	if combined.SpeedRatio != 0.2 {
		t.Errorf("ratio = %.6f, attendu 0.2", combined.SpeedRatio)
	}
	if combined.Duration != 10*time.Minute {
		t.Errorf("durée = %s, attendue 10m", combined.Duration)
	}
}

func TestCombinedSpeedPenaltyWrapsRuleError(t *testing.T) {
	evaluation := simultaneousSpeedPenalties()

	_, err := evaluation.CombinedSpeedPenalty(func(
		polar.ManeuverPenalty,
		polar.ManeuverPenalty,
	) (polar.ManeuverPenalty, error) {
		return polar.ManeuverPenalty{}, errors.New("règle indisponible")
	})
	if err == nil || !strings.Contains(err.Error(), "règle indisponible") {
		t.Fatalf("erreur = %v, doit contenir l'erreur de la règle", err)
	}
	if !strings.Contains(err.Error(), "combinaison de pénalités impossible") {
		t.Fatalf("erreur insuffisamment contextualisée: %v", err)
	}
}

func TestCombinedSpeedPenaltyRejectsInvalidRuleResult(t *testing.T) {
	evaluation := simultaneousSpeedPenalties()

	_, err := evaluation.CombinedSpeedPenalty(func(
		polar.ManeuverPenalty,
		polar.ManeuverPenalty,
	) (polar.ManeuverPenalty, error) {
		return polar.ManeuverPenalty{
			SpeedRatio: 1.1,
			Duration:   time.Minute,
		}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "résultat invalide") {
		t.Fatalf("erreur = %v, doit signaler le résultat invalide", err)
	}
}

func TestCombinedSpeedPenaltyRejectsInvalidSinglePenalty(t *testing.T) {
	invalid := polar.ManeuverPenalty{
		SpeedRatio: -0.1,
		Duration:   time.Minute,
	}
	evaluation := TransitionEvaluation{NauticalPenalty: &invalid}

	_, err := evaluation.CombinedSpeedPenalty(nil)
	if err == nil || !strings.Contains(err.Error(), "pénalité nautique invalide") {
		t.Fatalf("erreur = %v, doit signaler la pénalité invalide", err)
	}
}

func simultaneousSpeedPenalties() TransitionEvaluation {
	nautical := polar.ManeuverPenalty{
		SpeedRatio: 0.5,
		Duration:   4 * time.Minute,
	}
	sailChange := polar.ManeuverPenalty{
		SpeedRatio: 0.4,
		Duration:   6 * time.Minute,
	}
	return TransitionEvaluation{
		NauticalPenalty:   &nautical,
		SailChangePenalty: &sailChange,
	}
}

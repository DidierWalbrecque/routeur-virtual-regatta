package simulation

import (
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
)

func TestCombineSpeedPenaltiesUsesExplicitRule(t *testing.T) {
	active := polar.ManeuverPenalty{
		SpeedRatio: 0.8,
		Duration:   5 * time.Minute,
	}
	triggered := polar.ManeuverPenalty{
		SpeedRatio: 0.5,
		Duration:   10 * time.Minute,
	}

	combined, err := CombineSpeedPenalties(
		&active,
		&triggered,
		func(
			first polar.ManeuverPenalty,
			second polar.ManeuverPenalty,
		) (polar.ManeuverPenalty, error) {
			return polar.ManeuverPenalty{
				SpeedRatio: first.SpeedRatio * second.SpeedRatio,
				Duration:   first.Duration + second.Duration,
			}, nil
		},
	)
	if err != nil {
		t.Fatalf("combinaison refusée: %v", err)
	}
	if combined == nil {
		t.Fatal("pénalité combinée absente")
	}
	if combined.SpeedRatio != 0.4 {
		t.Errorf("ratio = %.6f, attendu 0.4", combined.SpeedRatio)
	}
	if combined.Duration != 15*time.Minute {
		t.Errorf("durée = %s, attendue 15m", combined.Duration)
	}
	if active.SpeedRatio != 0.8 || active.Duration != 5*time.Minute {
		t.Errorf("pénalité active modifiée: %+v", active)
	}
	if triggered.SpeedRatio != 0.5 ||
		triggered.Duration != 10*time.Minute {
		t.Errorf("nouvelle pénalité modifiée: %+v", triggered)
	}
}

func TestCombineSpeedPenaltiesRequiresExplicitRule(t *testing.T) {
	active := polar.ManeuverPenalty{
		SpeedRatio: 0.8,
		Duration:   5 * time.Minute,
	}
	triggered := polar.ManeuverPenalty{
		SpeedRatio: 0.5,
		Duration:   10 * time.Minute,
	}

	_, err := CombineSpeedPenalties(&active, &triggered, nil)
	if err == nil || !strings.Contains(err.Error(), "règle explicite") {
		t.Fatalf(
			"erreur = %v, doit signaler la règle explicite absente",
			err,
		)
	}
}

func TestApplySpeedPenaltyToStep(t *testing.T) {
	start := navigation.Position{Latitude: 13, Longitude: 121}
	at := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)
	original := StepResult{
		Position: start,
		Instant:  at,
		Heading:  90,
		Duration: 10 * time.Minute,
		BestSail: polar.SailPerformance{
			SailID:   1,
			SailName: "Jib",
			Speed:    12,
		},
		Distance:     2,
		NextPosition: navigation.Position{Latitude: 14, Longitude: 122},
		NextInstant:  at.Add(10 * time.Minute),
	}
	penalty := polar.ManeuverPenalty{
		SpeedRatio: 0.5,
		Duration:   4 * time.Minute,
	}

	corrected, application, err := ApplySpeedPenaltyToStep(
		original,
		&penalty,
	)
	if err != nil {
		t.Fatalf("correction refusée: %v", err)
	}

	assertClose(t, "distance corrigée", float64(corrected.Distance), 1.6)
	if application.PenalizedDuration != 4*time.Minute {
		t.Errorf(
			"durée pénalisée = %s, attendue 4m",
			application.PenalizedDuration,
		)
	}
	if application.UnpenalizedDuration != 6*time.Minute {
		t.Errorf(
			"durée normale = %s, attendue 6m",
			application.UnpenalizedDuration,
		)
	}
	if application.AppliedPenalty == nil ||
		*application.AppliedPenalty != penalty {
		t.Errorf(
			"pénalité appliquée = %+v, attendue %+v",
			application.AppliedPenalty,
			penalty,
		)
	}
	if application.RemainingPenalty != nil {
		t.Errorf(
			"pénalité restante inattendue: %+v",
			application.RemainingPenalty,
		)
	}
	if corrected.NextInstant != original.NextInstant {
		t.Errorf(
			"instant final = %s, attendu %s",
			corrected.NextInstant,
			original.NextInstant,
		)
	}
	if corrected.NextPosition == original.NextPosition {
		t.Error("la position non pénalisée a été conservée")
	}

	expectedPosition, err := navigation.DestinationPosition(
		start,
		90,
		1.6,
	)
	if err != nil {
		t.Fatalf("position attendue impossible: %v", err)
	}
	if corrected.NextPosition != expectedPosition {
		t.Errorf(
			"position corrigée = %+v, attendue %+v",
			corrected.NextPosition,
			expectedPosition,
		)
	}
	if original.Distance != 2 {
		t.Errorf("le pas source a été modifié: distance %.6f", original.Distance)
	}
}

func TestApplySpeedPenaltyToStepCarriesRemainingDuration(t *testing.T) {
	at := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)
	step := StepResult{
		Position: navigation.Position{},
		Instant:  at,
		Heading:  90,
		Duration: 10 * time.Minute,
		BestSail: polar.SailPerformance{
			SailID:   1,
			SailName: "Jib",
			Speed:    12,
		},
		NextInstant: at.Add(10 * time.Minute),
	}
	penalty := polar.ManeuverPenalty{
		SpeedRatio: 0.5,
		Duration:   15 * time.Minute,
	}

	corrected, application, err := ApplySpeedPenaltyToStep(step, &penalty)
	if err != nil {
		t.Fatalf("correction refusée: %v", err)
	}
	assertClose(t, "distance corrigée", float64(corrected.Distance), 1)
	if application.RemainingPenalty == nil {
		t.Fatal("pénalité restante absente")
	}
	if application.RemainingPenalty.SpeedRatio != 0.5 {
		t.Errorf(
			"ratio restant = %.6f, attendu 0.5",
			application.RemainingPenalty.SpeedRatio,
		)
	}
	if application.RemainingPenalty.Duration != 5*time.Minute {
		t.Errorf(
			"durée restante = %s, attendue 5m",
			application.RemainingPenalty.Duration,
		)
	}
}

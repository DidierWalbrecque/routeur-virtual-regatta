package simulation

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
)

func TestApplyManeuverSpeedPenaltyWithoutPenalty(t *testing.T) {
	result, err := ApplyManeuverSpeedPenalty(
		10,
		10*time.Minute,
		nil,
	)
	if err != nil {
		t.Fatalf("calcul refusé: %v", err)
	}
	assertPenaltyDistance(t, result.Distance, 10.0/6.0)
	if result.PenalizedDuration != 0 {
		t.Errorf(
			"durée pénalisée = %s, attendue 0",
			result.PenalizedDuration,
		)
	}
	if result.UnpenalizedDuration != 10*time.Minute {
		t.Errorf(
			"durée normale = %s, attendue 10m",
			result.UnpenalizedDuration,
		)
	}
	if result.RemainingPenalty != nil {
		t.Errorf(
			"reliquat inattendu: %+v",
			result.RemainingPenalty,
		)
	}
}

func TestApplyManeuverSpeedPenaltyWithinStep(t *testing.T) {
	penalty := &polar.ManeuverPenalty{
		SpeedRatio: 0.5,
		Duration:   4 * time.Minute,
	}
	result, err := ApplyManeuverSpeedPenalty(
		10,
		10*time.Minute,
		penalty,
	)
	if err != nil {
		t.Fatalf("calcul refusé: %v", err)
	}

	assertPenaltyDistance(t, result.Distance, 4.0/3.0)
	if result.PenalizedDuration != 4*time.Minute {
		t.Errorf(
			"durée pénalisée = %s, attendue 4m",
			result.PenalizedDuration,
		)
	}
	if result.UnpenalizedDuration != 6*time.Minute {
		t.Errorf(
			"durée normale = %s, attendue 6m",
			result.UnpenalizedDuration,
		)
	}
	if result.RemainingPenalty != nil {
		t.Errorf(
			"reliquat inattendu: %+v",
			result.RemainingPenalty,
		)
	}
}

func TestApplyManeuverSpeedPenaltyCarriesRemainder(t *testing.T) {
	penalty := &polar.ManeuverPenalty{
		SpeedRatio: 0.5,
		Duration:   15 * time.Minute,
	}
	result, err := ApplyManeuverSpeedPenalty(
		10,
		10*time.Minute,
		penalty,
	)
	if err != nil {
		t.Fatalf("calcul refusé: %v", err)
	}

	assertPenaltyDistance(t, result.Distance, 5.0/6.0)
	if result.PenalizedDuration != 10*time.Minute {
		t.Errorf(
			"durée pénalisée = %s, attendue 10m",
			result.PenalizedDuration,
		)
	}
	if result.UnpenalizedDuration != 0 {
		t.Errorf(
			"durée normale = %s, attendue 0",
			result.UnpenalizedDuration,
		)
	}
	if result.RemainingPenalty == nil {
		t.Fatal("reliquat de pénalité absent")
	}
	if result.RemainingPenalty.SpeedRatio != 0.5 ||
		result.RemainingPenalty.Duration != 5*time.Minute {
		t.Errorf(
			"reliquat = %+v, attendu ratio 0.5 pendant 5m",
			result.RemainingPenalty,
		)
	}
	if penalty.Duration != 15*time.Minute {
		t.Errorf(
			"pénalité d'entrée modifiée: %s",
			penalty.Duration,
		)
	}
}

func TestApplyManeuverSpeedPenaltyRejectsInvalidInputs(t *testing.T) {
	validPenalty := polar.ManeuverPenalty{
		SpeedRatio: 0.5,
		Duration:   time.Minute,
	}
	tests := []struct {
		name     string
		speed    navigation.Knots
		duration time.Duration
		penalty  *polar.ManeuverPenalty
		wantErr  string
	}{
		{
			name:     "vitesse négative",
			speed:    -1,
			duration: time.Minute,
			penalty:  &validPenalty,
			wantErr:  "vitesse invalide",
		},
		{
			name:     "durée nulle",
			speed:    10,
			duration: 0,
			penalty:  &validPenalty,
			wantErr:  "durée de pas",
		},
		{
			name:     "ratio supérieur à un",
			speed:    10,
			duration: time.Minute,
			penalty: &polar.ManeuverPenalty{
				SpeedRatio: 1.1,
				Duration:   time.Minute,
			},
			wantErr: "coefficient de vitesse",
		},
		{
			name:     "ratio non fini",
			speed:    10,
			duration: time.Minute,
			penalty: &polar.ManeuverPenalty{
				SpeedRatio: polar.SpeedRatio(math.NaN()),
				Duration:   time.Minute,
			},
			wantErr: "coefficient de vitesse",
		},
		{
			name:     "durée de pénalité nulle",
			speed:    10,
			duration: time.Minute,
			penalty: &polar.ManeuverPenalty{
				SpeedRatio: 0.5,
			},
			wantErr: "durée de pénalité",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ApplyManeuverSpeedPenalty(
				test.speed,
				test.duration,
				test.penalty,
			)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf(
					"erreur = %v, doit contenir %q",
					err,
					test.wantErr,
				)
			}
		})
	}
}

func assertPenaltyDistance(
	t *testing.T,
	got navigation.NauticalMiles,
	want float64,
) {
	t.Helper()
	if math.Abs(float64(got)-want) > 1e-9 {
		t.Errorf(
			"distance = %.12f NM, attendue %.12f NM",
			got,
			want,
		)
	}
}

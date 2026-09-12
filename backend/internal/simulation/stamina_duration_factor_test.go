package simulation

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
)

func TestStaminaDurationCurveFactorAt(t *testing.T) {
	curve := testStaminaDurationCurve()
	tests := []struct {
		name    string
		stamina StaminaLevel
		want    ManeuverDurationFactor
	}{
		{
			name:    "premier point",
			stamina: 0,
			want:    2,
		},
		{
			name:    "premier intervalle",
			stamina: 25,
			want:    1.75,
		},
		{
			name:    "point central",
			stamina: 50,
			want:    1.5,
		},
		{
			name:    "second intervalle",
			stamina: 75,
			want:    1.25,
		},
		{
			name:    "dernier point",
			stamina: 100,
			want:    1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := curve.FactorAt(test.stamina)
			if err != nil {
				t.Fatalf("interpolation refusée: %v", err)
			}
			if math.Abs(float64(got-test.want)) > 1e-12 {
				t.Errorf(
					"coefficient = %.12f, attendu %.12f",
					got,
					test.want,
				)
			}
		})
	}
}

func TestStaminaDurationCurveClampsToEndpoints(t *testing.T) {
	curve := StaminaDurationCurve{
		Points: []StaminaDurationPoint{
			{Stamina: 20, Factor: 1.8},
			{Stamina: 80, Factor: 1.1},
		},
	}
	tests := []struct {
		stamina StaminaLevel
		want    ManeuverDurationFactor
	}{
		{stamina: 0, want: 1.8},
		{stamina: 100, want: 1.1},
	}
	for _, test := range tests {
		got, err := curve.FactorAt(test.stamina)
		if err != nil {
			t.Fatalf("stamina %.2f refusée: %v", test.stamina, err)
		}
		if got != test.want {
			t.Errorf(
				"stamina %.2f: coefficient = %.6f, attendu %.6f",
				test.stamina,
				got,
				test.want,
			)
		}
	}
}

func TestStaminaDurationCurveAllowsConstantFactor(t *testing.T) {
	curve := StaminaDurationCurve{
		Points: []StaminaDurationPoint{
			{Stamina: 50, Factor: 1.25},
		},
	}
	for _, stamina := range []StaminaLevel{0, 50, 100} {
		got, err := curve.FactorAt(stamina)
		if err != nil {
			t.Fatalf("stamina %.2f refusée: %v", stamina, err)
		}
		if got != 1.25 {
			t.Errorf(
				"stamina %.2f: coefficient = %.6f, attendu 1.25",
				stamina,
				got,
			)
		}
	}
}

func TestStaminaDurationCurveAdjustPenalty(t *testing.T) {
	penalty := polar.ManeuverPenalty{
		SpeedRatio: 0.5,
		Duration:   4 * time.Minute,
	}
	got, err := testStaminaDurationCurve().AdjustPenalty(
		penalty,
		25,
	)
	if err != nil {
		t.Fatalf("ajustement refusé: %v", err)
	}
	if got.SpeedRatio != penalty.SpeedRatio {
		t.Errorf(
			"ratio = %.6f, attendu %.6f",
			got.SpeedRatio,
			penalty.SpeedRatio,
		)
	}
	if got.Duration != 7*time.Minute {
		t.Errorf(
			"durée = %s, attendue %s",
			got.Duration,
			7*time.Minute,
		)
	}
}

func TestStaminaDurationCurveAdjustPenaltyRoundsToNanosecond(t *testing.T) {
	curve := StaminaDurationCurve{
		Points: []StaminaDurationPoint{
			{Stamina: 0, Factor: 1.5},
		},
	}
	penalty := polar.ManeuverPenalty{
		SpeedRatio: 0.7,
		Duration:   time.Nanosecond,
	}
	got, err := curve.AdjustPenalty(penalty, 100)
	if err != nil {
		t.Fatalf("ajustement refusé: %v", err)
	}
	if got.Duration != 2*time.Nanosecond {
		t.Errorf(
			"durée = %s, attendue %s",
			got.Duration,
			2*time.Nanosecond,
		)
	}
}

func TestStaminaDurationCurveRejectsInvalidInputs(t *testing.T) {
	validCurve := testStaminaDurationCurve()
	tests := []struct {
		name    string
		stamina StaminaLevel
		curve   StaminaDurationCurve
		wantErr string
	}{
		{
			name:    "stamina négative",
			stamina: -1,
			curve:   validCurve,
			wantErr: "stamina",
		},
		{
			name:    "stamina non finie",
			stamina: StaminaLevel(math.NaN()),
			curve:   validCurve,
			wantErr: "stamina",
		},
		{
			name:    "courbe vide",
			stamina: 50,
			curve:   StaminaDurationCurve{},
			wantErr: "courbe",
		},
		{
			name:    "niveau de point invalide",
			stamina: 50,
			curve: StaminaDurationCurve{
				Points: []StaminaDurationPoint{
					{Stamina: 101, Factor: 1},
				},
			},
			wantErr: "stamina invalide",
		},
		{
			name:    "niveaux dupliqués",
			stamina: 50,
			curve: StaminaDurationCurve{
				Points: []StaminaDurationPoint{
					{Stamina: 20, Factor: 2},
					{Stamina: 20, Factor: 1},
				},
			},
			wantErr: "non strictement croissants",
		},
		{
			name:    "coefficient nul",
			stamina: 50,
			curve: StaminaDurationCurve{
				Points: []StaminaDurationPoint{
					{Stamina: 20, Factor: 0},
				},
			},
			wantErr: "coefficient de durée",
		},
		{
			name:    "coefficient négatif",
			stamina: 50,
			curve: StaminaDurationCurve{
				Points: []StaminaDurationPoint{
					{Stamina: 20, Factor: -1},
				},
			},
			wantErr: "coefficient de durée",
		},
		{
			name:    "coefficient non fini",
			stamina: 50,
			curve: StaminaDurationCurve{
				Points: []StaminaDurationPoint{
					{
						Stamina: 20,
						Factor: ManeuverDurationFactor(
							math.Inf(1),
						),
					},
				},
			},
			wantErr: "coefficient de durée",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.curve.FactorAt(test.stamina)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %v, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

func TestStaminaDurationCurveAdjustPenaltyRejectsInvalidPenalty(
	t *testing.T,
) {
	curve := testStaminaDurationCurve()
	tests := []struct {
		name    string
		penalty polar.ManeuverPenalty
		wantErr string
	}{
		{
			name: "ratio négatif",
			penalty: polar.ManeuverPenalty{
				SpeedRatio: -0.1,
				Duration:   time.Minute,
			},
			wantErr: "ratio de vitesse",
		},
		{
			name: "ratio supérieur à un",
			penalty: polar.ManeuverPenalty{
				SpeedRatio: 1.1,
				Duration:   time.Minute,
			},
			wantErr: "ratio de vitesse",
		},
		{
			name: "durée nulle",
			penalty: polar.ManeuverPenalty{
				SpeedRatio: 0.5,
				Duration:   0,
			},
			wantErr: "durée de base",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := curve.AdjustPenalty(test.penalty, 50)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %v, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

func TestStaminaDurationCurveAdjustPenaltyRejectsOverflow(t *testing.T) {
	curve := StaminaDurationCurve{
		Points: []StaminaDurationPoint{
			{Stamina: 0, Factor: 2},
		},
	}
	penalty := polar.ManeuverPenalty{
		SpeedRatio: 0.5,
		Duration:   time.Duration(1<<63 - 1),
	}
	_, err := curve.AdjustPenalty(penalty, 50)
	if err == nil || !strings.Contains(err.Error(), "durée calculée") {
		t.Fatalf(
			"erreur = %v, doit signaler une durée calculée invalide",
			err,
		)
	}
}

func testStaminaDurationCurve() StaminaDurationCurve {
	return StaminaDurationCurve{
		Points: []StaminaDurationPoint{
			{Stamina: 0, Factor: 2},
			{Stamina: 50, Factor: 1.5},
			{Stamina: 100, Factor: 1},
		},
	}
}

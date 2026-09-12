package simulation

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestStaminaStateRecoveredFor(t *testing.T) {
	rules := StaminaRecoveryRules{
		PointsPerHour: 12,
	}
	tests := []struct {
		name         string
		initial      StaminaLevel
		calmDuration time.Duration
		want         StaminaLevel
	}{
		{
			name:         "aucune durée calme",
			initial:      40,
			calmDuration: 0,
			want:         40,
		},
		{
			name:         "trente minutes",
			initial:      40,
			calmDuration: 30 * time.Minute,
			want:         46,
		},
		{
			name:         "une heure et quart",
			initial:      40,
			calmDuration: 75 * time.Minute,
			want:         55,
		},
		{
			name:         "plafond à cent",
			initial:      95,
			calmDuration: time.Hour,
			want:         100,
		},
		{
			name:         "déjà au maximum",
			initial:      100,
			calmDuration: time.Hour,
			want:         100,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := (StaminaState{Level: test.initial}).RecoveredFor(
				test.calmDuration,
				rules,
			)
			if err != nil {
				t.Fatalf("récupération refusée: %v", err)
			}
			if math.Abs(float64(got.Level-test.want)) > 1e-12 {
				t.Errorf(
					"stamina = %.12f, attendue %.12f",
					got.Level,
					test.want,
				)
			}
		})
	}
}

func TestStaminaStateRecoveredForUsesConfigurableRate(t *testing.T) {
	state := StaminaState{Level: 25}
	got, err := state.RecoveredFor(
		20*time.Minute,
		StaminaRecoveryRules{PointsPerHour: 7.5},
	)
	if err != nil {
		t.Fatalf("récupération refusée: %v", err)
	}
	if math.Abs(float64(got.Level-27.5)) > 1e-12 {
		t.Errorf("stamina = %.12f, attendue 27.5", got.Level)
	}
}

func TestStaminaStateRecoveredForAllowsZeroRate(t *testing.T) {
	state := StaminaState{Level: 25}
	got, err := state.RecoveredFor(
		4*time.Hour,
		StaminaRecoveryRules{PointsPerHour: 0},
	)
	if err != nil {
		t.Fatalf("récupération refusée: %v", err)
	}
	if got != state {
		t.Errorf("état = %+v, attendu %+v", got, state)
	}
}

func TestStaminaStateRecoveredForRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name         string
		state        StaminaState
		calmDuration time.Duration
		rules        StaminaRecoveryRules
		wantErr      string
	}{
		{
			name:         "stamina négative",
			state:        StaminaState{Level: -1},
			calmDuration: time.Minute,
			rules:        StaminaRecoveryRules{PointsPerHour: 10},
			wantErr:      "état de stamina invalide",
		},
		{
			name:         "stamina supérieure à cent",
			state:        StaminaState{Level: 101},
			calmDuration: time.Minute,
			rules:        StaminaRecoveryRules{PointsPerHour: 10},
			wantErr:      "état de stamina invalide",
		},
		{
			name:         "durée négative",
			state:        StaminaState{Level: 50},
			calmDuration: -time.Second,
			rules:        StaminaRecoveryRules{PointsPerHour: 10},
			wantErr:      "durée calme",
		},
		{
			name:         "taux négatif",
			state:        StaminaState{Level: 50},
			calmDuration: time.Minute,
			rules:        StaminaRecoveryRules{PointsPerHour: -1},
			wantErr:      "taux de récupération",
		},
		{
			name:         "taux non fini",
			state:        StaminaState{Level: 50},
			calmDuration: time.Minute,
			rules: StaminaRecoveryRules{
				PointsPerHour: StaminaRecoveryRate(math.NaN()),
			},
			wantErr: "taux de récupération",
		},
		{
			name:         "taux infini",
			state:        StaminaState{Level: 50},
			calmDuration: time.Minute,
			rules: StaminaRecoveryRules{
				PointsPerHour: StaminaRecoveryRate(math.Inf(1)),
			},
			wantErr: "taux de récupération",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.state.RecoveredFor(
				test.calmDuration,
				test.rules,
			)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %v, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

func TestStaminaRecoveryRulesValidate(t *testing.T) {
	tests := []struct {
		name    string
		rules   StaminaRecoveryRules
		wantErr bool
	}{
		{
			name:  "zéro",
			rules: StaminaRecoveryRules{PointsPerHour: 0},
		},
		{
			name:  "positif",
			rules: StaminaRecoveryRules{PointsPerHour: 10},
		},
		{
			name:    "négatif",
			rules:   StaminaRecoveryRules{PointsPerHour: -1},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.rules.Validate()
			if test.wantErr && err == nil {
				t.Fatal("erreur attendue, aucune reçue")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("validation refusée: %v", err)
			}
		})
	}
}

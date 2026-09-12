package simulation

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestStaminaStateConsumedBy(t *testing.T) {
	tests := []struct {
		name   string
		state  StaminaState
		amount StaminaLevel
		want   StaminaState
	}{
		{
			name:   "aucune consommation",
			state:  StaminaState{Level: 80},
			amount: 0,
			want:   StaminaState{Level: 80},
		},
		{
			name:   "consommation entière",
			state:  StaminaState{Level: 100},
			amount: 10,
			want:   StaminaState{Level: 90},
		},
		{
			name:   "consommation décimale",
			state:  StaminaState{Level: 75.5},
			amount: 12.25,
			want:   StaminaState{Level: 63.25},
		},
		{
			name:   "consommation exacte",
			state:  StaminaState{Level: 20},
			amount: 20,
			want:   StaminaState{Level: 0},
		},
		{
			name:   "plancher à zéro",
			state:  StaminaState{Level: 15},
			amount: 20,
			want:   StaminaState{Level: 0},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.state.ConsumedBy(test.amount)
			if err != nil {
				t.Fatalf("consommation refusée: %v", err)
			}
			if got != test.want {
				t.Errorf("état = %+v, attendu %+v", got, test.want)
			}
			if err := got.Validate(); err != nil {
				t.Errorf("état produit invalide: %v", err)
			}
		})
	}
}

func TestStaminaStateConsumedByRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name    string
		state   StaminaState
		amount  StaminaLevel
		wantErr string
	}{
		{
			name:    "état négatif",
			state:   StaminaState{Level: -1},
			amount:  10,
			wantErr: "état de stamina invalide",
		},
		{
			name:    "état supérieur à cent",
			state:   StaminaState{Level: 101},
			amount:  10,
			wantErr: "état de stamina invalide",
		},
		{
			name: "état non fini",
			state: StaminaState{
				Level: StaminaLevel(math.NaN()),
			},
			amount:  10,
			wantErr: "état de stamina invalide",
		},
		{
			name:    "coût négatif",
			state:   StaminaState{Level: 100},
			amount:  -1,
			wantErr: "coût invalide",
		},
		{
			name:    "coût supérieur à cent",
			state:   StaminaState{Level: 100},
			amount:  101,
			wantErr: "coût invalide",
		},
		{
			name:   "coût non fini",
			state:  StaminaState{Level: 100},
			amount: StaminaLevel(math.Inf(1)),
			wantErr: "coût invalide",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.state.ConsumedBy(test.amount)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %v, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

func TestStaminaRemainsBoundedAcrossConsumptionAndRecovery(t *testing.T) {
	state, err := (StaminaState{Level: 15}).ConsumedBy(20)
	if err != nil {
		t.Fatalf("consommation refusée: %v", err)
	}
	if state.Level != 0 {
		t.Fatalf("stamina après consommation = %.6f, attendue 0", state.Level)
	}

	state, err = state.RecoveredFor(
		2*time.Hour,
		StaminaRecoveryRules{PointsPerHour: 60},
	)
	if err != nil {
		t.Fatalf("récupération refusée: %v", err)
	}
	if state.Level != 100 {
		t.Errorf("stamina après récupération = %.6f, attendue 100", state.Level)
	}
	if err := state.Validate(); err != nil {
		t.Errorf("état final invalide: %v", err)
	}
}

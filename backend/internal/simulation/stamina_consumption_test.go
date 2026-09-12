package simulation

import (
	"math"
	"strings"
	"testing"
)

func TestDefaultStaminaConsumptionRules(t *testing.T) {
	rules := DefaultStaminaConsumptionRules()
	if rules.Tack != 10 {
		t.Errorf("coût du virement = %.6f, attendu 10", rules.Tack)
	}
	if rules.Gybe != 10 {
		t.Errorf("coût de l'empannage = %.6f, attendu 10", rules.Gybe)
	}
	if rules.SailChange != 20 {
		t.Errorf(
			"coût du changement de voile = %.6f, attendu 20",
			rules.SailChange,
		)
	}
}

func TestStaminaConsumptionRulesConsumptionFor(t *testing.T) {
	rules := DefaultStaminaConsumptionRules()
	tests := []struct {
		name     string
		maneuver ManeuverState
		want     StaminaConsumption
	}{
		{
			name:     "aucune action",
			maneuver: ManeuverState{Kind: ManeuverNone},
			want:     StaminaConsumption{},
		},
		{
			name:     "virement",
			maneuver: ManeuverState{Kind: ManeuverTack},
			want: StaminaConsumption{
				Nautical: 10,
			},
		},
		{
			name:     "empannage",
			maneuver: ManeuverState{Kind: ManeuverGybe},
			want: StaminaConsumption{
				Nautical: 10,
			},
		},
		{
			name: "changement de voile",
			maneuver: ManeuverState{
				Kind:        ManeuverNone,
				SailChanged: true,
			},
			want: StaminaConsumption{
				SailChange: 20,
			},
		},
		{
			name: "virement et changement de voile séparés",
			maneuver: ManeuverState{
				Kind:        ManeuverTack,
				SailChanged: true,
			},
			want: StaminaConsumption{
				Nautical:   10,
				SailChange: 20,
			},
		},
		{
			name: "empannage et changement de voile séparés",
			maneuver: ManeuverState{
				Kind:        ManeuverGybe,
				SailChanged: true,
			},
			want: StaminaConsumption{
				Nautical:   10,
				SailChange: 20,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := rules.ConsumptionFor(test.maneuver)
			if err != nil {
				t.Fatalf("calcul refusé: %v", err)
			}
			if got != test.want {
				t.Errorf("consommation = %+v, attendue %+v", got, test.want)
			}
		})
	}
}

func TestStaminaConsumptionRulesAreConfigurable(t *testing.T) {
	rules := StaminaConsumptionRules{
		Tack:       8,
		Gybe:       12,
		SailChange: 17.5,
	}
	got, err := rules.ConsumptionFor(ManeuverState{
		Kind:        ManeuverGybe,
		SailChanged: true,
	})
	if err != nil {
		t.Fatalf("calcul refusé: %v", err)
	}
	want := StaminaConsumption{
		Nautical:   12,
		SailChange: 17.5,
	}
	if got != want {
		t.Errorf("consommation = %+v, attendue %+v", got, want)
	}
}

func TestStaminaConsumptionRulesRejectInvalidInputs(t *testing.T) {
	tests := []struct {
		name     string
		rules    StaminaConsumptionRules
		maneuver ManeuverState
		wantErr  string
	}{
		{
			name: "coût de virement négatif",
			rules: StaminaConsumptionRules{
				Tack:       -1,
				Gybe:       10,
				SailChange: 20,
			},
			maneuver: ManeuverState{Kind: ManeuverTack},
			wantErr:  "virement",
		},
		{
			name: "coût d'empannage supérieur à cent",
			rules: StaminaConsumptionRules{
				Tack:       10,
				Gybe:       101,
				SailChange: 20,
			},
			maneuver: ManeuverState{Kind: ManeuverGybe},
			wantErr:  "empannage",
		},
		{
			name: "coût de changement de voile non fini",
			rules: StaminaConsumptionRules{
				Tack:       10,
				Gybe:       10,
				SailChange: StaminaLevel(math.NaN()),
			},
			maneuver: ManeuverState{
				Kind:        ManeuverNone,
				SailChanged: true,
			},
			wantErr: "changement de voile",
		},
		{
			name:     "type de manœuvre inconnu",
			rules:    DefaultStaminaConsumptionRules(),
			maneuver: ManeuverState{Kind: ManeuverKind("unknown")},
			wantErr:  "type de manœuvre inconnu",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.rules.ConsumptionFor(test.maneuver)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %v, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

func TestStaminaConsumptionValidate(t *testing.T) {
	tests := []struct {
		name        string
		consumption StaminaConsumption
		wantErr     string
	}{
		{
			name:        "valide",
			consumption: StaminaConsumption{Nautical: 10, SailChange: 20},
		},
		{
			name:        "nautique invalide",
			consumption: StaminaConsumption{Nautical: -1},
			wantErr:     "nautique",
		},
		{
			name: "changement de voile invalide",
			consumption: StaminaConsumption{
				SailChange: StaminaLevel(math.Inf(1)),
			},
			wantErr: "changement de voile",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.consumption.Validate()
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("validation refusée: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %v, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

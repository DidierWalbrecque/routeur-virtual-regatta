package simulation

import (
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
)

func TestIsolatedManeuverStaminaScenarios(t *testing.T) {
	rules := DefaultStaminaConsumptionRules()
	tests := []struct {
		name       string
		transition ManeuverTransition
		wantState  ManeuverState
		wantCost   StaminaConsumption
	}{
		{
			name: "virement isolé",
			transition: ManeuverTransition{
				PreviousTack:      navigation.TackStarboard,
				NextTack:          navigation.TackPort,
				PreviousWindAngle: 60,
				NextWindAngle:     50,
				PreviousSailID:    1,
				NextSailID:        1,
			},
			wantState: ManeuverState{Kind: ManeuverTack},
			wantCost:  StaminaConsumption{Nautical: 10},
		},
		{
			name: "empannage isolé",
			transition: ManeuverTransition{
				PreviousTack:      navigation.TackPort,
				NextTack:          navigation.TackStarboard,
				PreviousWindAngle: 120,
				NextWindAngle:     100,
				PreviousSailID:    1,
				NextSailID:        1,
			},
			wantState: ManeuverState{Kind: ManeuverGybe},
			wantCost:  StaminaConsumption{Nautical: 10},
		},
		{
			name: "changement de voile isolé",
			transition: ManeuverTransition{
				PreviousTack:      navigation.TackPort,
				NextTack:          navigation.TackPort,
				PreviousWindAngle: 70,
				NextWindAngle:     75,
				PreviousSailID:    1,
				NextSailID:        2,
			},
			wantState: ManeuverState{
				Kind:        ManeuverNone,
				SailChanged: true,
			},
			wantCost: StaminaConsumption{SailChange: 20},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			maneuver, err := DetectManeuver(test.transition)
			if err != nil {
				t.Fatalf("détection refusée: %v", err)
			}
			if maneuver != test.wantState {
				t.Errorf(
					"manœuvre = %+v, attendue %+v",
					maneuver,
					test.wantState,
				)
			}

			cost, err := rules.ConsumptionFor(maneuver)
			if err != nil {
				t.Fatalf("calcul du coût refusé: %v", err)
			}
			if cost != test.wantCost {
				t.Errorf(
					"coût = %+v, attendu %+v",
					cost,
					test.wantCost,
				)
			}
		})
	}
}

func TestSuccessiveManeuverStaminaScenario(t *testing.T) {
	consumptionRules := DefaultStaminaConsumptionRules()
	recoveryRules := StaminaRecoveryRules{PointsPerHour: 10}
	state := StaminaState{Level: 100}

	steps := []struct {
		name         string
		transition   ManeuverTransition
		calmDuration time.Duration
		wantLevel    StaminaLevel
	}{
		{
			name: "virement",
			transition: ManeuverTransition{
				PreviousTack:      navigation.TackStarboard,
				NextTack:          navigation.TackPort,
				PreviousWindAngle: 60,
				NextWindAngle:     50,
				PreviousSailID:    polar.SailID(1),
				NextSailID:        polar.SailID(1),
			},
			wantLevel: 90,
		},
		{
			name: "trente minutes sans action",
			transition: ManeuverTransition{
				PreviousTack:      navigation.TackPort,
				NextTack:          navigation.TackPort,
				PreviousWindAngle: 50,
				NextWindAngle:     55,
				PreviousSailID:    polar.SailID(1),
				NextSailID:        polar.SailID(1),
			},
			calmDuration: 30 * time.Minute,
			wantLevel:    95,
		},
		{
			name: "changement de voile",
			transition: ManeuverTransition{
				PreviousTack:      navigation.TackPort,
				NextTack:          navigation.TackPort,
				PreviousWindAngle: 55,
				NextWindAngle:     60,
				PreviousSailID:    polar.SailID(1),
				NextSailID:        polar.SailID(2),
			},
			wantLevel: 75,
		},
		{
			name: "empannage",
			transition: ManeuverTransition{
				PreviousTack:      navigation.TackPort,
				NextTack:          navigation.TackStarboard,
				PreviousWindAngle: 120,
				NextWindAngle:     100,
				PreviousSailID:    polar.SailID(2),
				NextSailID:        polar.SailID(2),
			},
			wantLevel: 65,
		},
	}

	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			maneuver, err := DetectManeuver(step.transition)
			if err != nil {
				t.Fatalf("détection refusée: %v", err)
			}

			if !maneuver.Active() {
				state, err = state.RecoveredFor(
					step.calmDuration,
					recoveryRules,
				)
				if err != nil {
					t.Fatalf("récupération refusée: %v", err)
				}
			} else {
				cost, costErr := consumptionRules.ConsumptionFor(
					maneuver,
				)
				if costErr != nil {
					t.Fatalf("calcul du coût refusé: %v", costErr)
				}
				if cost.Nautical > 0 && cost.SailChange > 0 {
					t.Fatal(
						"le scénario successif ne doit pas contenir deux actions simultanées",
					)
				}
				amount := cost.Nautical
				if cost.SailChange > 0 {
					amount = cost.SailChange
				}
				state, err = state.ConsumedBy(amount)
				if err != nil {
					t.Fatalf("consommation refusée: %v", err)
				}
			}

			if state.Level != step.wantLevel {
				t.Errorf(
					"stamina = %.6f, attendue %.6f",
					state.Level,
					step.wantLevel,
				)
			}
			if err := state.Validate(); err != nil {
				t.Errorf("état produit invalide: %v", err)
			}
		})
	}
}

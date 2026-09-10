package simulation

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

func TestStaminaLevelValidate(t *testing.T) {
	for _, level := range []StaminaLevel{0, 50, 100} {
		t.Run(fmt.Sprintf("%.1f", level), func(t *testing.T) {
			if err := level.Validate(); err != nil {
				t.Fatalf("stamina %.1f refusée: %v", level, err)
			}
		})
	}
}

func TestStaminaLevelValidateRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name  string
		level StaminaLevel
	}{
		{name: "négative", level: -0.1},
		{name: "supérieure à cent", level: 100.1},
		{name: "NaN", level: StaminaLevel(math.NaN())},
		{name: "infini", level: StaminaLevel(math.Inf(1))},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.level.Validate(); err == nil {
				t.Fatalf("stamina %.3f acceptée", test.level)
			}
		})
	}
}

func TestManeuverState(t *testing.T) {
	tests := []struct {
		name   string
		state  ManeuverState
		active bool
	}{
		{
			name:   "aucune action",
			state:  ManeuverState{Kind: ManeuverNone},
			active: false,
		},
		{
			name: "changement de voile seul",
			state: ManeuverState{
				Kind:        ManeuverNone,
				SailChanged: true,
			},
			active: true,
		},
		{
			name:   "virement",
			state:  ManeuverState{Kind: ManeuverTack},
			active: true,
		},
		{
			name: "empannage et voile",
			state: ManeuverState{
				Kind:        ManeuverGybe,
				SailChanged: true,
			},
			active: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.state.Validate(); err != nil {
				t.Fatalf("état refusé: %v", err)
			}
			if got := test.state.Active(); got != test.active {
				t.Errorf("Active() = %t, attendu %t", got, test.active)
			}
		})
	}
}

func TestManeuverStateRejectsUnknownKind(t *testing.T) {
	state := ManeuverState{Kind: "turn"}
	err := state.Validate()
	if err == nil || !strings.Contains(err.Error(), "inconnu") {
		t.Fatalf("erreur = %v, doit signaler un type inconnu", err)
	}
}

func TestBoatStateValidate(t *testing.T) {
	state := validBoatState()
	if err := state.Validate(); err != nil {
		t.Fatalf("état valide refusé: %v", err)
	}
}

func TestBoatStateValidateRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*BoatState)
		wantErr string
	}{
		{
			name: "position",
			mutate: func(state *BoatState) {
				state.Position.Latitude = 91
			},
			wantErr: "position",
		},
		{
			name: "instant",
			mutate: func(state *BoatState) {
				state.Instant = time.Time{}
			},
			wantErr: "instant",
		},
		{
			name: "cap",
			mutate: func(state *BoatState) {
				state.Heading = 360
			},
			wantErr: "cap",
		},
		{
			name: "voile",
			mutate: func(state *BoatState) {
				state.SailID = 0
			},
			wantErr: "voile",
		},
		{
			name: "amure",
			mutate: func(state *BoatState) {
				state.Tack = "left"
			},
			wantErr: "amure",
		},
		{
			name: "stamina",
			mutate: func(state *BoatState) {
				state.Stamina.Level = 101
			},
			wantErr: "stamina",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := validBoatState()
			test.mutate(&state)
			err := state.Validate()
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %v, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

func validBoatState() BoatState {
	return BoatState{
		Position: navigation.Position{
			Latitude:  13,
			Longitude: 121,
		},
		Instant: time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
		Heading: 321,
		SailID:  1,
		Tack:    navigation.TackPort,
		Stamina: StaminaState{Level: 100},
	}
}


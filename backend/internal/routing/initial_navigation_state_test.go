package routing

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
)

func TestBuildIsochroneSearchUsesInitialNavigationState(t *testing.T) {
	instant := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)
	initialNavigation := InitialNavigationState{
		Heading: 120,
		SailID:  1,
	}
	provider := &staticWeatherProvider{
		wind: navigation.Wind{
			Speed:     5,
			Direction: 180,
		},
	}
	search, err := BuildIsochroneSearch(
		context.Background(),
		provider,
		testPolar(),
		IsochroneSearchInput{
			Position:          navigation.Position{},
			Destination:       navigation.Position{Longitude: 10},
			Instant:           instant,
			StepDuration:      time.Hour,
			LayerCount:        1,
			HeadingSpacing:    120,
			CellSize:          0.01,
			InitialStamina:    simulation.StaminaState{Level: 80},
			InitialNavigation: &initialNavigation,
		},
	)
	if err != nil {
		t.Fatalf("construction refusée: %v", err)
	}
	if search.InitialState == nil {
		t.Fatal("état initial absent")
	}
	if search.InitialState.Heading != 120 {
		t.Errorf(
			"cap initial = %.1f°, attendu 120°",
			search.InitialState.Heading,
		)
	}
	if search.InitialState.SailID != 1 {
		t.Errorf(
			"voile initiale = %d, attendue 1",
			search.InitialState.SailID,
		)
	}
	if search.InitialState.Tack != navigation.TackStarboard {
		t.Errorf(
			"amure initiale = %q, attendue %q",
			search.InitialState.Tack,
			navigation.TackStarboard,
		)
	}
	if search.InitialState.Stamina.Level != 80 {
		t.Errorf(
			"stamina initiale = %.1f, attendue 80",
			search.InitialState.Stamina.Level,
		)
	}

	tackCount := 0
	for index, node := range search.Frontiers[0].Nodes {
		if node.Maneuver == nil {
			t.Errorf(
				"nœud %d du premier front: manœuvre indéterminée",
				index+1,
			)
			continue
		}
		if node.Maneuver.Kind == simulation.ManeuverTack {
			tackCount++
		}
	}
	if tackCount == 0 {
		t.Fatal("aucun virement détecté depuis l'état initial")
	}
}

func TestBuildIsochroneSearchRejectsInvalidInitialNavigationState(
	t *testing.T,
) {
	tests := []struct {
		name       string
		navigation InitialNavigationState
		wantErr    string
	}{
		{
			name: "cap invalide",
			navigation: InitialNavigationState{
				Heading: 360,
				SailID:  1,
			},
			wantErr: "cap invalide",
		},
		{
			name: "voile nulle",
			navigation: InitialNavigationState{
				Heading: 120,
				SailID:  0,
			},
			wantErr: "identifiant de voile",
		},
		{
			name: "voile indisponible",
			navigation: InitialNavigationState{
				Heading: 120,
				SailID:  polar.SailID(999),
			},
			wantErr: "indisponible",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			initialNavigation := test.navigation
			_, err := BuildIsochroneSearch(
				context.Background(),
				&staticWeatherProvider{
					wind: navigation.Wind{
						Speed:     5,
						Direction: 180,
					},
				},
				testPolar(),
				IsochroneSearchInput{
					Position:          navigation.Position{},
					Destination:       navigation.Position{Longitude: 10},
					Instant:           time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
					StepDuration:      time.Hour,
					LayerCount:        1,
					HeadingSpacing:    120,
					CellSize:          0.01,
					InitialStamina:    simulation.StaminaState{Level: 100},
					InitialNavigation: &initialNavigation,
				},
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

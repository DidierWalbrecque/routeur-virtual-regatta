package routing

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
)

func TestBuildIsochroneSearchCarriesBoatState(t *testing.T) {
	origin := navigation.Position{}
	instant := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)
	initialStamina := simulation.StaminaState{Level: 73}
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
			Position:       origin,
			Destination:    navigation.Position{Longitude: 10},
			Instant:        instant,
			StepDuration:   time.Hour,
			LayerCount:     2,
			HeadingSpacing: 90,
			CellSize:       1,
			InitialStamina: initialStamina,
		},
	)
	if err != nil {
		t.Fatalf("construction refusée: %v", err)
	}
	if search.InitialStamina != initialStamina {
		t.Errorf(
			"stamina initiale = %+v, attendue %+v",
			search.InitialStamina,
			initialStamina,
		)
	}

	for frontierIndex, frontier := range search.Frontiers {
		for nodeIndex, node := range frontier.Nodes {
			state := node.State
			step := node.Candidate.Step
			if err := state.Validate(); err != nil {
				t.Errorf(
					"front %d, nœud %d: état invalide: %v",
					frontierIndex+1,
					nodeIndex+1,
					err,
				)
			}
			if state.Position != step.NextPosition {
				t.Errorf(
					"front %d, nœud %d: position d'état incohérente",
					frontierIndex+1,
					nodeIndex+1,
				)
			}
			if !state.Instant.Equal(step.NextInstant) {
				t.Errorf(
					"front %d, nœud %d: instant d'état = %s, attendu %s",
					frontierIndex+1,
					nodeIndex+1,
					state.Instant,
					step.NextInstant,
				)
			}
			if state.Heading != step.Heading {
				t.Errorf(
					"front %d, nœud %d: cap d'état = %.1f°, attendu %.1f°",
					frontierIndex+1,
					nodeIndex+1,
					state.Heading,
					step.Heading,
				)
			}
			if state.SailID != step.BestSail.SailID {
				t.Errorf(
					"front %d, nœud %d: voile d'état = %d, attendue %d",
					frontierIndex+1,
					nodeIndex+1,
					state.SailID,
					step.BestSail.SailID,
				)
			}
			if state.Tack != step.Tack {
				t.Errorf(
					"front %d, nœud %d: amure d'état = %q, attendue %q",
					frontierIndex+1,
					nodeIndex+1,
					state.Tack,
					step.Tack,
				)
			}
			if state.Stamina != initialStamina {
				t.Errorf(
					"front %d, nœud %d: stamina = %+v, attendue %+v",
					frontierIndex+1,
					nodeIndex+1,
					state.Stamina,
					initialStamina,
				)
			}

			expectedPosition := origin
			expectedInstant := instant
			if frontierIndex > 0 {
				parent := search.Frontiers[frontierIndex-1].
					Nodes[node.ParentIndex]
				expectedPosition = parent.State.Position
				expectedInstant = parent.State.Instant
			}
			if step.Position != expectedPosition {
				t.Errorf(
					"front %d, nœud %d: position de départ non héritée",
					frontierIndex+1,
					nodeIndex+1,
				)
			}
			if !step.Instant.Equal(expectedInstant) {
				t.Errorf(
					"front %d, nœud %d: instant de départ non hérité",
					frontierIndex+1,
					nodeIndex+1,
				)
			}
		}
	}
}

func TestBuildIsochroneSearchRejectsInvalidInitialStamina(t *testing.T) {
	_, err := BuildIsochroneSearch(
		context.Background(),
		&staticWeatherProvider{
			wind: navigation.Wind{Speed: 5, Direction: 180},
		},
		testPolar(),
		IsochroneSearchInput{
			Position:       navigation.Position{},
			Destination:    navigation.Position{Longitude: 10},
			Instant:        time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
			StepDuration:   time.Hour,
			LayerCount:     1,
			HeadingSpacing: 90,
			CellSize:       1,
			InitialStamina: simulation.StaminaState{Level: 101},
		},
	)
	if err == nil || !strings.Contains(err.Error(), "stamina initiale") {
		t.Fatalf(
			"erreur = %v, doit signaler la stamina initiale invalide",
			err,
		)
	}
}

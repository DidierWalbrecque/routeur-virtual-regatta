package routing

import (
	"context"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
)

func TestBuildIsochroneSearchDetectsManeuversAfterFirstFrontier(
	t *testing.T,
) {
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
			Position:       navigation.Position{},
			Destination:    navigation.Position{Longitude: 10},
			Instant:        time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
			StepDuration:   time.Hour,
			LayerCount:     2,
			HeadingSpacing: 120,
			CellSize:       0.01,
			InitialStamina: simulation.StaminaState{Level: 100},
		},
	)
	if err != nil {
		t.Fatalf("construction refusée: %v", err)
	}
	if len(search.Frontiers) != 2 {
		t.Fatalf(
			"nombre de fronts = %d, attendu 2",
			len(search.Frontiers),
		)
	}

	for index, node := range search.Frontiers[0].Nodes {
		if node.Maneuver != nil {
			t.Errorf(
				"premier front, nœud %d: manœuvre inattendue %+v",
				index+1,
				*node.Maneuver,
			)
		}
	}

	activeCount := 0
	first := search.Frontiers[0]
	for index, node := range search.Frontiers[1].Nodes {
		if node.Maneuver == nil {
			t.Errorf(
				"second front, nœud %d: manœuvre non déterminée",
				index+1,
			)
			continue
		}
		if err := node.Maneuver.Validate(); err != nil {
			t.Errorf(
				"second front, nœud %d: manœuvre invalide: %v",
				index+1,
				err,
			)
		}
		parent := first.Nodes[node.ParentIndex]
		expected, err := simulation.DetectManeuver(
			simulation.ManeuverTransition{
				PreviousTack:      parent.State.Tack,
				NextTack:          node.State.Tack,
				PreviousWindAngle: parent.Candidate.Step.WindAngle,
				NextWindAngle:     node.Candidate.Step.WindAngle,
				PreviousSailID:    parent.State.SailID,
				NextSailID:        node.State.SailID,
			},
		)
		if err != nil {
			t.Fatalf(
				"second front, nœud %d: référence invalide: %v",
				index+1,
				err,
			)
		}
		if *node.Maneuver != expected {
			t.Errorf(
				"second front, nœud %d: manœuvre = %+v, attendue %+v",
				index+1,
				*node.Maneuver,
				expected,
			)
		}
		if node.Maneuver.Active() {
			activeCount++
		}
	}
	if activeCount == 0 {
		t.Fatal("aucune manœuvre active détectée au second front")
	}
}

func TestCandidateManeuverDetectsSailChange(t *testing.T) {
	maneuver, err := candidateManeuver(
		isochroneParentState{
			BoatState: simulation.BoatState{
				Tack:   navigation.TackStarboard,
				SailID: 1,
			},
			WindAngle:    60,
			HasBoatState: true,
		},
		HeadingCandidate{
			Step: simulation.StepResult{
				Tack:      navigation.TackStarboard,
				WindAngle: 70,
				BestSail: polar.SailPerformance{
					SailID: 2,
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("détection refusée: %v", err)
	}
	if maneuver == nil {
		t.Fatal("manœuvre absente")
	}
	if maneuver.Kind != simulation.ManeuverNone ||
		!maneuver.SailChanged {
		t.Errorf(
			"manœuvre = %+v, changement de voile attendu",
			*maneuver,
		)
	}
}

func TestCandidateManeuverLeavesAmbiguousTurnUndetermined(
	t *testing.T,
) {
	maneuver, err := candidateManeuver(
		isochroneParentState{
			BoatState: simulation.BoatState{
				Tack:   navigation.TackStarboard,
				SailID: 1,
			},
			WindAngle:    90,
			HasBoatState: true,
		},
		HeadingCandidate{
			Step: simulation.StepResult{
				Tack:      navigation.TackPort,
				WindAngle: 90,
				BestSail: polar.SailPerformance{
					SailID: 1,
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("détection refusée: %v", err)
	}
	if maneuver != nil {
		t.Errorf(
			"manœuvre = %+v, attendue indéterminée",
			*maneuver,
		)
	}
}

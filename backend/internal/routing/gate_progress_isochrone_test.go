package routing

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

func TestBuildIsochroneSearchRequiresGatesBeforeArrival(t *testing.T) {
	input := gateProgressSearchInput()
	input.Gates = []CourseGate{
		longitudeTestGate("porte lointaine", 1, GateLeftToRight),
	}

	search, err := BuildIsochroneSearch(
		context.Background(),
		gateProgressWeatherProvider(),
		testPolar(),
		input,
	)
	if err != nil {
		t.Fatalf("construction refusée: %v", err)
	}
	if search.Reached {
		t.Fatal("destination atteinte avant la porte obligatoire")
	}

	withinArrivalRadius := false
	for _, node := range search.Frontiers[0].Nodes {
		if node.RemainingDistance <= input.ArrivalRadius {
			withinArrivalRadius = true
		}
		if node.CourseProgress.NextGateIndex != 0 {
			t.Fatalf(
				"prochaine porte = %d, attendue 0",
				node.CourseProgress.NextGateIndex,
			)
		}
	}
	if !withinArrivalRadius {
		t.Fatal("le scénario ne contient aucun nœud dans le rayon d'arrivée")
	}
}

func TestBuildIsochroneSearchReachesAfterRequiredGate(t *testing.T) {
	input := gateProgressSearchInput()
	input.Gates = []CourseGate{
		longitudeTestGate("porte avant arrivée", 0.05, GateLeftToRight),
	}

	search, err := BuildIsochroneSearch(
		context.Background(),
		gateProgressWeatherProvider(),
		testPolar(),
		input,
	)
	if err != nil {
		t.Fatalf("construction refusée: %v", err)
	}
	if !search.Reached {
		t.Fatal("destination non atteinte après la porte obligatoire")
	}
	if len(search.Gates) != 1 ||
		search.Gates[0].Name != "porte avant arrivée" {
		t.Fatalf("portes conservées = %+v", search.Gates)
	}
	best, ok := search.Best()
	if !ok {
		t.Fatal("meilleur nœud absent")
	}
	if best.CourseProgress.NextGateIndex != 1 {
		t.Fatalf(
			"prochaine porte = %d, attendue 1",
			best.CourseProgress.NextGateIndex,
		)
	}
}

func TestPruneIsochroneNodesKeepsDistinctCourseProgress(t *testing.T) {
	beforeGate := stateAwarePruningNode(
		90,
		1,
		navigation.TackPort,
		100,
		10,
	)
	afterGate := stateAwarePruningNode(
		90,
		1,
		navigation.TackPort,
		90,
		9,
	)
	afterGate.CourseProgress = CourseProgress{NextGateIndex: 1}

	selected, err := pruneIsochroneNodes(
		navigation.Position{},
		[]IsochroneNode{beforeGate, afterGate},
		10,
	)
	if err != nil {
		t.Fatalf("élagage refusé: %v", err)
	}
	if len(selected) != 2 {
		t.Fatalf("nœuds conservés = %d, attendus 2", len(selected))
	}
	if selected[0].CourseProgress.NextGateIndex != 1 {
		t.Fatalf(
			"progression du premier nœud = %d, attendue 1",
			selected[0].CourseProgress.NextGateIndex,
		)
	}
}

func TestBuildIsochroneSearchRejectsInvalidGate(t *testing.T) {
	input := gateProgressSearchInput()
	input.Gates = []CourseGate{{
		Name: "porte sans largeur",
	}}

	_, err := BuildIsochroneSearch(
		context.Background(),
		gateProgressWeatherProvider(),
		testPolar(),
		input,
	)
	if err == nil || !strings.Contains(err.Error(), "parcours invalide") {
		t.Fatalf("erreur = %v, parcours invalide attendu", err)
	}
}

func gateProgressSearchInput() IsochroneSearchInput {
	return IsochroneSearchInput{
		Position:       navigation.Position{},
		Destination:    navigation.Position{Longitude: 0.13},
		Instant:        time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
		StepDuration:   time.Hour,
		LayerCount:     1,
		HeadingSpacing: 90,
		CellSize:       1,
		ArrivalRadius:  0.5,
	}
}

func gateProgressWeatherProvider() *staticWeatherProvider {
	return &staticWeatherProvider{
		wind: navigation.Wind{
			Speed:     5,
			Direction: 180,
		},
	}
}

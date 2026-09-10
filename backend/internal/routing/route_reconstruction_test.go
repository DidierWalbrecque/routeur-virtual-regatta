package routing

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

func TestIsochroneSearchReconstructBestRoute(t *testing.T) {
	instant := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)
	search := buildRouteReconstructionSearch(t, instant, 3)

	route, err := search.ReconstructBestRoute()
	if err != nil {
		t.Fatalf("reconstruction refusée: %v", err)
	}

	if route.Origin != search.Origin {
		t.Errorf("origine = %+v, attendue %+v", route.Origin, search.Origin)
	}
	if route.Destination != search.Destination {
		t.Errorf(
			"destination = %+v, attendue %+v",
			route.Destination,
			search.Destination,
		)
	}
	if !route.StartedAt.Equal(instant) {
		t.Errorf("départ = %s, attendu %s", route.StartedAt, instant)
	}
	if !route.FinishedAt.Equal(instant.Add(3 * time.Hour)) {
		t.Errorf(
			"arrivée = %s, attendue %s",
			route.FinishedAt,
			instant.Add(3*time.Hour),
		)
	}
	if len(route.Steps) != 3 {
		t.Fatalf("nombre de pas = %d, attendu 3", len(route.Steps))
	}

	for index, step := range route.Steps {
		if step.FrontierIndex != index {
			t.Errorf(
				"indice de front du pas %d = %d",
				index+1,
				step.FrontierIndex,
			)
		}
		if index == 0 {
			if step.Node.ParentIndex != -1 {
				t.Errorf(
					"parent du premier pas = %d, attendu -1",
					step.Node.ParentIndex,
				)
			}
			if step.Node.Candidate.Step.Position != search.Origin {
				t.Fatal("le premier pas ne part pas de l'origine")
			}
			continue
		}

		previous := route.Steps[index-1]
		if step.Node.ParentIndex != previous.NodeIndex {
			t.Errorf(
				"parent du pas %d = %d, attendu %d",
				index+1,
				step.Node.ParentIndex,
				previous.NodeIndex,
			)
		}
		if step.Node.Candidate.Step.Position !=
			previous.Node.Candidate.Step.NextPosition {
			t.Errorf(
				"la position du pas %d ne suit pas le pas précédent",
				index+1,
			)
		}
		if !step.Node.Candidate.Step.Instant.Equal(
			previous.Node.Candidate.Step.NextInstant,
		) {
			t.Errorf(
				"l'instant du pas %d ne suit pas le pas précédent",
				index+1,
			)
		}
	}

	best, ok := search.Best()
	if !ok {
		t.Fatal("meilleur nœud absent")
	}
	final := route.Steps[len(route.Steps)-1]
	if final.NodeIndex != 0 {
		t.Errorf(
			"indice du meilleur nœud final = %d, attendu 0",
			final.NodeIndex,
		)
	}
	if final.Node.Candidate.Step.NextPosition !=
		best.Candidate.Step.NextPosition {
		t.Fatal("la route ne termine pas sur le meilleur nœud")
	}

	positions := route.Positions()
	if len(positions) != 4 {
		t.Fatalf("nombre de positions = %d, attendu 4", len(positions))
	}
	if positions[0] != search.Origin {
		t.Fatal("la première position n'est pas l'origine")
	}
	for index, step := range route.Steps {
		if positions[index+1] != step.Node.Candidate.Step.NextPosition {
			t.Errorf(
				"position %d différente de la position du pas",
				index+2,
			)
		}
	}
	positions[0].Latitude = 1
	if route.Origin.Latitude != 0 {
		t.Fatal("Positions a exposé le stockage interne de la route")
	}
}

func TestIsochroneSearchReconstructBestRouteRejectsInvalidSearch(t *testing.T) {
	instant := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)
	valid := buildRouteReconstructionSearch(t, instant, 2)

	tests := []struct {
		name    string
		mutate  func(*IsochroneSearch)
		wantErr string
	}{
		{
			name: "aucun front",
			mutate: func(search *IsochroneSearch) {
				search.Frontiers = nil
			},
			wantErr: "aucun front",
		},
		{
			name: "front final vide",
			mutate: func(search *IsochroneSearch) {
				search.Frontiers[len(search.Frontiers)-1].Nodes = nil
			},
			wantErr: "front 2 vide",
		},
		{
			name: "indice de front incohérent",
			mutate: func(search *IsochroneSearch) {
				search.Frontiers[0].Index = 2
			},
			wantErr: "indice du front 1",
		},
		{
			name: "instant de front incohérent",
			mutate: func(search *IsochroneSearch) {
				search.Frontiers[0].Instant =
					search.Frontiers[0].Instant.Add(time.Minute)
			},
			wantErr: "instant du front 1",
		},
		{
			name: "parent absent",
			mutate: func(search *IsochroneSearch) {
				search.Frontiers[1].Nodes[0].ParentIndex = 999
			},
			wantErr: "parent 999 absent",
		},
		{
			name: "premier parent incohérent",
			mutate: func(search *IsochroneSearch) {
				for index := range search.Frontiers[0].Nodes {
					search.Frontiers[0].Nodes[index].ParentIndex = 0
				}
			},
			wantErr: "premier nœud",
		},
		{
			name: "liaison de position rompue",
			mutate: func(search *IsochroneSearch) {
				search.Frontiers[1].Nodes[0].Candidate.Step.Position =
					navigation.Position{Latitude: 1}
			},
			wantErr: "position initiale du front 2",
		},
		{
			name: "progression incohérente",
			mutate: func(search *IsochroneSearch) {
				search.Frontiers[1].Nodes[0].Progress++
			},
			wantErr: "progression du front 2",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			search := cloneIsochroneSearch(valid)
			test.mutate(&search)

			_, err := search.ReconstructBestRoute()
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %v, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

func TestIsochroneRoutePositionsWithNoStep(t *testing.T) {
	origin := navigation.Position{Latitude: 13, Longitude: 121}
	positions := (IsochroneRoute{Origin: origin}).Positions()
	if len(positions) != 1 || positions[0] != origin {
		t.Fatalf("positions = %+v, attendue uniquement l'origine", positions)
	}
}

func buildRouteReconstructionSearch(
	t *testing.T,
	instant time.Time,
	layerCount int,
) IsochroneSearch {
	t.Helper()

	search, err := BuildIsochroneSearch(
		context.Background(),
		&staticWeatherProvider{
			wind: navigation.Wind{
				Speed:     5,
				Direction: 180,
			},
		},
		testPolar(),
		IsochroneSearchInput{
			Position:       navigation.Position{},
			Destination:    navigation.Position{Longitude: 10},
			Instant:        instant,
			StepDuration:   time.Hour,
			LayerCount:     layerCount,
			HeadingSpacing: 90,
			CellSize:       1,
		},
	)
	if err != nil {
		t.Fatalf("préparation de la recherche impossible: %v", err)
	}
	return search
}

func cloneIsochroneSearch(source IsochroneSearch) IsochroneSearch {
	result := source
	result.Frontiers = append(
		[]IsochroneFrontier(nil),
		source.Frontiers...,
	)
	for index := range result.Frontiers {
		result.Frontiers[index].Nodes = append(
			[]IsochroneNode(nil),
			source.Frontiers[index].Nodes...,
		)
	}
	return result
}

package routing

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/weather"
)

func TestBuildIsochroneSearch(t *testing.T) {
	origin := navigation.Position{}
	destination := navigation.Position{Longitude: 10}
	instant := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)
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
			Destination:    destination,
			Instant:        instant,
			StepDuration:   time.Hour,
			LayerCount:     2,
			HeadingSpacing: 90,
			CellSize:       1,
		},
	)
	if err != nil {
		t.Fatalf("construction refusée: %v", err)
	}

	if search.Origin != origin {
		t.Errorf("origine = %+v, attendue %+v", search.Origin, origin)
	}
	if search.Destination != destination {
		t.Errorf(
			"destination = %+v, attendue %+v",
			search.Destination,
			destination,
		)
	}
	if !search.StartedAt.Equal(instant) {
		t.Errorf("instant initial = %s, attendu %s", search.StartedAt, instant)
	}
	if search.StepDuration != time.Hour {
		t.Errorf("durée de pas = %s, attendue 1h", search.StepDuration)
	}
	if len(search.Frontiers) != 2 {
		t.Fatalf(
			"nombre de fronts = %d, attendu 2",
			len(search.Frontiers),
		)
	}

	first := search.Frontiers[0]
	if first.Index != 1 {
		t.Errorf("indice du premier front = %d, attendu 1", first.Index)
	}
	if !first.Instant.Equal(instant.Add(time.Hour)) {
		t.Errorf(
			"instant du premier front = %s, attendu %s",
			first.Instant,
			instant.Add(time.Hour),
		)
	}
	if len(first.Nodes) != 4 {
		t.Fatalf(
			"positions du premier front = %d, attendues 4",
			len(first.Nodes),
		)
	}
	for index, node := range first.Nodes {
		if node.ParentIndex != -1 {
			t.Errorf(
				"parent du nœud %d du premier front = %d, attendu -1",
				index+1,
				node.ParentIndex,
			)
		}
	}

	second := search.Frontiers[1]
	if second.Index != 2 {
		t.Errorf("indice du second front = %d, attendu 2", second.Index)
	}
	if !second.Instant.Equal(instant.Add(2 * time.Hour)) {
		t.Errorf(
			"instant du second front = %s, attendu %s",
			second.Instant,
			instant.Add(2*time.Hour),
		)
	}
	if len(second.Nodes) == 0 || len(second.Nodes) > 16 {
		t.Fatalf(
			"positions du second front = %d, attendu entre 1 et 16",
			len(second.Nodes),
		)
	}
	for index, node := range second.Nodes {
		if node.ParentIndex < 0 || node.ParentIndex >= len(first.Nodes) {
			t.Errorf(
				"parent du nœud %d du second front = %d, hors du premier front",
				index+1,
				node.ParentIndex,
			)
		}
		if !node.Candidate.Step.Instant.Equal(first.Instant) {
			t.Errorf(
				"instant initial du nœud %d = %s, attendu %s",
				index+1,
				node.Candidate.Step.Instant,
				first.Instant,
			)
		}
		if node.InitialDistance != first.Nodes[0].InitialDistance {
			t.Errorf(
				"distance initiale du nœud %d incohérente",
				index+1,
			)
		}
		if index > 0 && second.Nodes[index-1].Progress < node.Progress {
			t.Fatal("le second front n'est pas classé par progression")
		}
	}

	wantCalls := 4 + len(first.Nodes)*4
	if provider.calls != wantCalls {
		t.Errorf(
			"appels météo = %d, attendus %d",
			provider.calls,
			wantCalls,
		)
	}

	best, ok := search.Best()
	if !ok {
		t.Fatal("meilleur nœud absent")
	}
	if best.Progress <= 0 {
		t.Errorf(
			"progression du meilleur nœud = %.6f NM, attendue positive",
			best.Progress,
		)
	}
	if best.Candidate.Step.NextInstant != second.Instant {
		t.Errorf(
			"instant final du meilleur nœud = %s, attendu %s",
			best.Candidate.Step.NextInstant,
			second.Instant,
		)
	}
}

func TestBuildIsochroneSearchPrunesEveryFrontier(t *testing.T) {
	provider := &staticWeatherProvider{
		wind: navigation.Wind{
			Speed:     5,
			Direction: 180,
		},
	}
	search, err := BuildIsochroneSearch(
		context.Background(),
		provider,
		zeroSpeedTestPolar(),
		IsochroneSearchInput{
			Position:       navigation.Position{},
			Destination:    navigation.Position{Longitude: 10},
			Instant:        time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
			StepDuration:   10 * time.Minute,
			LayerCount:     3,
			HeadingSpacing: 90,
			CellSize:       1,
		},
	)
	if err != nil {
		t.Fatalf("construction refusée: %v", err)
	}

	if len(search.Frontiers) != 3 {
		t.Fatalf(
			"nombre de fronts = %d, attendu 3",
			len(search.Frontiers),
		)
	}
	for index, frontier := range search.Frontiers {
		if len(frontier.Nodes) != 1 {
			t.Fatalf(
				"positions du front %d = %d, attendue 1",
				index+1,
				len(frontier.Nodes),
			)
		}
		wantParent := 0
		if index == 0 {
			wantParent = -1
		}
		if frontier.Nodes[0].ParentIndex != wantParent {
			t.Errorf(
				"parent du front %d = %d, attendu %d",
				index+1,
				frontier.Nodes[0].ParentIndex,
				wantParent,
			)
		}
		if frontier.Nodes[0].Candidate.Heading != 0 {
			t.Errorf(
				"cap conservé au front %d = %.1f°, attendu 0°",
				index+1,
				frontier.Nodes[0].Candidate.Heading,
			)
		}
	}
	if provider.calls != 12 {
		t.Errorf("appels météo = %d, attendus 12", provider.calls)
	}
}

func TestBuildIsochroneSearchLimitsFrontierSizeAdaptively(t *testing.T) {
	provider := &staticWeatherProvider{
		wind: navigation.Wind{
			Speed:     5,
			Direction: 180,
		},
	}
	const (
		layerCount    = 6
		maxCandidates = 8
	)
	search, err := BuildIsochroneSearch(
		context.Background(),
		provider,
		testPolar(),
		IsochroneSearchInput{
			Position:       navigation.Position{},
			Destination:    navigation.Position{Longitude: 10},
			Instant:        time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
			StepDuration:   time.Hour,
			LayerCount:     layerCount,
			HeadingSpacing: 15,
			CellSize:       1,
			MaxCandidates:  maxCandidates,
		},
	)
	if err != nil {
		t.Fatalf("construction refusée: %v", err)
	}
	if search.MaxCandidates != maxCandidates {
		t.Errorf(
			"limite = %d, attendue %d",
			search.MaxCandidates,
			maxCandidates,
		)
	}

	cellSizeIncreased := false
	for index, frontier := range search.Frontiers {
		if len(frontier.Nodes) > maxCandidates {
			t.Errorf(
				"front %d : %d positions, limite %d",
				index+1,
				len(frontier.Nodes),
				maxCandidates,
			)
		}
		if frontier.CellSize < 1 {
			t.Errorf(
				"front %d : cellule %.3f NM inférieure au minimum",
				index+1,
				frontier.CellSize,
			)
		}
		if frontier.CellSize > 1 {
			cellSizeIncreased = true
		}
	}
	if !cellSizeIncreased {
		t.Fatal("la taille des cellules n'a jamais été augmentée")
	}

	maximumCalls := 24 + (layerCount-1)*maxCandidates*24
	if provider.calls > maximumCalls {
		t.Errorf(
			"appels météo = %d, maximum attendu %d",
			provider.calls,
			maximumCalls,
		)
	}
}

func TestBuildIsochroneSearchStopsAtArrivalRadius(t *testing.T) {
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
			Destination:    navigation.Position{Longitude: 0.13},
			Instant:        time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
			StepDuration:   time.Hour,
			LayerCount:     5,
			HeadingSpacing: 90,
			CellSize:       1,
			ArrivalRadius:  0.5,
		},
	)
	if err != nil {
		t.Fatalf("construction refusée: %v", err)
	}
	if !search.Reached {
		t.Fatal("la destination doit être déclarée atteinte")
	}
	if search.ArrivalRadius != 0.5 {
		t.Errorf(
			"rayon d'arrivée = %.3f NM, attendu 0.5",
			search.ArrivalRadius,
		)
	}
	if len(search.Frontiers) != 1 {
		t.Fatalf(
			"nombre de fronts = %d, attendu 1",
			len(search.Frontiers),
		)
	}
	best, ok := search.Best()
	if !ok {
		t.Fatal("meilleur nœud absent")
	}
	if best.RemainingDistance > search.ArrivalRadius {
		t.Errorf(
			"distance restante = %.6f NM, rayon %.6f NM",
			best.RemainingDistance,
			search.ArrivalRadius,
		)
	}
	if provider.calls != 4 {
		t.Errorf("appels météo = %d, attendus 4", provider.calls)
	}
}

func TestIsochroneSearchBestRejectsEmptySearch(t *testing.T) {
	if _, ok := (IsochroneSearch{}).Best(); ok {
		t.Fatal("une recherche vide ne doit pas avoir de meilleur nœud")
	}
	search := IsochroneSearch{
		Frontiers: []IsochroneFrontier{{Index: 1}},
	}
	if _, ok := search.Best(); ok {
		t.Fatal("un front final vide ne doit pas avoir de meilleur nœud")
	}
}

func TestBuildIsochroneSearchRejectsInvalidInputs(t *testing.T) {
	validInput := IsochroneSearchInput{
		Position:       navigation.Position{},
		Destination:    navigation.Position{Longitude: 10},
		Instant:        time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
		StepDuration:   10 * time.Minute,
		LayerCount:     2,
		HeadingSpacing: 90,
		CellSize:       1,
	}
	validProvider := func() weather.WeatherProvider {
		return &staticWeatherProvider{
			wind: navigation.Wind{
				Speed:     5,
				Direction: 180,
			},
		}
	}

	tests := []struct {
		name      string
		provider  weather.WeatherProvider
		boatPolar *polar.Polar
		input     IsochroneSearchInput
		wantErr   string
	}{
		{
			name:      "fournisseur absent",
			provider:  nil,
			boatPolar: testPolar(),
			input:     validInput,
			wantErr:   "fournisseur météo",
		},
		{
			name:      "polaire absente",
			provider:  validProvider(),
			boatPolar: nil,
			input:     validInput,
			wantErr:   "polaire requise",
		},
		{
			name:      "nombre de couches nul",
			provider:  validProvider(),
			boatPolar: testPolar(),
			input: func() IsochroneSearchInput {
				input := validInput
				input.LayerCount = 0
				return input
			}(),
			wantErr: "nombre de couches",
		},
		{
			name:      "taille de cellule nulle",
			provider:  validProvider(),
			boatPolar: testPolar(),
			input: func() IsochroneSearchInput {
				input := validInput
				input.CellSize = 0
				return input
			}(),
			wantErr: "taille de cellule",
		},
		{
			name:      "taille de cellule non finie",
			provider:  validProvider(),
			boatPolar: testPolar(),
			input: func() IsochroneSearchInput {
				input := validInput
				input.CellSize = navigation.NauticalMiles(math.NaN())
				return input
			}(),
			wantErr: "taille de cellule",
		},
		{
			name:      "rayon d'arrivée négatif",
			provider:  validProvider(),
			boatPolar: testPolar(),
			input: func() IsochroneSearchInput {
				input := validInput
				input.ArrivalRadius = -1
				return input
			}(),
			wantErr: "rayon d'arrivée",
		},
		{
			name:      "rayon d'arrivée non fini",
			provider:  validProvider(),
			boatPolar: testPolar(),
			input: func() IsochroneSearchInput {
				input := validInput
				input.ArrivalRadius = navigation.NauticalMiles(math.NaN())
				return input
			}(),
			wantErr: "rayon d'arrivée",
		},
		{
			name:      "limite de candidats trop petite",
			provider:  validProvider(),
			boatPolar: testPolar(),
			input: func() IsochroneSearchInput {
				input := validInput
				input.MaxCandidates = 3
				return input
			}(),
			wantErr: "au moins égale à 4",
		},
		{
			name:      "grille de caps invalide",
			provider:  validProvider(),
			boatPolar: testPolar(),
			input: func() IsochroneSearchInput {
				input := validInput
				input.HeadingSpacing = 7
				return input
			}(),
			wantErr: "grille de caps",
		},
		{
			name:      "destination invalide",
			provider:  validProvider(),
			boatPolar: testPolar(),
			input: func() IsochroneSearchInput {
				input := validInput
				input.Destination = navigation.Position{Latitude: 91}
				return input
			}(),
			wantErr: "destination invalide",
		},
		{
			name: "météo indisponible",
			provider: &staticWeatherProvider{
				err: errors.New("prévision absente"),
			},
			boatPolar: testPolar(),
			input:     validInput,
			wantErr:   "développement du parent 1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := BuildIsochroneSearch(
				context.Background(),
				test.provider,
				test.boatPolar,
				test.input,
			)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %v, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

func zeroSpeedTestPolar() *polar.Polar {
	boatPolar := testPolar()
	for sailIndex := range boatPolar.Sails {
		for angleIndex := range boatPolar.Sails[sailIndex].Speeds.Values {
			for windIndex := range boatPolar.Sails[sailIndex].Speeds.Values[angleIndex] {
				boatPolar.Sails[sailIndex].Speeds.Values[angleIndex][windIndex] = 0
			}
		}
	}
	return boatPolar
}

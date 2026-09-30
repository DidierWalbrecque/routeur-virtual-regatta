package routing

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

func TestBuildIsochroneSearchReportsCompletedFrontiers(t *testing.T) {
	instant := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)
	provider := &staticWeatherProvider{
		wind: navigation.Wind{
			Speed:     5,
			Direction: 180,
		},
	}
	var reports []IsochroneProgress

	search, err := BuildIsochroneSearch(
		context.Background(),
		provider,
		testPolar(),
		IsochroneSearchInput{
			Position:       navigation.Position{},
			Destination:    navigation.Position{Longitude: 10},
			Instant:        instant,
			StepDuration:   time.Hour,
			LayerCount:     3,
			HeadingSpacing: 90,
			CellSize:       1,
			Progress: func(progress IsochroneProgress) {
				reports = append(reports, progress)
			},
		},
	)
	if err != nil {
		t.Fatalf("construction refusée: %v", err)
	}
	if len(reports) != len(search.Frontiers) {
		t.Fatalf(
			"rapports = %d, fronts = %d",
			len(reports),
			len(search.Frontiers),
		)
	}

	for index, report := range reports {
		frontier := search.Frontiers[index]
		if report.CompletedLayers != index+1 {
			t.Errorf(
				"rapport %d : couches terminées = %d, attendues %d",
				index+1,
				report.CompletedLayers,
				index+1,
			)
		}
		if report.TotalLayers != 3 {
			t.Errorf(
				"rapport %d : total = %d, attendu 3",
				index+1,
				report.TotalLayers,
			)
		}
		if !report.Instant.Equal(frontier.Instant) {
			t.Errorf(
				"rapport %d : instant = %s, attendu %s",
				index+1,
				report.Instant,
				frontier.Instant,
			)
		}
		if report.CandidateCount != len(frontier.Nodes) {
			t.Errorf(
				"rapport %d : candidats = %d, attendus %d",
				index+1,
				report.CandidateCount,
				len(frontier.Nodes),
			)
		}
		if report.CellSize != frontier.CellSize {
			t.Errorf(
				"rapport %d : cellule = %.3f, attendue %.3f",
				index+1,
				report.CellSize,
				frontier.CellSize,
			)
		}
		if report.Frontier.Index != frontier.Index || len(report.Frontier.Nodes) != len(frontier.Nodes) {
			t.Errorf(
				"rapport %d : frontier = index %d avec %d nœud(s), attendu index %d avec %d",
				index+1,
				report.Frontier.Index,
				len(report.Frontier.Nodes),
				frontier.Index,
				len(frontier.Nodes),
			)
		}
		if report.Reached {
			t.Errorf(
				"rapport %d : destination déclarée atteinte",
				index+1,
			)
		}
	}
}

func TestBuildIsochroneSearchReportsArrival(t *testing.T) {
	var reports []IsochroneProgress
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
			Destination:    navigation.Position{Longitude: 0.13},
			Instant:        time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
			StepDuration:   time.Hour,
			LayerCount:     5,
			HeadingSpacing: 90,
			CellSize:       1,
			ArrivalRadius:  0.5,
			Progress: func(progress IsochroneProgress) {
				reports = append(reports, progress)
			},
		},
	)
	if err != nil {
		t.Fatalf("construction refusée: %v", err)
	}
	if !search.Reached {
		t.Fatal("destination non atteinte")
	}
	if len(reports) != 1 {
		t.Fatalf("rapports = %d, attendu 1", len(reports))
	}
	if !reports[0].Reached {
		t.Fatal("le rapport final ne signale pas l'arrivée")
	}
}

func TestBuildIsochroneSearchCanBeCanceledFromProgress(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	provider := &staticWeatherProvider{
		wind: navigation.Wind{
			Speed:     5,
			Direction: 180,
		},
	}
	reportCount := 0
	search, err := BuildIsochroneSearch(
		ctx,
		provider,
		zeroSpeedTestPolar(),
		IsochroneSearchInput{
			Position:       navigation.Position{},
			Destination:    navigation.Position{Longitude: 10},
			Instant:        time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
			StepDuration:   time.Hour,
			LayerCount:     3,
			HeadingSpacing: 90,
			CellSize:       1,
			Progress: func(IsochroneProgress) {
				reportCount++
				cancel()
			},
		},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("erreur = %v, context.Canceled attendu", err)
	}
	if reportCount != 1 {
		t.Errorf("rapports = %d, attendu 1", reportCount)
	}
	if len(search.Frontiers) != 1 {
		t.Fatalf(
			"fronts partiels = %d, attendu 1",
			len(search.Frontiers),
		)
	}
	if provider.calls != 1 {
		t.Errorf("appels météo = %d, attendus 1", provider.calls)
	}
}

func TestBuildIsochroneSearchRejectsCanceledOrMissingContext(t *testing.T) {
	validInput := IsochroneSearchInput{
		Position:       navigation.Position{},
		Destination:    navigation.Position{Longitude: 10},
		Instant:        time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
		StepDuration:   time.Hour,
		LayerCount:     1,
		HeadingSpacing: 90,
		CellSize:       1,
	}
	newProvider := func() *staticWeatherProvider {
		return &staticWeatherProvider{
			wind: navigation.Wind{
				Speed:     5,
				Direction: 180,
			},
		}
	}

	t.Run("contexte déjà annulé", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		provider := newProvider()

		search, err := BuildIsochroneSearch(
			ctx,
			provider,
			testPolar(),
			validInput,
		)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("erreur = %v, context.Canceled attendu", err)
		}
		if len(search.Frontiers) != 0 {
			t.Errorf("fronts = %d, attendu 0", len(search.Frontiers))
		}
		if provider.calls != 0 {
			t.Errorf("appels météo = %d, attendu 0", provider.calls)
		}
	})

	t.Run("contexte absent", func(t *testing.T) {
		_, err := BuildIsochroneSearch(
			nil,
			newProvider(),
			testPolar(),
			validInput,
		)
		if err == nil || !strings.Contains(err.Error(), "contexte requis") {
			t.Fatalf("erreur = %v, doit contenir %q", err, "contexte requis")
		}
	})
}

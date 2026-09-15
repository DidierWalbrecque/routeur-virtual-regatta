package main

import (
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/routing"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
)

func TestWriteResultIncludesAppliedSpeedPenalty(t *testing.T) {
	instant := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)
	position := navigation.Position{Latitude: 13, Longitude: 121}
	destination := navigation.Position{Latitude: 14, Longitude: 120}
	node := routing.IsochroneNode{
		ParentIndex: -1,
		Candidate: routing.HeadingCandidate{
			Heading: 321,
			Step: simulation.StepResult{
				Position:     position,
				Instant:      instant,
				Heading:      321,
				Duration:     10 * time.Minute,
				Distance:     1,
				Wind:         navigation.Wind{Speed: 4, Direction: 343},
				WindKnots:    7.77538,
				WindAngle:    22,
				Tack:         navigation.TackStarboard,
				BestSail:     polar.SailPerformance{SailName: "Jib", Speed: 8},
				NextPosition: navigation.Position{Latitude: 13.01, Longitude: 120.99},
				NextInstant:  instant.Add(10 * time.Minute),
			},
		},
		InitialDistance:   84,
		RemainingDistance: 83,
		Progress:          1,
		SpeedPenalty: &simulation.SpeedPenaltyApplication{
			PenalizedDuration: 10 * time.Minute,
			AppliedPenalty: &polar.ManeuverPenalty{
				SpeedRatio: 0.5,
				Duration:   15 * time.Minute,
			},
			RemainingPenalty: &polar.ManeuverPenalty{
				SpeedRatio: 0.5,
				Duration:   5 * time.Minute,
			},
		},
		State: simulation.BoatState{
			Stamina: simulation.StaminaState{Level: 72.5},
		},
	}
	search := routing.IsochroneSearch{
		Origin:       position,
		Destination:  destination,
		StartedAt:    instant,
		StepDuration: 10 * time.Minute,
		Frontiers: []routing.IsochroneFrontier{
			{
				Index:    1,
				Instant:  instant.Add(10 * time.Minute),
				CellSize: 1,
				Nodes:    []routing.IsochroneNode{node},
			},
		},
	}
	route := routing.IsochroneRoute{
		Origin:      position,
		Destination: destination,
		StartedAt:   instant,
		FinishedAt:  instant.Add(10 * time.Minute),
		Steps: []routing.IsochroneRouteStep{
			{
				FrontierIndex: 0,
				NodeIndex:     0,
				Node:          node,
			},
		},
	}

	var output strings.Builder
	err := writeResult(
		&output,
		&polar.Polar{Label: "Test boat"},
		options{
			position:       position,
			destination:    destination,
			instant:        instant,
			totalDuration:  10 * time.Minute,
			stepDuration:   10 * time.Minute,
			layerCount:     1,
			headingSpacing: 10,
			cellSize:       1,
			maxCandidates:  500,
			arrivalRadius:  1,
		},
		search,
		route,
	)
	if err != nil {
		t.Fatalf("écriture refusée: %v", err)
	}

	expected := "pénalité vitesse 0.500 | appliquée 10m0s | reste 5m0s | vitesse moyenne 6.000 nd"
	if !strings.Contains(output.String(), expected) {
		t.Fatalf("sortie absente %q:\n%s", expected, output.String())
	}
}

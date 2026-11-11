package httpapi

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/equipment"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/routing"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
)

// Mesure ce que l'élagage du front laisse sur des routes complètes : milles
// restants après 48 heures, taille moyenne des fronts et durée du calcul.
// Ce calcul est long et dépend des prévisions locales : il ne tourne que sur
// demande.
//
//	VR_PRUNING_COMPARISON=quick go test ./internal/httpapi/ -run TestVRFrontierPruningRoutes -v -timeout 120m
func TestVRFrontierPruningRoutes(t *testing.T) {
	mode := os.Getenv("VR_PRUNING_COMPARISON")
	if mode == "" {
		t.Skip("définir VR_PRUNING_COMPARISON=quick ou precise pour lancer cette mesure")
	}
	profile, err := calculationProfileFor(mode)
	if err != nil {
		t.Fatal(err)
	}
	boatPolar := loadCombinationPolar(t)
	provider, instant := comparisonWeather(t)
	evaluator, combine := staminaManeuverRuntime(boatPolar)
	recovery := staminaRecoveryRules()
	owned, err := equipment.NewSet(equipment.All()...)
	if err != nil {
		t.Fatal(err)
	}

	layers := int(comparisonHorizon / profile.IsochroneStep)
	for _, course := range comparisonCourses {
		t.Run(course.name, func(t *testing.T) {
			started := time.Now()
			search, err := routing.BuildIsochroneSearch(context.Background(), provider, boatPolar, routing.IsochroneSearchInput{
				Position: course.origin, Destination: course.dest, Instant: instant,
				StepDuration: profile.IsochroneStep, LayerCount: layers, Owned: owned,
				HeadingSpacing: profile.HeadingSpacing, RefineHeadings: true,
				CellSize: profile.CellSize, MaxCandidates: profile.MaxCandidates,
				ArrivalRadius:         1,
				InitialStamina:        simulation.StaminaState{Level: 100},
				TransitionEvaluator:   evaluator,
				CombineSpeedPenalties: combine,
				StaminaRecoveryRules:  &recovery,
			})
			if err != nil {
				t.Fatalf("calcul impossible: %v", err)
			}
			tally := tallyRoute(t, search, time.Since(started))
			total := 0
			for _, frontier := range search.Frontiers {
				total += len(frontier.Nodes)
			}
			t.Logf("restant %8.2f NM | front moyen %5d / %d, dernier %5d | virements/empannages %2d, voiles %2d | stamina %5.1f | %v",
				tally.remaining, total/len(search.Frontiers), profile.MaxCandidates,
				len(search.Frontiers[len(search.Frontiers)-1].Nodes),
				tally.maneuvers, tally.sails, tally.finalStamina, tally.elapsed.Round(time.Second))
		})
	}
}

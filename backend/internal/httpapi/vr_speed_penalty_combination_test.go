package httpapi

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
)

// Ce test chiffre la règle actuelle de cumul des pénalités de vitesse
// (combineVRSpeedPenalties : ratios multipliés, durées additionnées) avec les
// valeurs réelles de l'Ultim BP XI. Il décrit le comportement actuel sans
// affirmer que c'est la règle du jeu : les autres cumuls servent seulement de
// comparaison, et ce test devra être adapté si la règle change.

const combinationBaseSpeedKnots = 20.0

type penaltySegment struct {
	ratio    float64
	duration time.Duration
}

func lostMiles(segments ...penaltySegment) float64 {
	lost := 0.0
	for _, segment := range segments {
		lost += (1 - segment.ratio) * combinationBaseSpeedKnots * segment.duration.Hours()
	}
	return lost
}

// overlappedSegments applique les deux pénalités en même temps : le produit
// des ratios pendant le chevauchement, puis seule la plus longue.
func overlappedSegments(first, second polar.ManeuverPenalty) []penaltySegment {
	shorter, longer := first, second
	if shorter.Duration > longer.Duration {
		shorter, longer = longer, shorter
	}
	return []penaltySegment{
		{float64(first.SpeedRatio) * float64(second.SpeedRatio), shorter.Duration},
		{float64(longer.SpeedRatio), longer.Duration - shorter.Duration},
	}
}

func loadCombinationPolar(t *testing.T) *polar.Polar {
	t.Helper()
	boatPolar, err := polar.LoadFile(
		filepath.Join("..", "..", "..", "data", "polars", "ultim_bp_11.json"),
	)
	if err != nil {
		t.Fatalf("chargement de la polaire impossible: %v", err)
	}
	return boatPolar
}

func tackWithSailChange(
	t *testing.T,
	evaluator *simulation.TransitionEvaluator,
	stamina simulation.StaminaLevel,
	professional bool,
) simulation.TransitionEvaluation {
	t.Helper()
	evaluation, err := evaluator.Evaluate(simulation.TransitionEvaluationRequest{
		Transition: simulation.ManeuverTransition{
			PreviousTack:      navigation.TackPort,
			NextTack:          navigation.TackStarboard,
			PreviousWindAngle: 50,
			NextWindAngle:     50,
			PreviousSailID:    1,
			NextSailID:        5,
		},
		WindSpeed:           20,
		Stamina:             simulation.StaminaState{Level: stamina},
		ProfessionalWinches: professional,
	})
	if err != nil {
		t.Fatalf("évaluation de la manœuvre impossible: %v", err)
	}
	return evaluation
}

func TestVRSimultaneousSpeedPenaltiesCost(t *testing.T) {
	evaluator, combine := staminaManeuverRuntime(loadCombinationPolar(t))

	cases := []struct {
		name         string
		stamina      simulation.StaminaLevel
		professional bool
	}{
		{"winchs standard, stamina 100", 100, false},
		{"winchs standard, stamina 50", 50, false},
		{"winchs standard, stamina 0", 0, false},
		{"winchs pro, stamina 100", 100, true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			evaluation := tackWithSailChange(t, evaluator, testCase.stamina, testCase.professional)
			tack, sail := *evaluation.NauticalPenalty, *evaluation.SailChangePenalty

			combined, err := evaluation.CombinedSpeedPenalty(combine)
			if err != nil {
				t.Fatalf("cumul impossible: %v", err)
			}
			if combined.Duration != tack.Duration+sail.Duration {
				t.Errorf("durée cumulée = %v, attendu %v", combined.Duration, tack.Duration+sail.Duration)
			}
			if want := float64(tack.SpeedRatio) * float64(sail.SpeedRatio); math.Abs(float64(combined.SpeedRatio)-want) > 1e-9 {
				t.Errorf("ratio cumulé = %v, attendu %v", combined.SpeedRatio, want)
			}

			// La perte calculée ici doit être celle que le moteur applique au pas.
			step := 2 * time.Hour
			application, err := simulation.ApplyManeuverSpeedPenalty(combinationBaseSpeedKnots, step, combined)
			if err != nil {
				t.Fatalf("application impossible: %v", err)
			}
			current := combinationBaseSpeedKnots*step.Hours() - float64(application.Distance)
			if want := lostMiles(penaltySegment{float64(combined.SpeedRatio), combined.Duration}); math.Abs(current-want) > 1e-9 {
				t.Errorf("perte du moteur = %.6f, attendu %.6f", current, want)
			}

			sequential := lostMiles(
				penaltySegment{float64(tack.SpeedRatio), tack.Duration},
				penaltySegment{float64(sail.SpeedRatio), sail.Duration},
			)
			overlapped := lostMiles(overlappedSegments(tack, sail)...)
			if current <= sequential || current <= overlapped {
				t.Errorf("le cumul actuel (%.3f) devrait coûter plus que à la suite (%.3f) et simultané (%.3f)",
					current, sequential, overlapped)
			}
			t.Logf("virement %v ×%.2f, voile %v ×%.2f, cumul %v ×%.2f",
				tack.Duration, float64(tack.SpeedRatio), sail.Duration, float64(sail.SpeedRatio),
				combined.Duration, float64(combined.SpeedRatio))
			t.Logf("milles perdus à %.0f nd : actuel %.3f | à la suite %.3f | simultané %.3f",
				combinationBaseSpeedKnots, current, sequential, overlapped)
		})
	}
}

// Valeurs fixes : tack 240 s et voile 390 s à 20 nd, divisées par 2 à stamina 100.
func TestVRSimultaneousSpeedPenaltiesReferenceValues(t *testing.T) {
	evaluator, combine := staminaManeuverRuntime(loadCombinationPolar(t))
	evaluation := tackWithSailChange(t, evaluator, 100, false)

	combined, err := evaluation.CombinedSpeedPenalty(combine)
	if err != nil {
		t.Fatalf("cumul impossible: %v", err)
	}
	if evaluation.NauticalPenalty.Duration != 120*time.Second {
		t.Errorf("durée du virement = %v, attendu 2m0s", evaluation.NauticalPenalty.Duration)
	}
	if evaluation.SailChangePenalty.Duration != 195*time.Second {
		t.Errorf("durée du changement de voile = %v, attendu 3m15s", evaluation.SailChangePenalty.Duration)
	}
	if combined.Duration != 315*time.Second || math.Abs(float64(combined.SpeedRatio)-0.25) > 1e-9 {
		t.Errorf("cumul = %v ×%v, attendu 5m15s ×0.25", combined.Duration, combined.SpeedRatio)
	}
	if got := lostMiles(penaltySegment{float64(combined.SpeedRatio), combined.Duration}); math.Abs(got-1.3125) > 1e-9 {
		t.Errorf("milles perdus = %.4f, attendu 1.3125", got)
	}
}

// Une nouvelle manœuvre qui chevauche le reste d'une ancienne passe par la même
// règle : le produit des ratios s'étend alors sur toute la durée additionnée.
func TestVROverlappingSpeedPenaltiesCost(t *testing.T) {
	_, combine := staminaManeuverRuntime(loadCombinationPolar(t))
	remaining := polar.ManeuverPenalty{SpeedRatio: 0.5, Duration: time.Minute}
	triggered := polar.ManeuverPenalty{SpeedRatio: 0.5, Duration: 2 * time.Minute}

	combined, err := simulation.CombineSpeedPenalties(&remaining, &triggered, combine)
	if err != nil {
		t.Fatalf("superposition impossible: %v", err)
	}
	if combined.Duration != 3*time.Minute || math.Abs(float64(combined.SpeedRatio)-0.25) > 1e-9 {
		t.Errorf("superposition = %v ×%v, attendu 3m0s ×0.25", combined.Duration, combined.SpeedRatio)
	}
	current := lostMiles(penaltySegment{float64(combined.SpeedRatio), combined.Duration})
	overlapped := lostMiles(overlappedSegments(remaining, triggered)...)
	if current <= overlapped {
		t.Errorf("la superposition actuelle (%.3f) devrait coûter plus que le chevauchement réel (%.3f)", current, overlapped)
	}
	t.Logf("milles perdus à %.0f nd : actuel %.3f | chevauchement réel %.3f",
		combinationBaseSpeedKnots, current, overlapped)
}

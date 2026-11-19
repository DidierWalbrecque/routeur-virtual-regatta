package httpapi

import (
	"math"
	"testing"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
)

func TestBoatStaminaCoefficient(t *testing.T) {
	if got := boatStaminaCoefficient(21); got != 1.5 {
		t.Errorf("Ultim BP XI (21) = %.2f, attendu 1.5", got)
	}
	if got := boatStaminaCoefficient(3); got != 1 {
		t.Errorf("Class 40 (3) = %.2f, attendu 1 (défaut)", got)
	}
	if got := boatStaminaCoefficient(999); got != 1 {
		t.Errorf("bateau inconnu = %.2f, attendu 1", got)
	}
}

// Coûts relevés dans l'audit de l'interface (Ultim BP XI, novembre 2026) : un
// changement de voile par 9,7 nœuds coûte 35,8 points, un empannage par
// 9,4 nœuds environ 17 points. Ils découlent du modèle VR/ITYC : coût de base
// (10 virement ou empannage, 20 changement de voile) × coefficient du bateau
// (1,5) × facteur de vent (1 à 0 nœud, 1,2 à 10 nœuds).
func TestUltimManeuverCostsMatchTheAuditedValues(t *testing.T) {
	evaluator, _ := staminaManeuverRuntime(&polar.Polar{BoatID: 21})
	cost := func(transition simulation.ManeuverTransition, wind float64) float64 {
		t.Helper()
		maneuver, err := simulation.DetectManeuver(transition)
		if err != nil {
			t.Fatal(err)
		}
		base, err := evaluator.ConsumptionRules.ConsumptionFor(maneuver)
		if err != nil {
			t.Fatal(err)
		}
		adjusted, err := base.AdjustedForWind(polar.WindSpeed(wind), evaluator.WindCurve)
		if err != nil {
			t.Fatal(err)
		}
		total, err := evaluator.CombineConsumption(adjusted)
		if err != nil {
			t.Fatal(err)
		}
		return float64(total)
	}

	sailChange := cost(simulation.ManeuverTransition{
		PreviousTack: navigation.TackPort, NextTack: navigation.TackPort,
		PreviousWindAngle: 110, NextWindAngle: 113,
		PreviousSailID: 1, NextSailID: 5,
	}, 9.7)
	if math.Abs(sailChange-35.82) > 0.05 {
		t.Errorf("changement de voile à 9,7 nœuds = %.2f points, attendu 35,8", sailChange)
	}

	gybe := cost(simulation.ManeuverTransition{
		PreviousTack: navigation.TackPort, NextTack: navigation.TackStarboard,
		PreviousWindAngle: 140, NextWindAngle: 140,
		PreviousSailID: 5, NextSailID: 5,
	}, 9.4)
	if math.Abs(gybe-17.82) > 0.05 {
		t.Errorf("empannage à 9,4 nœuds = %.2f points, attendu 17,8", gybe)
	}
}

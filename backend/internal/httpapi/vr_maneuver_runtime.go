package httpapi

import (
	"fmt"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
)

// staminaManeuverRuntime enables the stamina bookkeeping and the maneuver
// speed/duration penalties for every routed leg, independently of whether the
// position, stamina or gates come from Virtual Regatta. It uses the Virtual
// Regatta stamina model (ITYC reference values): per-boat consumption, a
// wind-dependent consumption curve, and a stamina-dependent maneuver-duration
// curve (faster when fresh, much slower when depleted).
func staminaManeuverRuntime(boatPolar *polar.Polar) (*simulation.TransitionEvaluator, simulation.ManeuverSpeedPenaltyCombiner) {
	coefficient := boatStaminaCoefficient(boatPolar.BoatID)
	base := simulation.DefaultStaminaConsumptionRules()
	evaluator := &simulation.TransitionEvaluator{
		WinchProfile: boatPolar.Winch,
		ConsumptionRules: simulation.StaminaConsumptionRules{
			Tack:       simulation.StaminaLevel(float64(base.Tack) * coefficient),
			Gybe:       simulation.StaminaLevel(float64(base.Gybe) * coefficient),
			SailChange: simulation.StaminaLevel(float64(base.SailChange) * coefficient),
		},
		// Consommation d'énergie selon le vent (modèle VR/ITYC).
		WindCurve: simulation.StaminaWindCurve{
			Points: []simulation.StaminaWindPoint{
				{WindSpeed: 0, Factor: 1},
				{WindSpeed: 10, Factor: 1.2},
				{WindSpeed: 20, Factor: 1.5},
				{WindSpeed: 30, Factor: 2},
			},
		},
		// Impact de la stamina sur la durée des manœuvres : ×0.5 à 100, ×2 à 0.
		DurationCurve: simulation.StaminaDurationCurve{
			Points: []simulation.StaminaDurationPoint{
				{Stamina: 0, Factor: 2},
				{Stamina: 100, Factor: 0.5},
			},
		},
		CombineConsumption: combineVRStaminaConsumption,
	}
	return evaluator, combineVRSpeedPenalties
}

func combineVRStaminaConsumption(
	consumption simulation.StaminaConsumption,
) (simulation.StaminaLevel, error) {
	total := consumption.Nautical + consumption.SailChange
	if err := total.Validate(); err != nil {
		return 0, fmt.Errorf("cumul de stamina Virtual Regatta invalide: %w", err)
	}
	return total, nil
}

func combineVRSpeedPenalties(
	first polar.ManeuverPenalty,
	second polar.ManeuverPenalty,
) (polar.ManeuverPenalty, error) {
	return polar.ManeuverPenalty{
		SpeedRatio: first.SpeedRatio * second.SpeedRatio,
		Duration:   first.Duration + second.Duration,
	}, nil
}

package httpapi

import (
	"fmt"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
)

// vrManeuverRuntime enables the confirmed stamina bookkeeping for routes that
// start from a Virtual Regatta state. The polar still supplies the actual
// maneuver speed and duration penalties; the stamina curves are neutral until
// the corresponding in-game curves are confirmed.
func vrManeuverRuntime(boatPolar *polar.Polar) (*simulation.TransitionEvaluator, simulation.ManeuverSpeedPenaltyCombiner) {
	evaluator := &simulation.TransitionEvaluator{
		WinchProfile:     boatPolar.Winch,
		ConsumptionRules: simulation.DefaultStaminaConsumptionRules(),
		WindCurve: simulation.StaminaWindCurve{
			Points: []simulation.StaminaWindPoint{
				{WindSpeed: 0, Factor: 1},
			},
		},
		DurationCurve: simulation.StaminaDurationCurve{
			Points: []simulation.StaminaDurationPoint{
				{Stamina: 0, Factor: 1},
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

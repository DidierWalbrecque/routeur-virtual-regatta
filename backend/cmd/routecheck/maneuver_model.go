package main

import (
	"errors"
	"fmt"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
)

type maneuverModel string

const (
	maneuverModelOff                  maneuverModel = "off"
	maneuverModelExperimentalAdditive maneuverModel = "experimental-additive"
)

type maneuverRuntime struct {
	evaluator             *simulation.TransitionEvaluator
	combineSpeedPenalties simulation.ManeuverSpeedPenaltyCombiner
}

func parseManeuverModel(value string) (maneuverModel, error) {
	model := maneuverModel(value)
	switch model {
	case maneuverModelOff, maneuverModelExperimentalAdditive:
		return model, nil
	default:
		return "", fmt.Errorf(
			"routecheck: -maneuver-model doit valoir %q ou %q",
			maneuverModelOff,
			maneuverModelExperimentalAdditive,
		)
	}
}

func newManeuverRuntime(
	model maneuverModel,
	winchProfile polar.WinchProfile,
) (maneuverRuntime, error) {
	switch model {
	case maneuverModelOff:
		return maneuverRuntime{}, nil
	case maneuverModelExperimentalAdditive:
		evaluator := simulation.TransitionEvaluator{
			WinchProfile:     winchProfile,
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
			CombineConsumption: combineExperimentalStaminaConsumption,
		}
		return maneuverRuntime{
			evaluator:             &evaluator,
			combineSpeedPenalties: combineExperimentalSpeedPenalties,
		}, nil
	default:
		return maneuverRuntime{}, errors.New(
			"routecheck: construction du modèle de manœuvres impossible: modèle inconnu",
		)
	}
}

func combineExperimentalStaminaConsumption(
	consumption simulation.StaminaConsumption,
) (simulation.StaminaLevel, error) {
	total := consumption.Nautical + consumption.SailChange
	if err := total.Validate(); err != nil {
		return 0, fmt.Errorf(
			"routecheck: cumul expérimental de stamina invalide: %w",
			err,
		)
	}
	return total, nil
}

func combineExperimentalSpeedPenalties(
	first polar.ManeuverPenalty,
	second polar.ManeuverPenalty,
) (polar.ManeuverPenalty, error) {
	return polar.ManeuverPenalty{
		SpeedRatio: first.SpeedRatio * second.SpeedRatio,
		Duration:   first.Duration + second.Duration,
	}, nil
}

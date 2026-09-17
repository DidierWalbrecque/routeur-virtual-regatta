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
	staminaRecoveryRules  *simulation.StaminaRecoveryRules
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
	recoveryRate simulation.StaminaRecoveryRate,
) (maneuverRuntime, error) {
	if err := recoveryRate.Validate(); err != nil {
		return maneuverRuntime{}, fmt.Errorf(
			"routecheck: taux de récupération de stamina invalide: %w",
			err,
		)
	}

	switch model {
	case maneuverModelOff:
		if recoveryRate != 0 {
			return maneuverRuntime{}, errors.New(
				"routecheck: -stamina-recovery-rate nécessite -maneuver-model experimental-additive",
			)
		}
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
		recoveryRules := simulation.StaminaRecoveryRules{
			PointsPerHour: recoveryRate,
		}
		return maneuverRuntime{
			evaluator:             &evaluator,
			combineSpeedPenalties: combineExperimentalSpeedPenalties,
			staminaRecoveryRules:  &recoveryRules,
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

package simulation

import (
	"errors"
	"fmt"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
)

// StaminaConsumptionCombiner resolves simultaneous nautical and sail-change
// costs. No default is provided until the in-game rule is confirmed.
type StaminaConsumptionCombiner func(
	StaminaConsumption,
) (StaminaLevel, error)

// TransitionEvaluator assembles maneuver detection, polar penalties and stamina
// calculations without combining simultaneous performance penalties.
type TransitionEvaluator struct {
	WinchProfile       polar.WinchProfile
	ConsumptionRules   StaminaConsumptionRules
	WindCurve          StaminaWindCurve
	DurationCurve      StaminaDurationCurve
	CombineConsumption StaminaConsumptionCombiner
}

// TransitionEvaluationRequest contains the state required to evaluate one
// transition.
type TransitionEvaluationRequest struct {
	Transition          ManeuverTransition
	WindSpeed           polar.WindSpeed
	Stamina             StaminaState
	ProfessionalWinches bool
}

// TransitionEvaluation contains independent penalties and the resulting
// stamina state.
type TransitionEvaluation struct {
	Maneuver          ManeuverState
	NauticalPenalty   *polar.ManeuverPenalty
	SailChangePenalty *polar.ManeuverPenalty
	Consumption       StaminaConsumption
	TotalConsumption  StaminaLevel
	NextStamina       StaminaState
}

// Evaluate calculates all confirmed effects of one boat transition. Duration
// factors use the stamina available at the start of the transition.
func (evaluator TransitionEvaluator) Evaluate(
	request TransitionEvaluationRequest,
) (TransitionEvaluation, error) {
	if err := request.Stamina.Validate(); err != nil {
		return TransitionEvaluation{}, fmt.Errorf(
			"simulation: évaluation de transition impossible: %w",
			err,
		)
	}

	maneuver, err := DetectManeuver(request.Transition)
	if err != nil {
		return TransitionEvaluation{}, fmt.Errorf(
			"simulation: évaluation de transition impossible: %w",
			err,
		)
	}

	baseConsumption, err := evaluator.ConsumptionRules.ConsumptionFor(
		maneuver,
	)
	if err != nil {
		return TransitionEvaluation{}, fmt.Errorf(
			"simulation: évaluation de transition impossible: %w",
			err,
		)
	}
	consumption, err := baseConsumption.AdjustedForWind(
		request.WindSpeed,
		evaluator.WindCurve,
	)
	if err != nil {
		return TransitionEvaluation{}, fmt.Errorf(
			"simulation: évaluation de transition impossible: %w",
			err,
		)
	}
	totalConsumption, err := evaluator.totalConsumption(consumption)
	if err != nil {
		return TransitionEvaluation{}, fmt.Errorf(
			"simulation: évaluation de transition impossible: %w",
			err,
		)
	}
	nextStamina, err := request.Stamina.ConsumedBy(totalConsumption)
	if err != nil {
		return TransitionEvaluation{}, fmt.Errorf(
			"simulation: évaluation de transition impossible: %w",
			err,
		)
	}

	result := TransitionEvaluation{
		Maneuver:         maneuver,
		Consumption:      consumption,
		TotalConsumption: totalConsumption,
		NextStamina:      nextStamina,
	}

	switch maneuver.Kind {
	case ManeuverNone:
	case ManeuverTack:
		penalty, penaltyErr := evaluator.WinchProfile.TackPenaltyAt(
			request.WindSpeed,
			request.ProfessionalWinches,
		)
		if penaltyErr != nil {
			return TransitionEvaluation{}, fmt.Errorf(
				"simulation: évaluation de transition impossible: %w",
				penaltyErr,
			)
		}
		adjusted, adjustErr := evaluator.DurationCurve.AdjustPenalty(
			penalty,
			request.Stamina.Level,
		)
		if adjustErr != nil {
			return TransitionEvaluation{}, fmt.Errorf(
				"simulation: évaluation de transition impossible: %w",
				adjustErr,
			)
		}
		result.NauticalPenalty = &adjusted
	case ManeuverGybe:
		penalty, penaltyErr := evaluator.WinchProfile.GybePenaltyAt(
			request.WindSpeed,
			request.ProfessionalWinches,
		)
		if penaltyErr != nil {
			return TransitionEvaluation{}, fmt.Errorf(
				"simulation: évaluation de transition impossible: %w",
				penaltyErr,
			)
		}
		adjusted, adjustErr := evaluator.DurationCurve.AdjustPenalty(
			penalty,
			request.Stamina.Level,
		)
		if adjustErr != nil {
			return TransitionEvaluation{}, fmt.Errorf(
				"simulation: évaluation de transition impossible: %w",
				adjustErr,
			)
		}
		result.NauticalPenalty = &adjusted
	}

	if maneuver.SailChanged {
		penalty, penaltyErr := evaluator.WinchProfile.SailChangePenaltyAt(
			request.WindSpeed,
			request.ProfessionalWinches,
		)
		if penaltyErr != nil {
			return TransitionEvaluation{}, fmt.Errorf(
				"simulation: évaluation de transition impossible: %w",
				penaltyErr,
			)
		}
		adjusted, adjustErr := evaluator.DurationCurve.AdjustPenalty(
			penalty,
			request.Stamina.Level,
		)
		if adjustErr != nil {
			return TransitionEvaluation{}, fmt.Errorf(
				"simulation: évaluation de transition impossible: %w",
				adjustErr,
			)
		}
		result.SailChangePenalty = &adjusted
	}

	return result, nil
}

func (evaluator TransitionEvaluator) totalConsumption(
	consumption StaminaConsumption,
) (StaminaLevel, error) {
	if consumption.Nautical == 0 {
		return consumption.SailChange, nil
	}
	if consumption.SailChange == 0 {
		return consumption.Nautical, nil
	}
	if evaluator.CombineConsumption == nil {
		return 0, errors.New(
			"règle de cumul de stamina requise pour des actions simultanées",
		)
	}

	total, err := evaluator.CombineConsumption(consumption)
	if err != nil {
		return 0, fmt.Errorf(
			"cumul de stamina impossible: %w",
			err,
		)
	}
	if err := total.Validate(); err != nil {
		return 0, fmt.Errorf(
			"cumul de stamina invalide: %w",
			err,
		)
	}
	return total, nil
}

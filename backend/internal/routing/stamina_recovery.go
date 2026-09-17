package routing

import (
	"fmt"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
)

func nextStaminaForStep(
	current simulation.StaminaState,
	maneuver *simulation.ManeuverState,
	transition *simulation.TransitionEvaluation,
	stepDuration time.Duration,
	rules *simulation.StaminaRecoveryRules,
) (simulation.StaminaState, error) {
	if transition != nil {
		if transition.TotalConsumption > 0 {
			return transition.NextStamina, nil
		}
		current = transition.NextStamina
	}
	if maneuver == nil {
		return current, nil
	}
	if maneuver.Kind == simulation.ManeuverTack ||
		maneuver.Kind == simulation.ManeuverGybe ||
		maneuver.SailChanged {
		return current, nil
	}
	if rules == nil {
		return current, nil
	}

	recovered, err := current.RecoveredFor(stepDuration, *rules)
	if err != nil {
		return simulation.StaminaState{}, fmt.Errorf(
			"récupération de stamina du pas impossible: %w",
			err,
		)
	}
	return recovered, nil
}

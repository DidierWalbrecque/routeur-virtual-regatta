package simulation

import "fmt"

// ConsumedBy subtracts an already-combined energy cost and floors the resulting
// stamina level at zero. The caller remains responsible for choosing how
// simultaneous action costs are combined.
func (state StaminaState) ConsumedBy(
	amount StaminaLevel,
) (StaminaState, error) {
	if err := state.Validate(); err != nil {
		return StaminaState{}, fmt.Errorf(
			"simulation: consommation de stamina impossible: %w",
			err,
		)
	}
	if err := amount.Validate(); err != nil {
		return StaminaState{}, fmt.Errorf(
			"simulation: consommation de stamina impossible: coût invalide: %w",
			err,
		)
	}

	level := state.Level - amount
	if level < 0 {
		level = 0
	}
	return StaminaState{Level: level}, nil
}

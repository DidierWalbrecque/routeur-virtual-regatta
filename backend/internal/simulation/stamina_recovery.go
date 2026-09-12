package simulation

import (
	"errors"
	"fmt"
	"math"
	"time"
)

// StaminaRecoveryRate is expressed in stamina percentage points per hour.
type StaminaRecoveryRate float64

// Validate checks that the recovery rate is finite and non-negative.
func (rate StaminaRecoveryRate) Validate() error {
	value := float64(rate)
	if math.IsNaN(value) || math.IsInf(value, 0) || rate < 0 {
		return errors.New(
			"simulation: taux de récupération fini positif ou nul requis",
		)
	}
	return nil
}

// StaminaRecoveryRules contains the configurable recovery rate. The caller
// supplies only the duration during which no stamina-consuming action occurs.
type StaminaRecoveryRules struct {
	PointsPerHour StaminaRecoveryRate
}

// Validate checks the stamina-recovery configuration.
func (rules StaminaRecoveryRules) Validate() error {
	if err := rules.PointsPerHour.Validate(); err != nil {
		return fmt.Errorf(
			"simulation: règles de récupération invalides: %w",
			err,
		)
	}
	return nil
}

// RecoveredFor applies recovery for an explicitly calm duration and caps the
// resulting stamina level at 100 percent.
func (state StaminaState) RecoveredFor(
	calmDuration time.Duration,
	rules StaminaRecoveryRules,
) (StaminaState, error) {
	if err := state.Validate(); err != nil {
		return StaminaState{}, fmt.Errorf(
			"simulation: récupération de stamina impossible: %w",
			err,
		)
	}
	if calmDuration < 0 {
		return StaminaState{}, errors.New(
			"simulation: récupération de stamina impossible: durée calme positive ou nulle requise",
		)
	}
	if err := rules.Validate(); err != nil {
		return StaminaState{}, fmt.Errorf(
			"simulation: récupération de stamina impossible: %w",
			err,
		)
	}

	recovered := float64(state.Level) +
		calmDuration.Hours()*float64(rules.PointsPerHour)
	if recovered >= 100 {
		recovered = 100
	}
	return StaminaState{Level: StaminaLevel(recovered)}, nil
}

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

// StaminaRecoveryRules configures stamina recovery during calm steps. Two
// models are supported: a constant rate (PointsPerHour), kept for backward
// compatibility; and the Virtual Regatta wind-dependent model, where the time
// to recover Points stamina points rises with a cosine profile from LowTime at
// LowWind to HighTime at HighWind. The wind model applies as soon as it is
// fully configured (usesWindModel); otherwise the constant rate is used.
type StaminaRecoveryRules struct {
	PointsPerHour StaminaRecoveryRate

	Points   float64
	LowWind  float64
	HighWind float64
	LowTime  time.Duration
	HighTime time.Duration
}

// usesWindModel reports whether the wind-dependent recovery model is configured.
func (rules StaminaRecoveryRules) usesWindModel() bool {
	return rules.Points > 0 &&
		rules.LowTime > 0 &&
		rules.HighTime > 0 &&
		rules.HighWind > rules.LowWind
}

// Validate checks the stamina-recovery configuration.
func (rules StaminaRecoveryRules) Validate() error {
	if err := rules.PointsPerHour.Validate(); err != nil {
		return fmt.Errorf(
			"simulation: règles de récupération invalides: %w",
			err,
		)
	}
	if rules.usesWindModel() {
		for _, value := range []float64{rules.Points, rules.LowWind, rules.HighWind} {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return errors.New(
					"simulation: règles de récupération invalides: valeur non finie",
				)
			}
		}
		if rules.LowWind < 0 {
			return errors.New(
				"simulation: règles de récupération invalides: vent minimal négatif",
			)
		}
	}
	return nil
}

// recoveryIntervalFor returns the time needed to recover Points stamina points
// at the given wind speed, following the cosine profile of the wind model.
func (rules StaminaRecoveryRules) recoveryIntervalFor(windKnots float64) time.Duration {
	low := float64(rules.LowTime)
	high := float64(rules.HighTime)
	switch {
	case windKnots <= rules.LowWind:
		return time.Duration(low)
	case windKnots >= rules.HighWind:
		return time.Duration(high)
	default:
		average := (high + low) / 2
		amplitude := (high - low) / 2
		position := (windKnots - rules.LowWind) /
			(rules.HighWind - rules.LowWind)
		return time.Duration(average - math.Cos(position*math.Pi)*amplitude)
	}
}

// RecoveredFor applies the constant-rate recovery for an explicitly calm
// duration and caps the resulting stamina level at 100 percent.
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

// RecoveredForWind applies recovery for a calm duration, using the
// wind-dependent model when it is configured and falling back to the
// constant-rate model otherwise. Recovery is capped at 100 percent.
func (state StaminaState) RecoveredForWind(
	calmDuration time.Duration,
	windKnots float64,
	rules StaminaRecoveryRules,
) (StaminaState, error) {
	if !rules.usesWindModel() {
		return state.RecoveredFor(calmDuration, rules)
	}
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

	interval := rules.recoveryIntervalFor(windKnots)
	if interval <= 0 {
		return StaminaState{}, errors.New(
			"simulation: récupération de stamina impossible: intervalle de récupération invalide",
		)
	}
	gained := rules.Points * float64(calmDuration) / float64(interval)
	recovered := float64(state.Level) + gained
	if recovered >= 100 {
		recovered = 100
	}
	return StaminaState{Level: StaminaLevel(recovered)}, nil
}

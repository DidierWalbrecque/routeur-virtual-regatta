package simulation

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
)

// ManeuverDurationFactor multiplies the base maneuver duration.
type ManeuverDurationFactor float64

// Validate checks that the duration factor is finite and strictly positive.
func (factor ManeuverDurationFactor) Validate() error {
	value := float64(factor)
	if math.IsNaN(value) || math.IsInf(value, 0) || factor <= 0 {
		return errors.New(
			"simulation: coefficient de durée fini strictement positif requis",
		)
	}
	return nil
}

// StaminaDurationPoint associates a stamina level with a duration factor.
type StaminaDurationPoint struct {
	Stamina StaminaLevel
	Factor  ManeuverDurationFactor
}

// StaminaDurationCurve describes the configurable effect of current stamina on
// maneuver duration. Values outside its range use the nearest endpoint.
type StaminaDurationCurve struct {
	Points []StaminaDurationPoint
}

// Validate checks curve values and their strictly increasing stamina order.
func (curve StaminaDurationCurve) Validate() error {
	if len(curve.Points) == 0 {
		return errors.New(
			"simulation: courbe de durée selon la stamina vide",
		)
	}
	for index, point := range curve.Points {
		if err := point.Stamina.Validate(); err != nil {
			return fmt.Errorf(
				"simulation: stamina invalide au point %d: %w",
				index,
				err,
			)
		}
		if index > 0 &&
			point.Stamina <= curve.Points[index-1].Stamina {
			return fmt.Errorf(
				"simulation: niveaux de stamina non strictement croissants au point %d",
				index,
			)
		}
		if err := point.Factor.Validate(); err != nil {
			return fmt.Errorf(
				"simulation: coefficient de durée invalide au point %d: %w",
				index,
				err,
			)
		}
	}
	return nil
}

// FactorAt returns the linearly interpolated maneuver-duration factor.
func (curve StaminaDurationCurve) FactorAt(
	stamina StaminaLevel,
) (ManeuverDurationFactor, error) {
	if err := stamina.Validate(); err != nil {
		return 0, fmt.Errorf(
			"simulation: calcul du coefficient de durée impossible: %w",
			err,
		)
	}
	if err := curve.Validate(); err != nil {
		return 0, fmt.Errorf(
			"simulation: calcul du coefficient de durée impossible: %w",
			err,
		)
	}

	if stamina <= curve.Points[0].Stamina {
		return curve.Points[0].Factor, nil
	}
	last := curve.Points[len(curve.Points)-1]
	if stamina >= last.Stamina {
		return last.Factor, nil
	}

	for index := 1; index < len(curve.Points); index++ {
		upper := curve.Points[index]
		if stamina > upper.Stamina {
			continue
		}
		lower := curve.Points[index-1]
		position := float64(
			(stamina - lower.Stamina) /
				(upper.Stamina - lower.Stamina),
		)
		return ManeuverDurationFactor(
			float64(lower.Factor) +
				position*float64(upper.Factor-lower.Factor),
		), nil
	}

	return 0, errors.New(
		"simulation: calcul du coefficient de durée impossible: intervalle introuvable",
	)
}

// AdjustPenalty applies the stamina factor to a maneuver duration while
// preserving its speed ratio.
func (curve StaminaDurationCurve) AdjustPenalty(
	penalty polar.ManeuverPenalty,
	stamina StaminaLevel,
) (polar.ManeuverPenalty, error) {
	ratio := float64(penalty.SpeedRatio)
	if math.IsNaN(ratio) ||
		math.IsInf(ratio, 0) ||
		penalty.SpeedRatio < 0 ||
		penalty.SpeedRatio > 1 {
		return polar.ManeuverPenalty{}, errors.New(
			"simulation: ajustement de pénalité impossible: ratio de vitesse fini compris entre 0 et 1 requis",
		)
	}
	if penalty.Duration <= 0 {
		return polar.ManeuverPenalty{}, errors.New(
			"simulation: ajustement de pénalité impossible: durée de base strictement positive requise",
		)
	}

	factor, err := curve.FactorAt(stamina)
	if err != nil {
		return polar.ManeuverPenalty{}, fmt.Errorf(
			"simulation: ajustement de pénalité impossible: %w",
			err,
		)
	}

	duration := float64(penalty.Duration) * float64(factor)
	const maximumDuration = float64(1<<63 - 1)
	if math.IsNaN(duration) ||
		math.IsInf(duration, 0) ||
		duration < 1 ||
		duration > maximumDuration {
		return polar.ManeuverPenalty{}, errors.New(
			"simulation: ajustement de pénalité impossible: durée calculée invalide",
		)
	}

	return polar.ManeuverPenalty{
		SpeedRatio: penalty.SpeedRatio,
		Duration:   time.Duration(math.Round(duration)),
	}, nil
}

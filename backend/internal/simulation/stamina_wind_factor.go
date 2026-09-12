package simulation

import (
	"errors"
	"fmt"
	"math"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
)

// StaminaWindFactor multiplies a base maneuver-energy cost.
type StaminaWindFactor float64

// Validate checks that the factor is finite and non-negative.
func (factor StaminaWindFactor) Validate() error {
	value := float64(factor)
	if math.IsNaN(value) || math.IsInf(value, 0) || factor < 0 {
		return errors.New(
			"simulation: coefficient de vent fini positif ou nul requis",
		)
	}
	return nil
}

// StaminaWindPoint associates a wind speed in knots with an energy factor.
type StaminaWindPoint struct {
	WindSpeed polar.WindSpeed
	Factor    StaminaWindFactor
}

// StaminaWindCurve describes a configurable piecewise-linear energy curve.
// Values outside its wind-speed range use the nearest endpoint.
type StaminaWindCurve struct {
	Points []StaminaWindPoint
}

// Validate checks curve values and their strictly increasing wind-speed order.
func (curve StaminaWindCurve) Validate() error {
	if len(curve.Points) == 0 {
		return errors.New(
			"simulation: courbe de coefficient de vent vide",
		)
	}
	for index, point := range curve.Points {
		windSpeed := float64(point.WindSpeed)
		if math.IsNaN(windSpeed) ||
			math.IsInf(windSpeed, 0) ||
			point.WindSpeed < 0 {
			return fmt.Errorf(
				"simulation: vitesse du vent invalide au point %d",
				index,
			)
		}
		if index > 0 &&
			point.WindSpeed <= curve.Points[index-1].WindSpeed {
			return fmt.Errorf(
				"simulation: vitesses du vent non strictement croissantes au point %d",
				index,
			)
		}
		if err := point.Factor.Validate(); err != nil {
			return fmt.Errorf(
				"simulation: coefficient de vent invalide au point %d: %w",
				index,
				err,
			)
		}
	}
	return nil
}

// FactorAt returns the linearly interpolated energy factor at a wind speed.
func (curve StaminaWindCurve) FactorAt(
	windSpeed polar.WindSpeed,
) (StaminaWindFactor, error) {
	value := float64(windSpeed)
	if math.IsNaN(value) || math.IsInf(value, 0) || windSpeed < 0 {
		return 0, errors.New(
			"simulation: calcul du coefficient de vent impossible: vitesse du vent finie positive ou nulle requise",
		)
	}
	if err := curve.Validate(); err != nil {
		return 0, fmt.Errorf(
			"simulation: calcul du coefficient de vent impossible: %w",
			err,
		)
	}

	if windSpeed <= curve.Points[0].WindSpeed {
		return curve.Points[0].Factor, nil
	}
	last := curve.Points[len(curve.Points)-1]
	if windSpeed >= last.WindSpeed {
		return last.Factor, nil
	}

	for index := 1; index < len(curve.Points); index++ {
		upper := curve.Points[index]
		if windSpeed > upper.WindSpeed {
			continue
		}
		lower := curve.Points[index-1]
		position := float64(
			(windSpeed - lower.WindSpeed) /
				(upper.WindSpeed - lower.WindSpeed),
		)
		return StaminaWindFactor(
			float64(lower.Factor) +
				position*float64(upper.Factor-lower.Factor),
		), nil
	}

	return 0, errors.New(
		"simulation: calcul du coefficient de vent impossible: intervalle introuvable",
	)
}

// AdjustedForWind applies the same interpolated wind factor to both independent
// action costs without combining them.
func (consumption StaminaConsumption) AdjustedForWind(
	windSpeed polar.WindSpeed,
	curve StaminaWindCurve,
) (StaminaConsumption, error) {
	if err := consumption.Validate(); err != nil {
		return StaminaConsumption{}, fmt.Errorf(
			"simulation: ajustement de consommation impossible: %w",
			err,
		)
	}
	factor, err := curve.FactorAt(windSpeed)
	if err != nil {
		return StaminaConsumption{}, fmt.Errorf(
			"simulation: ajustement de consommation impossible: %w",
			err,
		)
	}

	result := StaminaConsumption{
		Nautical: StaminaLevel(
			float64(consumption.Nautical) * float64(factor),
		),
		SailChange: StaminaLevel(
			float64(consumption.SailChange) * float64(factor),
		),
	}
	if err := result.Validate(); err != nil {
		return StaminaConsumption{}, fmt.Errorf(
			"simulation: ajustement de consommation impossible: résultat invalide: %w",
			err,
		)
	}
	return result, nil
}

package polar

import "fmt"

// RatioAt calcule le coefficient des foils pour une TWS et une TWA données.
// Il reproduit les transitions bilinéaires utilisées par VRPolarsChart.
func (foil FoilProfile) RatioAt(
	windSpeed WindSpeed,
	windAngle TrueWindAngle,
) (SpeedRatio, error) {
	if err := foil.validate(); err != nil {
		return 0, err
	}
	if !isFinite(float64(windSpeed)) || windSpeed < 0 {
		return 0, fmt.Errorf("polar: la TWS des foils doit être un nombre fini positif ou nul")
	}
	if !isFinite(float64(windAngle)) || windAngle < 0 || windAngle > 180 {
		return 0, fmt.Errorf("polar: la TWA des foils doit être comprise entre 0 et 180 degrés")
	}

	windSpeedActivation := foilAxisActivation(
		float64(windSpeed),
		float64(foil.TWSMin),
		float64(foil.TWSMax),
		float64(foil.TWSMerge),
	)
	windAngleActivation := foilAxisActivation(
		float64(windAngle),
		float64(foil.TWAMin),
		float64(foil.TWAMax),
		float64(foil.TWAMerge),
	)

	activation := windSpeedActivation * windAngleActivation
	return 1 + (foil.MaxSpeedRatio-1)*SpeedRatio(activation), nil
}

func (foil FoilProfile) validate() error {
	if !isFinite(float64(foil.MaxSpeedRatio)) || foil.MaxSpeedRatio < 1 {
		return fmt.Errorf(
			"polar: foil.speedRatio doit être un nombre fini supérieur ou égal à 1",
		)
	}

	if !isFinite(float64(foil.TWAMin)) ||
		!isFinite(float64(foil.TWAMax)) ||
		foil.TWAMin < 0 ||
		foil.TWAMax > 180 ||
		foil.TWAMin > foil.TWAMax {
		return fmt.Errorf(
			"polar: la plage TWA des foils doit être finie, croissante et comprise entre 0 et 180 degrés",
		)
	}
	if !isFinite(float64(foil.TWAMerge)) ||
		foil.TWAMerge < 0 ||
		foil.TWAMerge > foil.TWAMin {
		return fmt.Errorf(
			"polar: foil.twaMerge doit être fini, positif ou nul et inférieur ou égal à twaMin",
		)
	}

	if !isFinite(float64(foil.TWSMin)) ||
		!isFinite(float64(foil.TWSMax)) ||
		foil.TWSMin < 0 ||
		foil.TWSMin > foil.TWSMax {
		return fmt.Errorf(
			"polar: la plage TWS des foils doit être finie, croissante et positive ou nulle",
		)
	}
	if !isFinite(float64(foil.TWSMerge)) ||
		foil.TWSMerge < 0 ||
		foil.TWSMerge > foil.TWSMin {
		return fmt.Errorf(
			"polar: foil.twsMerge doit être fini, positif ou nul et inférieur ou égal à twsMin",
		)
	}

	if !isFinite(float64(foil.TWAMax+foil.TWAMerge)) ||
		!isFinite(float64(foil.TWSMax+foil.TWSMerge)) {
		return fmt.Errorf("polar: les limites supérieures des foils doivent être finies")
	}

	return nil
}

func foilAxisActivation(
	value float64,
	minimum float64,
	maximum float64,
	merge float64,
) float64 {
	if merge == 0 {
		if value >= minimum && value <= maximum {
			return 1
		}
		return 0
	}

	lowerStart := minimum - merge
	upperEnd := maximum + merge

	switch {
	case value <= lowerStart:
		return 0
	case value < minimum:
		return (value - lowerStart) / merge
	case value <= maximum:
		return 1
	case value < upperEnd:
		return (upperEnd - value) / merge
	default:
		return 0
	}
}

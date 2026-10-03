package polar

import "fmt"

// SpeedOptions décrit les équipements qui modifient la vitesse brute.
type SpeedOptions struct {
	Foils      bool
	HullPolish bool
}

// EffectiveSpeedAt calcule la vitesse d'une voile après application du
// coefficient global et des équipements activés.
func (polar *Polar) EffectiveSpeedAt(
	sailID SailID,
	windSpeed WindSpeed,
	windAngle TrueWindAngle,
	options SpeedOptions,
) (BoatSpeed, error) {
	rawSpeed, err := polar.SpeedAt(sailID, windSpeed, windAngle)
	if err != nil {
		return 0, fmt.Errorf("polar: calcul de la vitesse effective impossible: %w", err)
	}
	return polar.effectiveSpeedFromRaw(rawSpeed, windSpeed, windAngle, options)
}

func (polar *Polar) effectiveSpeedFromRaw(
	rawSpeed BoatSpeed,
	windSpeed WindSpeed,
	windAngle TrueWindAngle,
	options SpeedOptions,
) (BoatSpeed, error) {
	effectiveSpeed, err := applySpeedRatio(
		rawSpeed,
		polar.GlobalSpeedRatio,
		"globalSpeedRatio",
	)
	if err != nil {
		return 0, err
	}

	if options.Foils {
		foilRatio, ratioErr := polar.Foil.RatioAt(windSpeed, windAngle)
		if ratioErr != nil {
			return 0, fmt.Errorf(
				"polar: calcul du coefficient des foils impossible: %w",
				ratioErr,
			)
		}

		effectiveSpeed, err = applySpeedRatio(
			effectiveSpeed,
			foilRatio,
			"foil.speedRatioAt",
		)
		if err != nil {
			return 0, err
		}
	}

	if options.HullPolish {
		effectiveSpeed, err = applySpeedRatio(
			effectiveSpeed,
			polar.Hull.SpeedRatio,
			"hull.speedRatio",
		)
		if err != nil {
			return 0, err
		}
	}

	return effectiveSpeed, nil
}

func applySpeedRatio(
	speed BoatSpeed,
	ratio SpeedRatio,
	ratioName string,
) (BoatSpeed, error) {
	numericRatio := float64(ratio)
	if !isFinite(numericRatio) || ratio <= 0 {
		return 0, fmt.Errorf(
			"polar: application du coefficient impossible: %s doit être un nombre fini strictement positif",
			ratioName,
		)
	}

	result := speed * BoatSpeed(ratio)
	if !isFinite(float64(result)) {
		return 0, fmt.Errorf(
			"polar: application du coefficient %s produit une vitesse non finie",
			ratioName,
		)
	}
	return result, nil
}

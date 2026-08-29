package polar

import "fmt"

// SpeedOptions décrit les équipements qui modifient la vitesse brute.
// Les foils seront ajoutés séparément car leur coefficient dépend du vent.
type SpeedOptions struct {
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

	effectiveSpeed, err := applySpeedRatio(
		rawSpeed,
		polar.GlobalSpeedRatio,
		"globalSpeedRatio",
	)
	if err != nil {
		return 0, err
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

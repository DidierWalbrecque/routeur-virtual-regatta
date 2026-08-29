package polar

import "fmt"

// SpeedAt calcule la vitesse d'une voile pour une TWS et une TWA données.
// La polaire doit avoir été validée avant l'appel.
func (polar *Polar) SpeedAt(
	sailID SailID,
	windSpeed WindSpeed,
	windAngle TrueWindAngle,
) (BoatSpeed, error) {
	if polar == nil {
		return 0, fmt.Errorf("polar: calcul de vitesse impossible: la polaire est nil")
	}

	for sailIndex := range polar.Sails {
		sail := &polar.Sails[sailIndex]
		if sail.ID != sailID {
			continue
		}

		speed, err := interpolateSpeedGrid(
			polar.WindSpeeds,
			polar.WindAngles,
			sail.Speeds,
			windSpeed,
			windAngle,
		)
		if err != nil {
			return 0, fmt.Errorf(
				"polar: calcul de vitesse impossible pour la voile %q (%d): %w",
				sail.Name,
				sail.ID,
				err,
			)
		}
		return speed, nil
	}

	return 0, fmt.Errorf("polar: voile %d introuvable", sailID)
}

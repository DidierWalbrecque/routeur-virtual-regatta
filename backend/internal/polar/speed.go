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

	sail, found := polar.sailByID(sailID)
	if !found {
		return 0, fmt.Errorf("polar: voile %d introuvable", sailID)
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

func (polar *Polar) sailByID(sailID SailID) (*Sail, bool) {
	if polar == nil {
		return nil, false
	}
	for sailIndex := range polar.Sails {
		if polar.Sails[sailIndex].ID == sailID {
			return &polar.Sails[sailIndex], true
		}
	}
	return nil, false
}

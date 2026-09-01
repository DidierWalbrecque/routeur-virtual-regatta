package polar

import "fmt"

// SailPerformance associe une voile à sa vitesse calculée.
type SailPerformance struct {
	SailID   SailID
	SailName string
	Speed    BoatSpeed
}

// BestSailAt retourne la voile disponible la plus rapide pour une TWS et une
// TWA données. En cas d'égalité, la première voile de la liste est conservée.
func (polar *Polar) BestSailAt(
	windSpeed WindSpeed,
	windAngle TrueWindAngle,
	availableSails []SailID,
) (SailPerformance, error) {
	if polar == nil {
		return SailPerformance{}, fmt.Errorf(
			"polar: sélection de la meilleure voile impossible: la polaire est nil",
		)
	}
	if len(availableSails) == 0 {
		return SailPerformance{}, fmt.Errorf(
			"polar: sélection de la meilleure voile impossible: aucune voile disponible",
		)
	}

	seen := make(map[SailID]struct{}, len(availableSails))
	var best SailPerformance
	found := false

	for _, sailID := range availableSails {
		if _, duplicate := seen[sailID]; duplicate {
			continue
		}
		seen[sailID] = struct{}{}

		sail, exists := polar.sailByID(sailID)
		if !exists {
			return SailPerformance{}, fmt.Errorf(
				"polar: sélection de la meilleure voile impossible: voile disponible %d introuvable",
				sailID,
			)
		}

		speed, err := polar.SpeedAt(sailID, windSpeed, windAngle)
		if err != nil {
			return SailPerformance{}, fmt.Errorf(
				"polar: comparaison de la voile %q (%d) impossible: %w",
				sail.Name,
				sail.ID,
				err,
			)
		}

		if !found || speed > best.Speed {
			best = SailPerformance{
				SailID:   sail.ID,
				SailName: sail.Name,
				Speed:    speed,
			}
			found = true
		}
	}

	return best, nil
}

// BestEffectiveSailAt retourne la voile disponible la plus rapide et sa vitesse
// après application du coefficient global et des équipements activés.
func (polar *Polar) BestEffectiveSailAt(
	windSpeed WindSpeed,
	windAngle TrueWindAngle,
	availableSails []SailID,
	options SpeedOptions,
) (SailPerformance, error) {
	best, err := polar.BestSailAt(windSpeed, windAngle, availableSails)
	if err != nil {
		return SailPerformance{}, fmt.Errorf(
			"polar: sélection de la meilleure vitesse effective impossible: %w",
			err,
		)
	}

	effectiveSpeed, err := polar.EffectiveSpeedAt(
		best.SailID,
		windSpeed,
		windAngle,
		options,
	)
	if err != nil {
		return SailPerformance{}, fmt.Errorf(
			"polar: calcul de la vitesse effective de la voile %q (%d) impossible: %w",
			best.SailName,
			best.SailID,
			err,
		)
	}

	best.Speed = effectiveSpeed
	return best, nil
}

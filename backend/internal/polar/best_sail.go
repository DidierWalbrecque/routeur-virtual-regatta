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

	effectiveSpeed, err := polar.effectiveSpeedFromRaw(
		best.Speed,
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

// PreferredEffectiveSailAt returns the fastest available sail, but keeps the
// current sail when no alternative beats it by more than the sail-change
// tolerance (Virtual Regatta's autoSailChangeTolerance). A purely greedy
// per-step choice otherwise oscillates between near-equal sails and drains
// stamina. A zero or non-positive currentSailID, or a tolerance <= 1, disables
// the hysteresis and returns the fastest sail.
func (polar *Polar) PreferredEffectiveSailAt(
	windSpeed WindSpeed,
	windAngle TrueWindAngle,
	availableSails []SailID,
	options SpeedOptions,
	currentSailID SailID,
	tolerance SpeedRatio,
) (SailPerformance, error) {
	best, err := polar.BestEffectiveSailAt(
		windSpeed,
		windAngle,
		availableSails,
		options,
	)
	if err != nil {
		return SailPerformance{}, err
	}
	if currentSailID <= 0 || tolerance <= 1 || currentSailID == best.SailID {
		return best, nil
	}
	available := false
	for _, sailID := range availableSails {
		if sailID == currentSailID {
			available = true
			break
		}
	}
	if !available {
		return best, nil
	}
	currentSpeed, err := polar.EffectiveSpeedAt(
		currentSailID,
		windSpeed,
		windAngle,
		options,
	)
	if err != nil {
		return SailPerformance{}, err
	}
	// On ne change de voile que si la meilleure bat la voile courante de plus
	// que la tolérance ; sinon on garde la courante (hystérésis).
	if float64(best.Speed) <= float64(currentSpeed)*float64(tolerance) {
		name := ""
		if sail, exists := polar.sailByID(currentSailID); exists {
			name = sail.Name
		}
		return SailPerformance{
			SailID:   currentSailID,
			SailName: name,
			Speed:    currentSpeed,
		}, nil
	}
	return best, nil
}

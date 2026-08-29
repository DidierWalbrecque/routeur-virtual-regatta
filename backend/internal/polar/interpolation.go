package polar

import "fmt"

// interpolateWindSpeed calcule la vitesse du bateau pour une TWS donnée
// sur une seule ligne de la grille, donc pour une TWA déjà fixée.
func interpolateWindSpeed(
	windSpeeds []WindSpeed,
	boatSpeeds []BoatSpeed,
	target WindSpeed,
) (BoatSpeed, error) {
	if len(windSpeeds) != len(boatSpeeds) {
		return 0, fmt.Errorf(
			"polar: dimensions incompatibles pour l'interpolation TWS: %d vents et %d vitesses",
			len(windSpeeds),
			len(boatSpeeds),
		)
	}

	bounds, err := findAxisBounds(windSpeeds, target)
	if err != nil {
		return 0, fmt.Errorf("polar: interpolation TWS impossible: %w", err)
	}

	if bounds.LowerIndex == bounds.UpperIndex {
		return boatSpeeds[bounds.LowerIndex], nil
	}

	lowerWindSpeed := windSpeeds[bounds.LowerIndex]
	upperWindSpeed := windSpeeds[bounds.UpperIndex]
	lowerBoatSpeed := boatSpeeds[bounds.LowerIndex]
	upperBoatSpeed := boatSpeeds[bounds.UpperIndex]

	position := float64(target-lowerWindSpeed) / float64(upperWindSpeed-lowerWindSpeed)
	return lowerBoatSpeed + BoatSpeed(position)*(upperBoatSpeed-lowerBoatSpeed), nil
}

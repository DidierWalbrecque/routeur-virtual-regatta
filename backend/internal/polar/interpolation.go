package polar

import "fmt"

// interpolateWindSpeed calcule la vitesse du bateau pour une TWS donnée
// sur une seule ligne de la grille, donc pour une TWA déjà fixée.
func interpolateWindSpeed(
	windSpeeds []WindSpeed,
	boatSpeeds []BoatSpeed,
	target WindSpeed,
) (BoatSpeed, error) {
	return interpolateAxis(windSpeeds, boatSpeeds, target, "TWS")
}

// interpolateWindAngle calcule la vitesse du bateau pour une TWA donnée
// à partir de vitesses déjà déterminées pour chaque angle.
func interpolateWindAngle(
	windAngles []TrueWindAngle,
	boatSpeeds []BoatSpeed,
	target TrueWindAngle,
) (BoatSpeed, error) {
	return interpolateAxis(windAngles, boatSpeeds, target, "TWA")
}

// interpolateAxis applique la même interpolation linéaire aux axes TWS et TWA.
func interpolateAxis[T ~float64](
	axis []T,
	boatSpeeds []BoatSpeed,
	target T,
	axisName string,
) (BoatSpeed, error) {
	if len(axis) != len(boatSpeeds) {
		return 0, fmt.Errorf(
			"polar: dimensions incompatibles pour l'interpolation %s: %d points d'axe et %d vitesses",
			axisName,
			len(axis),
			len(boatSpeeds),
		)
	}

	bounds, err := findAxisBounds(axis, target)
	if err != nil {
		return 0, fmt.Errorf("polar: interpolation %s impossible: %w", axisName, err)
	}

	if bounds.LowerIndex == bounds.UpperIndex {
		return boatSpeeds[bounds.LowerIndex], nil
	}

	lowerAxisValue := axis[bounds.LowerIndex]
	upperAxisValue := axis[bounds.UpperIndex]
	lowerBoatSpeed := boatSpeeds[bounds.LowerIndex]
	upperBoatSpeed := boatSpeeds[bounds.UpperIndex]

	position := float64(target-lowerAxisValue) / float64(upperAxisValue-lowerAxisValue)
	return lowerBoatSpeed + BoatSpeed(position)*(upperBoatSpeed-lowerBoatSpeed), nil
}

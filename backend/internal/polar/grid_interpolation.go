package polar

import "fmt"

// interpolateSpeedGrid calcule la vitesse du bateau pour une TWS et une TWA
// situées sur la grille ou entre quatre de ses points.
func interpolateSpeedGrid(
	windSpeeds []WindSpeed,
	windAngles []TrueWindAngle,
	grid SpeedGrid,
	targetWindSpeed WindSpeed,
	targetWindAngle TrueWindAngle,
) (BoatSpeed, error) {
	if len(grid.Values) != len(windAngles) {
		return 0, fmt.Errorf(
			"polar: dimensions incompatibles pour l'interpolation de grille: %d lignes et %d angles TWA",
			len(grid.Values),
			len(windAngles),
		)
	}
	for rowIndex, row := range grid.Values {
		if len(row) != len(windSpeeds) {
			return 0, fmt.Errorf(
				"polar: dimensions incompatibles pour l'interpolation de grille: la ligne %d possède %d colonnes, attendu %d",
				rowIndex,
				len(row),
				len(windSpeeds),
			)
		}
	}

	angleBounds, err := findAxisBounds(windAngles, targetWindAngle)
	if err != nil {
		return 0, fmt.Errorf("polar: interpolation de grille impossible sur l'axe TWA: %w", err)
	}

	lowerSpeed, err := interpolateWindSpeed(
		windSpeeds,
		grid.Values[angleBounds.LowerIndex],
		targetWindSpeed,
	)
	if err != nil {
		return 0, fmt.Errorf(
			"polar: interpolation de grille impossible à TWA=%g: %w",
			float64(windAngles[angleBounds.LowerIndex]),
			err,
		)
	}

	if angleBounds.LowerIndex == angleBounds.UpperIndex {
		return lowerSpeed, nil
	}

	upperSpeed, err := interpolateWindSpeed(
		windSpeeds,
		grid.Values[angleBounds.UpperIndex],
		targetWindSpeed,
	)
	if err != nil {
		return 0, fmt.Errorf(
			"polar: interpolation de grille impossible à TWA=%g: %w",
			float64(windAngles[angleBounds.UpperIndex]),
			err,
		)
	}

	return interpolateWindAngle(
		[]TrueWindAngle{
			windAngles[angleBounds.LowerIndex],
			windAngles[angleBounds.UpperIndex],
		},
		[]BoatSpeed{lowerSpeed, upperSpeed},
		targetWindAngle,
	)
}

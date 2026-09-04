package navigation

import (
	"fmt"
	"math"
)

// InitialHeading calcule le cap initial de la route orthodromique entre deux
// positions. Le résultat est exprimé en degrés depuis le nord dans [0, 360[.
func InitialHeading(
	from Position,
	to Position,
) (Heading, error) {
	if err := from.Validate(); err != nil {
		return 0, fmt.Errorf(
			"navigation: position de départ invalide: %w",
			err,
		)
	}
	if err := to.Validate(); err != nil {
		return 0, fmt.Errorf(
			"navigation: position d'arrivée invalide: %w",
			err,
		)
	}

	fromLatitude := degreesToRadians(float64(from.Latitude))
	toLatitude := degreesToRadians(float64(to.Latitude))
	longitudeDelta := degreesToRadians(
		float64(to.Longitude - from.Longitude),
	)

	eastComponent := math.Sin(longitudeDelta) * math.Cos(toLatitude)
	northComponent := math.Cos(fromLatitude)*math.Sin(toLatitude) -
		math.Sin(fromLatitude)*math.Cos(toLatitude)*
			math.Cos(longitudeDelta)

	if math.Hypot(eastComponent, northComponent) < 1e-15 {
		return 0, fmt.Errorf(
			"navigation: le cap initial est indéfini pour des positions identiques ou antipodales",
		)
	}

	heading := math.Mod(
		radiansToDegrees(math.Atan2(eastComponent, northComponent))+360,
		360,
	)
	result := Heading(heading)
	if err := result.Validate(); err != nil {
		return 0, fmt.Errorf(
			"navigation: calcul du cap initial impossible: %w",
			err,
		)
	}
	return result, nil
}

func radiansToDegrees(radians float64) float64 {
	return radians * 180 / math.Pi
}

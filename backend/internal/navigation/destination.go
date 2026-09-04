package navigation

import (
	"fmt"
	"math"
)

// DestinationPosition calcule la position atteinte après avoir parcouru une
// distance sur une route orthodromique suivant le cap initial indiqué.
func DestinationPosition(
	from Position,
	heading Heading,
	distance NauticalMiles,
) (Position, error) {
	if err := from.Validate(); err != nil {
		return Position{}, fmt.Errorf(
			"navigation: position de départ invalide: %w",
			err,
		)
	}
	if err := heading.Validate(); err != nil {
		return Position{}, fmt.Errorf(
			"navigation: cap invalide: %w",
			err,
		)
	}
	if err := distance.Validate(); err != nil {
		return Position{}, fmt.Errorf(
			"navigation: distance invalide: %w",
			err,
		)
	}
	if distance == 0 {
		return from, nil
	}

	fromLatitude := degreesToRadians(float64(from.Latitude))
	fromLongitude := degreesToRadians(float64(from.Longitude))
	initialHeading := degreesToRadians(float64(heading))
	angularDistance := float64(distance) / earthRadiusNauticalMiles

	destinationLatitude := math.Asin(
		math.Sin(fromLatitude)*math.Cos(angularDistance) +
			math.Cos(fromLatitude)*math.Sin(angularDistance)*
				math.Cos(initialHeading),
	)
	destinationLongitude := fromLongitude + math.Atan2(
		math.Sin(initialHeading)*math.Sin(angularDistance)*
			math.Cos(fromLatitude),
		math.Cos(angularDistance)-
			math.Sin(fromLatitude)*math.Sin(destinationLatitude),
	)

	result := Position{
		Latitude: Latitude(radiansToDegrees(destinationLatitude)),
		Longitude: Longitude(normalizeLongitude(
			radiansToDegrees(destinationLongitude),
		)),
	}
	if err := result.Validate(); err != nil {
		return Position{}, fmt.Errorf(
			"navigation: position calculée invalide: %w",
			err,
		)
	}
	return result, nil
}

func normalizeLongitude(longitude float64) float64 {
	normalized := math.Mod(longitude+180, 360)
	if normalized < 0 {
		normalized += 360
	}
	return normalized - 180
}

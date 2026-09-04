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

	normalizedLongitude, err := NormalizeLongitude(
		radiansToDegrees(destinationLongitude),
	)
	if err != nil {
		return Position{}, fmt.Errorf(
			"navigation: normalisation de la longitude impossible: %w",
			err,
		)
	}
	result := Position{
		Latitude:  Latitude(radiansToDegrees(destinationLatitude)),
		Longitude: normalizedLongitude,
	}
	if err := result.Validate(); err != nil {
		return Position{}, fmt.Errorf(
			"navigation: position calculée invalide: %w",
			err,
		)
	}
	return result, nil
}

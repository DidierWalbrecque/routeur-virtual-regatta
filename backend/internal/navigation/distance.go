package navigation

import (
	"fmt"
	"math"
)

const earthRadiusNauticalMiles = 3440.065

// DistanceBetween calcule la distance orthodromique entre deux positions avec
// la formule de Haversine. Le résultat est exprimé en milles nautiques.
func DistanceBetween(
	from Position,
	to Position,
) (NauticalMiles, error) {
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
	latitudeDelta := toLatitude - fromLatitude
	longitudeDelta := degreesToRadians(
		float64(to.Longitude - from.Longitude),
	)

	halfLatitudeSine := math.Sin(latitudeDelta / 2)
	halfLongitudeSine := math.Sin(longitudeDelta / 2)

	haversine := halfLatitudeSine*halfLatitudeSine +
		math.Cos(fromLatitude)*math.Cos(toLatitude)*
			halfLongitudeSine*halfLongitudeSine

	// Les arrondis flottants peuvent produire une valeur très légèrement hors
	// de l'intervalle mathématique [0, 1].
	haversine = math.Max(0, math.Min(1, haversine))

	centralAngle := 2 * math.Atan2(
		math.Sqrt(haversine),
		math.Sqrt(1-haversine),
	)

	return NauticalMiles(earthRadiusNauticalMiles * centralAngle), nil
}

func degreesToRadians(degrees float64) float64 {
	return degrees * math.Pi / 180
}

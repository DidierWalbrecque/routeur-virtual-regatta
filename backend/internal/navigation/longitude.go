package navigation

import (
	"fmt"
	"math"
)

// Validate vérifie que la longitude est finie et appartient à [-180, 180].
func (longitude Longitude) Validate() error {
	value := float64(longitude)
	if math.IsNaN(value) || math.IsInf(value, 0) ||
		longitude < -180 || longitude > 180 {
		return fmt.Errorf(
			"navigation: la longitude doit être un nombre fini compris entre -180 et 180 degrés",
		)
	}
	return nil
}

// NormalizeLongitude ramène toute longitude finie dans l'intervalle canonique
// [-180, 180[. Les valeurs -180 et 180 représentent le même méridien.
func NormalizeLongitude(degrees float64) (Longitude, error) {
	if math.IsNaN(degrees) || math.IsInf(degrees, 0) {
		return 0, fmt.Errorf(
			"navigation: la longitude à normaliser doit être un nombre fini",
		)
	}

	normalized := math.Mod(degrees+180, 360)
	if normalized < 0 {
		normalized += 360
	}
	longitude := Longitude(normalized - 180)
	if err := longitude.Validate(); err != nil {
		return 0, fmt.Errorf(
			"navigation: longitude normalisée invalide: %w",
			err,
		)
	}
	return longitude, nil
}

// ShortestLongitudeDelta calcule l'écart signé le plus court entre deux
// longitudes. Le résultat appartient à [-180, 180[ : une valeur positive va
// vers l'est et une valeur négative vers l'ouest.
func ShortestLongitudeDelta(
	from Longitude,
	to Longitude,
) (float64, error) {
	if err := from.Validate(); err != nil {
		return 0, fmt.Errorf(
			"navigation: longitude de départ invalide: %w",
			err,
		)
	}
	if err := to.Validate(); err != nil {
		return 0, fmt.Errorf(
			"navigation: longitude d'arrivée invalide: %w",
			err,
		)
	}

	delta, err := NormalizeLongitude(float64(to - from))
	if err != nil {
		return 0, fmt.Errorf(
			"navigation: calcul de l'écart de longitude impossible: %w",
			err,
		)
	}
	return float64(delta), nil
}

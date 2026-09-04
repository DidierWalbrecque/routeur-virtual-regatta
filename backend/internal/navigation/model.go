// Package navigation contient les calculs géographiques déterministes du
// moteur de routage.
package navigation

import (
	"fmt"
	"math"
)

// Latitude représente une latitude en degrés, comprise entre -90 et 90.
type Latitude float64

// Longitude représente une longitude en degrés, comprise entre -180 et 180.
type Longitude float64

// NauticalMiles représente une distance en milles nautiques.
type NauticalMiles float64

// Heading représente un cap en degrés depuis le nord, dans le sens horaire.
type Heading float64

// Validate vérifie que le cap est fini et appartient à [0, 360[.
func (heading Heading) Validate() error {
	numericHeading := float64(heading)
	if math.IsNaN(numericHeading) || math.IsInf(numericHeading, 0) ||
		heading < 0 || heading >= 360 {
		return fmt.Errorf(
			"navigation: le cap doit être un nombre fini compris entre 0 inclus et 360 degrés exclus",
		)
	}
	return nil
}

// Position représente un point géographique en degrés décimaux.
type Position struct {
	Latitude  Latitude
	Longitude Longitude
}

// Validate vérifie que la position contient des coordonnées finies et valides.
func (position Position) Validate() error {
	latitude := float64(position.Latitude)
	if math.IsNaN(latitude) || math.IsInf(latitude, 0) ||
		position.Latitude < -90 || position.Latitude > 90 {
		return fmt.Errorf(
			"navigation: la latitude doit être un nombre fini compris entre -90 et 90 degrés",
		)
	}

	longitude := float64(position.Longitude)
	if math.IsNaN(longitude) || math.IsInf(longitude, 0) ||
		position.Longitude < -180 || position.Longitude > 180 {
		return fmt.Errorf(
			"navigation: la longitude doit être un nombre fini compris entre -180 et 180 degrés",
		)
	}

	return nil
}

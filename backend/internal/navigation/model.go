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

// Validate vérifie que la distance est finie et positive ou nulle.
func (distance NauticalMiles) Validate() error {
	numericDistance := float64(distance)
	if math.IsNaN(numericDistance) || math.IsInf(numericDistance, 0) ||
		distance < 0 {
		return fmt.Errorf(
			"navigation: la distance doit être un nombre fini positif ou nul",
		)
	}
	return nil
}

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

	if err := position.Longitude.Validate(); err != nil {
		return err
	}

	return nil
}

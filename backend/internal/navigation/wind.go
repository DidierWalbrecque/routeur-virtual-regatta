package navigation

import (
	"fmt"
	"math"
)

// WindComponent représente une composante horizontale du vent en mètres par
// seconde. Elle est signée : U est positif vers l'est et V vers le nord.
type WindComponent float64

// WindDirection représente la direction météorologique d'où vient le vent,
// en degrés depuis le nord dans le sens horaire, dans [0, 360[.
type WindDirection float64

// Wind représente la vitesse et la direction météorologique du vent.
type Wind struct {
	Speed     MetersPerSecond
	Direction WindDirection
}

// Validate vérifie que la composante du vent est finie.
func (component WindComponent) Validate() error {
	value := float64(component)
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return fmt.Errorf(
			"navigation: la composante du vent doit être un nombre fini",
		)
	}
	return nil
}

// Validate vérifie que la direction du vent est finie et appartient à
// [0, 360[.
func (direction WindDirection) Validate() error {
	value := float64(direction)
	if math.IsNaN(value) || math.IsInf(value, 0) ||
		direction < 0 || direction >= 360 {
		return fmt.Errorf(
			"navigation: la direction du vent doit être un nombre fini compris entre 0 inclus et 360 degrés exclus",
		)
	}
	return nil
}

// Validate vérifie que la vitesse et la direction du vent sont valides.
func (wind Wind) Validate() error {
	if err := wind.Speed.Validate(); err != nil {
		return fmt.Errorf("navigation: vent invalide: %w", err)
	}
	if err := wind.Direction.Validate(); err != nil {
		return fmt.Errorf("navigation: vent invalide: %w", err)
	}
	return nil
}

// WindFromComponents calcule la vitesse et la direction météorologique du vent
// à partir des composantes NOAA U et V en mètres par seconde.
//
// U est positif vers l'est et V vers le nord. La direction indique d'où vient
// le vent. Pour un vent nul, elle vaut conventionnellement 0 degré.
func WindFromComponents(u, v WindComponent) (Wind, error) {
	if err := u.Validate(); err != nil {
		return Wind{}, fmt.Errorf(
			"navigation: composante U invalide: %w",
			err,
		)
	}
	if err := v.Validate(); err != nil {
		return Wind{}, fmt.Errorf(
			"navigation: composante V invalide: %w",
			err,
		)
	}

	speed := MetersPerSecond(math.Hypot(float64(u), float64(v)))
	direction := WindDirection(0)
	if speed != 0 {
		degrees := math.Atan2(-float64(u), -float64(v)) * 180 / math.Pi
		if degrees < 0 {
			degrees += 360
		}
		direction = WindDirection(degrees)
	}

	wind := Wind{
		Speed:     speed,
		Direction: direction,
	}
	if err := wind.Validate(); err != nil {
		return Wind{}, fmt.Errorf(
			"navigation: calcul du vent impossible: %w",
			err,
		)
	}
	return wind, nil
}

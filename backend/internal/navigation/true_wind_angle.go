package navigation

import (
	"fmt"
	"math"
)

// TrueWindAngle représente l'angle absolu entre le cap du bateau et la
// direction d'où vient le vent, compris entre 0 et 180 degrés.
type TrueWindAngle float64

// Validate vérifie que le TWA est fini et appartient à [0, 180].
func (angle TrueWindAngle) Validate() error {
	value := float64(angle)
	if math.IsNaN(value) || math.IsInf(value, 0) ||
		angle < 0 || angle > 180 {
		return fmt.Errorf(
			"navigation: le TWA doit être un nombre fini compris entre 0 et 180 degrés",
		)
	}
	return nil
}

// TrueWindAngleFromHeading calcule le TWA à partir du cap du bateau et de la
// direction météorologique d'où vient le vent.
func TrueWindAngleFromHeading(
	heading Heading,
	windDirection WindDirection,
) (TrueWindAngle, error) {
	relativeAngle, err := signedWindAngle(heading, windDirection)
	if err != nil {
		return 0, fmt.Errorf(
			"navigation: calcul du TWA impossible: %w",
			err,
		)
	}

	angle := TrueWindAngle(math.Abs(relativeAngle))
	if err := angle.Validate(); err != nil {
		return 0, fmt.Errorf(
			"navigation: TWA calculé invalide: %w",
			err,
		)
	}
	return angle, nil
}

// signedWindAngle calcule l'angle signé le plus court entre le cap et
// l'origine du vent. Une valeur négative place le vent à bâbord et une valeur
// positive à tribord. Le résultat appartient à [-180, 180[.
func signedWindAngle(
	heading Heading,
	windDirection WindDirection,
) (float64, error) {
	if err := heading.Validate(); err != nil {
		return 0, fmt.Errorf("cap invalide: %w", err)
	}
	if err := windDirection.Validate(); err != nil {
		return 0, fmt.Errorf("direction du vent invalide: %w", err)
	}

	angle := math.Mod(
		float64(windDirection)-float64(heading)+180,
		360,
	)
	if angle < 0 {
		angle += 360
	}
	return angle - 180, nil
}

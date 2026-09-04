package navigation

import (
	"fmt"
	"math"
)

// Tack représente le côté du bateau d'où vient le vent.
type Tack string

const (
	// TackUndefined indique que l'amure ne peut pas être déterminée, notamment
	// lorsque le vent vient exactement de face ou de l'arrière.
	TackUndefined Tack = "undefined"

	// TackPort indique que le vent vient de bâbord.
	TackPort Tack = "port"

	// TackStarboard indique que le vent vient de tribord.
	TackStarboard Tack = "starboard"
)

// Validate vérifie que l'amure est une valeur connue.
func (tack Tack) Validate() error {
	switch tack {
	case TackUndefined, TackPort, TackStarboard:
		return nil
	default:
		return fmt.Errorf("navigation: amure inconnue %q", tack)
	}
}

// TackFromHeading identifie l'amure à partir du cap du bateau et de la
// direction météorologique d'où vient le vent.
//
// Lorsque le vent vient exactement de face ou de l'arrière, aucune amure ne
// peut être déduite de ces deux valeurs seules.
func TackFromHeading(
	heading Heading,
	windDirection WindDirection,
) (Tack, error) {
	relativeAngle, err := signedWindAngle(heading, windDirection)
	if err != nil {
		return TackUndefined, fmt.Errorf(
			"navigation: identification de l'amure impossible: %w",
			err,
		)
	}

	tack := TackStarboard
	switch {
	case relativeAngle == 0 || math.Abs(relativeAngle) == 180:
		tack = TackUndefined
	case relativeAngle < 0:
		tack = TackPort
	}

	if err := tack.Validate(); err != nil {
		return TackUndefined, fmt.Errorf(
			"navigation: amure calculée invalide: %w",
			err,
		)
	}
	return tack, nil
}

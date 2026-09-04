package equipment

import (
	"fmt"
	"strings"
)

// Parse transforme l'identifiant textuel stable d'un équipement en valeur
// métier.
func Parse(value string) (Equipment, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "light":
		return Light, nil
	case "heavy":
		return Heavy, nil
	case "code0":
		return Code0, nil
	case "foils":
		return Foils, nil
	case "hull_polish":
		return HullPolish, nil
	case "pro_winches":
		return ProWinches, nil
	default:
		return 0, fmt.Errorf(
			"equipment: option %q inconnue; valeurs autorisées: light, heavy, code0, foils, hull_polish, pro_winches",
			value,
		)
	}
}

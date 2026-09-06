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


// ParseSet transforme une liste d'identifiants séparés par des virgules en
// ensemble d'équipements. Une chaîne vide représente l'équipement de base.
func ParseSet(value string) (EquipmentSet, error) {
	if strings.TrimSpace(value) == "" {
		return NewSet()
	}

	parts := strings.Split(value, ",")
	items := make([]Equipment, 0, len(parts))
	for _, part := range parts {
		item, err := Parse(part)
		if err != nil {
			return 0, fmt.Errorf(
				"equipment: lecture de la liste impossible: %w",
				err,
			)
		}
		items = append(items, item)
	}
	return NewSet(items...)
}

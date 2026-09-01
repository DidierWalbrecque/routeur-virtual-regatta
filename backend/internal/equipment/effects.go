package equipment

import (
	"fmt"
	"sort"
	"strings"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
)

var baseSailNames = []string{
	"Jib",
	"Spi",
}

var equipmentSailNames = map[Equipment][]string{
	Light: {
		"LightJib",
		"LightGnk",
	},
	Heavy: {
		"Staysail",
		"HeavyGnk",
	},
	Code0: {
		"Code0",
	},
}

// Effects contient les conséquences d'un ensemble d'équipements pour le moteur.
type Effects struct {
	AvailableSails []polar.SailID
	SpeedOptions   polar.SpeedOptions
	ProWinches     bool
}

// ResolveEffects transforme les options actives en voiles disponibles et en
// modificateurs de performance pour une polaire donnée.
func ResolveEffects(
	boatPolar *polar.Polar,
	equipments EquipmentSet,
) (Effects, error) {
	if boatPolar == nil {
		return Effects{}, fmt.Errorf(
			"equipment: résolution des effets impossible: la polaire est nil",
		)
	}
	if err := equipments.Validate(); err != nil {
		return Effects{}, fmt.Errorf(
			"equipment: résolution des effets impossible: %w",
			err,
		)
	}

	requiredNames := make(map[string]struct{})
	for _, name := range baseSailNames {
		requiredNames[name] = struct{}{}
	}
	for item, names := range equipmentSailNames {
		if !equipments.Has(item) {
			continue
		}
		for _, name := range names {
			requiredNames[name] = struct{}{}
		}
	}

	availableSails := make([]polar.SailID, 0, len(requiredNames))
	missingNames := make(map[string]struct{}, len(requiredNames))
	for name := range requiredNames {
		missingNames[name] = struct{}{}
	}

	// L'ordre du fichier de polaires est conservé pour rendre le départage des
	// vitesses égales déterministe.
	for _, sail := range boatPolar.Sails {
		if _, required := requiredNames[sail.Name]; !required {
			continue
		}
		availableSails = append(availableSails, sail.ID)
		delete(missingNames, sail.Name)
	}

	if len(missingNames) > 0 {
		missing := make([]string, 0, len(missingNames))
		for name := range missingNames {
			missing = append(missing, name)
		}
		sort.Strings(missing)

		return Effects{}, fmt.Errorf(
			"equipment: résolution des effets impossible: voiles attendues absentes de la polaire: %s",
			strings.Join(missing, ", "),
		)
	}

	return Effects{
		AvailableSails: availableSails,
		SpeedOptions: polar.SpeedOptions{
			Foils:      equipments.Has(Foils),
			HullPolish: equipments.Has(HullPolish),
		},
		ProWinches: equipments.Has(ProWinches),
	}, nil
}

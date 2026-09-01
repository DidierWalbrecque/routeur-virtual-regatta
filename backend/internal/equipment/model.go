// Package equipment contient le modèle des options de performance et du
// budget d'une course Virtual Regatta.
package equipment

import "fmt"

// Credits représente un montant entier de crédits Virtual Regatta.
type Credits int

// RaceCategory représente une catégorie de course comprise entre 1 et 6.
type RaceCategory uint8

const (
	Category1 RaceCategory = 1
	Category2 RaceCategory = 2
	Category3 RaceCategory = 3
	Category4 RaceCategory = 4
	Category5 RaceCategory = 5
	Category6 RaceCategory = 6
)

// CostMultiplier retourne le coefficient tarifaire 7 - catégorie.
func (category RaceCategory) CostMultiplier() (int, error) {
	if err := category.Validate(); err != nil {
		return 0, err
	}
	return 7 - int(category), nil
}

// Validate vérifie que la catégorie est comprise entre 1 et 6.
func (category RaceCategory) Validate() error {
	if category < Category1 || category > Category6 {
		return fmt.Errorf(
			"equipment: la catégorie de course doit être comprise entre 1 et 6",
		)
	}
	return nil
}

// Equipment représente une option de performance sous forme de drapeau binaire.
type Equipment uint8

const (
	Light Equipment = 1 << iota
	Heavy
	Code0
	Foils
	HullPolish
	ProWinches
)

// EquipmentSet regroupe zéro ou plusieurs options de performance.
type EquipmentSet uint8

const allEquipments EquipmentSet = EquipmentSet(
	Light | Heavy | Code0 | Foils | HullPolish | ProWinches,
)

// All retourne les six options de performance dans un ordre stable.
func All() []Equipment {
	return []Equipment{
		Light,
		Heavy,
		Code0,
		Foils,
		HullPolish,
		ProWinches,
	}
}

// NewSet construit un ensemble validé d'options de performance.
func NewSet(equipments ...Equipment) (EquipmentSet, error) {
	var result EquipmentSet
	for _, item := range equipments {
		if err := item.Validate(); err != nil {
			return 0, err
		}
		result |= EquipmentSet(item)
	}
	return result, nil
}

// Has indique si une option valide appartient à l'ensemble.
func (set EquipmentSet) Has(item Equipment) bool {
	if item.Validate() != nil {
		return false
	}
	return set&EquipmentSet(item) != 0
}

// Validate refuse les bits ne correspondant à aucune option connue.
func (set EquipmentSet) Validate() error {
	if set&^allEquipments != 0 {
		return fmt.Errorf(
			"equipment: l'ensemble contient une option inconnue: %08b",
			set&^allEquipments,
		)
	}
	return nil
}

// Validate vérifie qu'une valeur représente exactement une option connue.
func (item Equipment) Validate() error {
	switch item {
	case Light, Heavy, Code0, Foils, HullPolish, ProWinches:
		return nil
	default:
		return fmt.Errorf("equipment: option inconnue: %d", item)
	}
}

// String retourne l'identifiant stable d'une option.
func (item Equipment) String() string {
	switch item {
	case Light:
		return "light"
	case Heavy:
		return "heavy"
	case Code0:
		return "code0"
	case Foils:
		return "foils"
	case HullPolish:
		return "hull_polish"
	case ProWinches:
		return "pro_winches"
	default:
		return "unknown"
	}
}

// RaceBudget décrit le plafond total de dépenses d'une course.
// TotalLimit inclut le bonus de départ.
type RaceBudget struct {
	Category   RaceCategory
	TotalLimit Credits
}

// Validate vérifie la catégorie et le plafond total.
func (budget RaceBudget) Validate() error {
	if err := budget.Category.Validate(); err != nil {
		return err
	}
	if budget.TotalLimit < 0 {
		return fmt.Errorf(
			"equipment: le plafond total de crédits doit être positif ou nul",
		)
	}
	return nil
}

// RaceConfiguration contient les informations de course nécessaires à
// l'optimisation des équipements.
type RaceConfiguration struct {
	Budget RaceBudget
	Owned  EquipmentSet
}

// Validate vérifie le budget et les options déjà possédées.
func (configuration RaceConfiguration) Validate() error {
	if err := configuration.Budget.Validate(); err != nil {
		return err
	}
	if err := configuration.Owned.Validate(); err != nil {
		return err
	}
	return nil
}

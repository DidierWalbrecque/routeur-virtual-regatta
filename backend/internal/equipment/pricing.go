package equipment

import "fmt"

const baseStartBonus Credits = 400

var baseCosts = map[Equipment]Credits{
	Light:      300,
	Heavy:      240,
	Code0:      340,
	Foils:      440,
	HullPolish: 140,
	ProWinches: 200,
}

// CostBreakdown distingue le coût total, le bonus consommé et les crédits
// personnels nécessaires.
type CostBreakdown struct {
	TotalCost       Credits
	StartBonusUsed  Credits
	PersonalCredits Credits
}

// BaseCost retourne le prix d'une option pour une course de catégorie 6.
func BaseCost(item Equipment) (Credits, error) {
	if err := item.Validate(); err != nil {
		return 0, err
	}

	cost, exists := baseCosts[item]
	if !exists {
		return 0, fmt.Errorf(
			"equipment: aucun tarif de base pour l'option %q",
			item.String(),
		)
	}
	return cost, nil
}

// CostAt retourne le prix d'une option pour une catégorie de course.
func CostAt(category RaceCategory, item Equipment) (Credits, error) {
	multiplier, err := category.CostMultiplier()
	if err != nil {
		return 0, err
	}
	baseCost, err := BaseCost(item)
	if err != nil {
		return 0, err
	}
	return baseCost * Credits(multiplier), nil
}

// StartBonusAt retourne le bonus de départ d'une catégorie de course.
func StartBonusAt(category RaceCategory) (Credits, error) {
	multiplier, err := category.CostMultiplier()
	if err != nil {
		return 0, err
	}
	return baseStartBonus * Credits(multiplier), nil
}

// BreakdownAt calcule le coût d'un ensemble d'options pour une catégorie.
func BreakdownAt(
	category RaceCategory,
	equipments EquipmentSet,
) (CostBreakdown, error) {
	if err := equipments.Validate(); err != nil {
		return CostBreakdown{}, err
	}

	var total Credits
	for _, item := range All() {
		if !equipments.Has(item) {
			continue
		}
		cost, err := CostAt(category, item)
		if err != nil {
			return CostBreakdown{}, err
		}
		total += cost
	}

	bonus, err := StartBonusAt(category)
	if err != nil {
		return CostBreakdown{}, err
	}

	bonusUsed := bonus
	if total < bonusUsed {
		bonusUsed = total
	}

	return CostBreakdown{
		TotalCost:       total,
		StartBonusUsed:  bonusUsed,
		PersonalCredits: total - bonusUsed,
	}, nil
}

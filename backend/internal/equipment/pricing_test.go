package equipment

import "testing"

func TestPricingMatchesReferenceTable(t *testing.T) {
	tests := []struct {
		category RaceCategory
		bonus    Credits
		costs    map[Equipment]Credits
	}{
		{
			category: Category6,
			bonus:    400,
			costs: map[Equipment]Credits{
				Light: 300, Heavy: 240, Code0: 340,
				Foils: 440, HullPolish: 140, ProWinches: 200,
			},
		},
		{
			category: Category5,
			bonus:    800,
			costs: map[Equipment]Credits{
				Light: 600, Heavy: 480, Code0: 680,
				Foils: 880, HullPolish: 280, ProWinches: 400,
			},
		},
		{
			category: Category4,
			bonus:    1200,
			costs: map[Equipment]Credits{
				Light: 900, Heavy: 720, Code0: 1020,
				Foils: 1320, HullPolish: 420, ProWinches: 600,
			},
		},
		{
			category: Category3,
			bonus:    1600,
			costs: map[Equipment]Credits{
				Light: 1200, Heavy: 960, Code0: 1360,
				Foils: 1760, HullPolish: 560, ProWinches: 800,
			},
		},
		{
			category: Category2,
			bonus:    2000,
			costs: map[Equipment]Credits{
				Light: 1500, Heavy: 1200, Code0: 1700,
				Foils: 2200, HullPolish: 700, ProWinches: 1000,
			},
		},
		{
			category: Category1,
			bonus:    2400,
			costs: map[Equipment]Credits{
				Light: 1800, Heavy: 1440, Code0: 2040,
				Foils: 2640, HullPolish: 840, ProWinches: 1200,
			},
		},
	}

	for _, test := range tests {
		gotBonus, err := StartBonusAt(test.category)
		if err != nil {
			t.Fatalf(
				"StartBonusAt(%d) a retourné une erreur inattendue: %v",
				test.category,
				err,
			)
		}
		if gotBonus != test.bonus {
			t.Errorf(
				"StartBonusAt(%d) = %d, attendu %d",
				test.category,
				gotBonus,
				test.bonus,
			)
		}

		for item, want := range test.costs {
			got, costErr := CostAt(test.category, item)
			if costErr != nil {
				t.Fatalf(
					"CostAt(%d, %q) a retourné une erreur inattendue: %v",
					test.category,
					item.String(),
					costErr,
				)
			}
			if got != want {
				t.Errorf(
					"CostAt(%d, %q) = %d, attendu %d",
					test.category,
					item.String(),
					got,
					want,
				)
			}
		}
	}
}

func TestFullEquipmentBreakdownMatchesReferenceTable(t *testing.T) {
	fullSet, err := NewSet(All()...)
	if err != nil {
		t.Fatalf("NewSet(All()...) a retourné une erreur inattendue: %v", err)
	}

	tests := []struct {
		category RaceCategory
		total    Credits
		bonus    Credits
		personal Credits
	}{
		{category: Category6, total: 1660, bonus: 400, personal: 1260},
		{category: Category5, total: 3320, bonus: 800, personal: 2520},
		{category: Category4, total: 4980, bonus: 1200, personal: 3780},
		{category: Category3, total: 6640, bonus: 1600, personal: 5040},
		{category: Category2, total: 8300, bonus: 2000, personal: 6300},
		{category: Category1, total: 9960, bonus: 2400, personal: 7560},
	}

	for _, test := range tests {
		got, breakdownErr := BreakdownAt(test.category, fullSet)
		if breakdownErr != nil {
			t.Fatalf(
				"BreakdownAt(%d) a retourné une erreur inattendue: %v",
				test.category,
				breakdownErr,
			)
		}

		if got.TotalCost != test.total {
			t.Errorf(
				"TotalCost catégorie %d = %d, attendu %d",
				test.category,
				got.TotalCost,
				test.total,
			)
		}
		if got.StartBonusUsed != test.bonus {
			t.Errorf(
				"StartBonusUsed catégorie %d = %d, attendu %d",
				test.category,
				got.StartBonusUsed,
				test.bonus,
			)
		}
		if got.PersonalCredits != test.personal {
			t.Errorf(
				"PersonalCredits catégorie %d = %d, attendu %d",
				test.category,
				got.PersonalCredits,
				test.personal,
			)
		}
	}
}

func TestBreakdownUsesOnlyNecessaryBonus(t *testing.T) {
	set, err := NewSet(HullPolish)
	if err != nil {
		t.Fatalf("NewSet() a retourné une erreur inattendue: %v", err)
	}

	got, err := BreakdownAt(Category6, set)
	if err != nil {
		t.Fatalf("BreakdownAt() a retourné une erreur inattendue: %v", err)
	}

	if got.TotalCost != 140 || got.StartBonusUsed != 140 || got.PersonalCredits != 0 {
		t.Errorf("BreakdownAt() = %+v, attendu coût 140 entièrement couvert", got)
	}
}

func TestPricingRejectsInvalidInputs(t *testing.T) {
	if _, err := CostAt(0, Light); err == nil {
		t.Fatal("CostAt() doit refuser une catégorie invalide")
	}
	if _, err := CostAt(Category6, Equipment(1<<7)); err == nil {
		t.Fatal("CostAt() doit refuser une option inconnue")
	}
	if _, err := BreakdownAt(Category6, EquipmentSet(1<<7)); err == nil {
		t.Fatal("BreakdownAt() doit refuser un ensemble inconnu")
	}
}

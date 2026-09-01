package equipment

import (
	"strings"
	"testing"
)

func TestRaceCategoryCostMultiplier(t *testing.T) {
	tests := []struct {
		category RaceCategory
		want     int
	}{
		{category: Category6, want: 1},
		{category: Category5, want: 2},
		{category: Category4, want: 3},
		{category: Category3, want: 4},
		{category: Category2, want: 5},
		{category: Category1, want: 6},
	}

	for _, test := range tests {
		got, err := test.category.CostMultiplier()
		if err != nil {
			t.Fatalf(
				"CostMultiplier(%d) a retourné une erreur inattendue: %v",
				test.category,
				err,
			)
		}
		if got != test.want {
			t.Errorf(
				"CostMultiplier(%d) = %d, attendu %d",
				test.category,
				got,
				test.want,
			)
		}
	}
}

func TestRaceCategoryRejectsInvalidValues(t *testing.T) {
	for _, category := range []RaceCategory{0, 7} {
		if _, err := category.CostMultiplier(); err == nil {
			t.Errorf("CostMultiplier(%d) doit refuser cette catégorie", category)
		}
	}
}

func TestEquipmentSet(t *testing.T) {
	set, err := NewSet(Light, Code0, Foils, Code0)
	if err != nil {
		t.Fatalf("NewSet() a retourné une erreur inattendue: %v", err)
	}

	for _, item := range []Equipment{Light, Code0, Foils} {
		if !set.Has(item) {
			t.Errorf("l'ensemble doit contenir %q", item.String())
		}
	}
	for _, item := range []Equipment{Heavy, HullPolish, ProWinches} {
		if set.Has(item) {
			t.Errorf("l'ensemble ne doit pas contenir %q", item.String())
		}
	}
}

func TestEquipmentSetRejectsUnknownOptions(t *testing.T) {
	unknown := Equipment(1 << 7)

	if _, err := NewSet(unknown); err == nil {
		t.Fatal("NewSet() doit refuser une option inconnue")
	}

	set := EquipmentSet(unknown)
	if err := set.Validate(); err == nil {
		t.Fatal("EquipmentSet.Validate() doit refuser un bit inconnu")
	}
}

func TestRaceConfigurationValidate(t *testing.T) {
	owned, err := NewSet(Light, HullPolish)
	if err != nil {
		t.Fatalf("NewSet() a retourné une erreur inattendue: %v", err)
	}

	configuration := RaceConfiguration{
		Budget: RaceBudget{
			Category:   Category3,
			TotalLimit: 5000,
		},
		Owned: owned,
	}
	if err := configuration.Validate(); err != nil {
		t.Fatalf("Validate() a retourné une erreur inattendue: %v", err)
	}
}

func TestRaceConfigurationRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name          string
		configuration RaceConfiguration
		errorPart     string
	}{
		{
			name: "catégorie invalide",
			configuration: RaceConfiguration{
				Budget: RaceBudget{Category: 0, TotalLimit: 1000},
			},
			errorPart: "catégorie",
		},
		{
			name: "plafond négatif",
			configuration: RaceConfiguration{
				Budget: RaceBudget{Category: Category6, TotalLimit: -1},
			},
			errorPart: "plafond total",
		},
		{
			name: "option possédée inconnue",
			configuration: RaceConfiguration{
				Budget: RaceBudget{Category: Category6, TotalLimit: 1000},
				Owned:  EquipmentSet(1 << 7),
			},
			errorPart: "option inconnue",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.configuration.Validate()
			if err == nil {
				t.Fatal("Validate() doit refuser cette configuration")
			}
			if !strings.Contains(err.Error(), test.errorPart) {
				t.Errorf("erreur = %q, doit contenir %q", err, test.errorPart)
			}
		})
	}
}

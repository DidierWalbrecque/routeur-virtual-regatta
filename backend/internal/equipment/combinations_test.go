package equipment

import (
	"strings"
	"testing"
)

func TestGenerateAffordableCombinationsReturnsAll64Options(t *testing.T) {
	configuration := RaceConfiguration{
		Budget: RaceBudget{
			Category:   Category6,
			TotalLimit: 1660,
		},
	}

	got, err := GenerateAffordableCombinations(configuration)
	if err != nil {
		t.Fatalf(
			"GenerateAffordableCombinations() a retourné une erreur inattendue: %v",
			err,
		)
	}
	if len(got) != 64 {
		t.Fatalf("nombre de combinaisons = %d, attendu 64", len(got))
	}

	seen := make(map[EquipmentSet]struct{}, len(got))
	for _, combination := range got {
		if _, duplicate := seen[combination.Active]; duplicate {
			t.Errorf("combinaison dupliquée: %08b", combination.Active)
		}
		seen[combination.Active] = struct{}{}

		if combination.Cost.TotalCost > configuration.Budget.TotalLimit {
			t.Errorf(
				"la combinaison %08b coûte %d, plafond %d",
				combination.Active,
				combination.Cost.TotalCost,
				configuration.Budget.TotalLimit,
			)
		}
	}

	fullSet, err := NewSet(All()...)
	if err != nil {
		t.Fatalf("NewSet(All()...) a retourné une erreur inattendue: %v", err)
	}
	full := findCombination(t, got, fullSet)
	if full.Cost.TotalCost != 1660 {
		t.Errorf("coût total = %d, attendu 1660", full.Cost.TotalCost)
	}
	if full.Cost.StartBonusUsed != 400 {
		t.Errorf("bonus utilisé = %d, attendu 400", full.Cost.StartBonusUsed)
	}
	if full.Cost.PersonalCredits != 1260 {
		t.Errorf(
			"crédits personnels = %d, attendu 1260",
			full.Cost.PersonalCredits,
		)
	}
	if full.PurchaseCost != 1660 || full.AdditionalPersonalCredits != 1260 {
		t.Errorf("coûts d'achat inattendus: %+v", full)
	}
}

func TestGenerateAffordableCombinationsFiltersBudget(t *testing.T) {
	configuration := RaceConfiguration{
		Budget: RaceBudget{
			Category:   Category6,
			TotalLimit: 300,
		},
	}

	got, err := GenerateAffordableCombinations(configuration)
	if err != nil {
		t.Fatalf(
			"GenerateAffordableCombinations() a retourné une erreur inattendue: %v",
			err,
		)
	}

	// Configurations autorisées : aucune option, Light, Heavy, polish ou
	// winchs pro. Toute combinaison de deux options dépasse 300 crédits.
	if len(got) != 5 {
		t.Fatalf("nombre de combinaisons = %d, attendu 5", len(got))
	}

	for _, expected := range []EquipmentSet{
		0,
		EquipmentSet(Light),
		EquipmentSet(Heavy),
		EquipmentSet(HullPolish),
		EquipmentSet(ProWinches),
	} {
		findCombination(t, got, expected)
	}
}

func TestGenerateAffordableCombinationsHandlesOwnedEquipment(t *testing.T) {
	owned, err := NewSet(HullPolish)
	if err != nil {
		t.Fatalf("NewSet() a retourné une erreur inattendue: %v", err)
	}

	configuration := RaceConfiguration{
		Budget: RaceBudget{
			Category:   Category6,
			TotalLimit: 600,
		},
		Owned: owned,
	}

	got, err := GenerateAffordableCombinations(configuration)
	if err != nil {
		t.Fatalf(
			"GenerateAffordableCombinations() a retourné une erreur inattendue: %v",
			err,
		)
	}

	for _, combination := range got {
		if !combination.Active.Has(HullPolish) {
			t.Errorf(
				"la combinaison %08b doit conserver le polish possédé",
				combination.Active,
			)
		}
		if combination.ToPurchase.Has(HullPolish) {
			t.Errorf(
				"la combinaison %08b ne doit pas racheter le polish",
				combination.Active,
			)
		}
	}

	baseline := findCombination(t, got, owned)
	if baseline.PurchaseCost != 0 || baseline.AdditionalPersonalCredits != 0 {
		t.Errorf("la configuration déjà possédée ne doit rien coûter: %+v", baseline)
	}

	withFoils := findCombination(
		t,
		got,
		owned|EquipmentSet(Foils),
	)
	if withFoils.PurchaseCost != 440 {
		t.Errorf("PurchaseCost = %d, attendu 440", withFoils.PurchaseCost)
	}
	if withFoils.AdditionalPersonalCredits != 180 {
		t.Errorf(
			"AdditionalPersonalCredits = %d, attendu 180",
			withFoils.AdditionalPersonalCredits,
		)
	}
}

func TestGenerateAffordableCombinationsRejectsOwnedEquipmentOverBudget(
	t *testing.T,
) {
	owned, err := NewSet(Light)
	if err != nil {
		t.Fatalf("NewSet() a retourné une erreur inattendue: %v", err)
	}

	_, err = GenerateAffordableCombinations(RaceConfiguration{
		Budget: RaceBudget{
			Category:   Category6,
			TotalLimit: 200,
		},
		Owned: owned,
	})
	if err == nil {
		t.Fatal("les équipements possédés dépassant le plafond doivent être refusés")
	}
	if !strings.Contains(err.Error(), "dépassent le plafond") {
		t.Errorf("erreur inattendue: %v", err)
	}
}

func TestGenerateAffordableCombinationsRejectsInvalidConfiguration(
	t *testing.T,
) {
	_, err := GenerateAffordableCombinations(RaceConfiguration{
		Budget: RaceBudget{
			Category:   0,
			TotalLimit: 1000,
		},
	})
	if err == nil {
		t.Fatal("une catégorie invalide doit être refusée")
	}
	if !strings.Contains(err.Error(), "catégorie") {
		t.Errorf("erreur inattendue: %v", err)
	}
}

func findCombination(
	t *testing.T,
	combinations []Combination,
	active EquipmentSet,
) Combination {
	t.Helper()

	for _, combination := range combinations {
		if combination.Active == active {
			return combination
		}
	}

	t.Fatalf("combinaison %08b introuvable", active)
	return Combination{}
}

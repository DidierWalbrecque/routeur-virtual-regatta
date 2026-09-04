package equipment

import (
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
)

func TestRankNextPurchasesAtWithUltimSample(t *testing.T) {
	boatPolar := loadUltimForPurchaseTest(t)

	got, err := RankNextPurchasesAt(
		boatPolar,
		RaceConfiguration{
			Budget: RaceBudget{
				Category:   Category6,
				TotalLimit: 440,
			},
		},
		20,
		90,
	)
	if err != nil {
		t.Fatalf("RankNextPurchasesAt() a retourné une erreur inattendue: %v", err)
	}
	if len(got) != 6 {
		t.Fatalf("nombre d'achats = %d, attendu 6", len(got))
	}

	assertPurchase(t, got[0], Code0, 340, 0, 36.8, 1.84)
	assertPurchase(t, got[1], Foils, 440, 40, 36.3584, 1.3984)
	assertPurchase(t, got[2], HullPolish, 140, 0, 35.06488, 0.10488)

	var proWinches *PurchaseEvaluation
	for index := range got {
		if got[index].Equipment == ProWinches {
			proWinches = &got[index]
			break
		}
	}
	if proWinches == nil {
		t.Fatal("les winchs pro abordables doivent être évalués")
	}
	if proWinches.SpeedGain != 0 {
		t.Errorf(
			"gain instantané des winchs pro = %v, attendu 0",
			proWinches.SpeedGain,
		)
	}
}

func TestRankNextPurchasesAtUsesOwnedEquipment(t *testing.T) {
	boatPolar := loadUltimForPurchaseTest(t)
	owned, err := NewSet(Code0)
	if err != nil {
		t.Fatalf("NewSet() a retourné une erreur inattendue: %v", err)
	}

	got, err := RankNextPurchasesAt(
		boatPolar,
		RaceConfiguration{
			Budget: RaceBudget{
				Category:   Category6,
				TotalLimit: 780,
			},
			Owned: owned,
		},
		20,
		90,
	)
	if err != nil {
		t.Fatalf("RankNextPurchasesAt() a retourné une erreur inattendue: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("nombre d'achats = %d, attendu 5", len(got))
	}

	assertPurchase(t, got[0], Foils, 440, 380, 38.272, 1.472)
	for _, evaluation := range got {
		if evaluation.Equipment == Code0 {
			t.Error("Code0 déjà possédé ne doit pas être proposé à l'achat")
		}
		if !evaluation.Active.Has(Code0) {
			t.Errorf(
				"la combinaison %08b doit conserver Code0",
				evaluation.Active,
			)
		}
	}
}

func TestRankNextPurchasesAtReturnsEmptyWhenNothingIsAffordable(t *testing.T) {
	got, err := RankNextPurchasesAt(
		loadUltimForPurchaseTest(t),
		RaceConfiguration{
			Budget: RaceBudget{
				Category:   Category6,
				TotalLimit: 100,
			},
		},
		20,
		90,
	)
	if err != nil {
		t.Fatalf("RankNextPurchasesAt() a retourné une erreur inattendue: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("nombre d'achats = %d, attendu 0", len(got))
	}
}

func TestRankNextPurchasesAtReturnsEmptyWhenEverythingIsOwned(t *testing.T) {
	owned, err := NewSet(All()...)
	if err != nil {
		t.Fatalf("NewSet(All()...) a retourné une erreur inattendue: %v", err)
	}

	got, err := RankNextPurchasesAt(
		loadUltimForPurchaseTest(t),
		RaceConfiguration{
			Budget: RaceBudget{
				Category:   Category6,
				TotalLimit: 1660,
			},
			Owned: owned,
		},
		20,
		90,
	)
	if err != nil {
		t.Fatalf("RankNextPurchasesAt() a retourné une erreur inattendue: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("nombre d'achats = %d, attendu 0", len(got))
	}
}

func TestRankNextPurchasesAtPreservesErrors(t *testing.T) {
	_, err := RankNextPurchasesAt(
		nil,
		RaceConfiguration{
			Budget: RaceBudget{
				Category:   Category6,
				TotalLimit: 440,
			},
		},
		20,
		90,
	)
	if err == nil {
		t.Fatal("une polaire nil doit être refusée")
	}
	if !strings.Contains(err.Error(), "polaire est nil") {
		t.Errorf("erreur inattendue: %v", err)
	}
}

func assertPurchase(
	t *testing.T,
	got PurchaseEvaluation,
	wantEquipment Equipment,
	wantPurchaseCost Credits,
	wantAdditionalPersonal Credits,
	wantSpeed polar.BoatSpeed,
	wantGain polar.BoatSpeed,
) {
	t.Helper()

	if got.Equipment != wantEquipment {
		t.Errorf(
			"équipement = %q, attendu %q",
			got.Equipment.String(),
			wantEquipment.String(),
		)
	}
	if got.PurchaseCost != wantPurchaseCost {
		t.Errorf(
			"PurchaseCost pour %q = %d, attendu %d",
			got.Equipment.String(),
			got.PurchaseCost,
			wantPurchaseCost,
		)
	}
	if got.AdditionalPersonalCredits != wantAdditionalPersonal {
		t.Errorf(
			"AdditionalPersonalCredits pour %q = %d, attendu %d",
			got.Equipment.String(),
			got.AdditionalPersonalCredits,
			wantAdditionalPersonal,
		)
	}
	if difference := math.Abs(float64(got.BestSail.Speed - wantSpeed)); difference > 1e-9 {
		t.Errorf(
			"vitesse pour %q = %.12f, attendu %.12f",
			got.Equipment.String(),
			got.BestSail.Speed,
			wantSpeed,
		)
	}
	if difference := math.Abs(float64(got.SpeedGain - wantGain)); difference > 1e-9 {
		t.Errorf(
			"gain pour %q = %.12f, attendu %.12f",
			got.Equipment.String(),
			got.SpeedGain,
			wantGain,
		)
	}
}

func loadUltimForPurchaseTest(t *testing.T) *polar.Polar {
	t.Helper()

	path := filepath.Join("..", "..", "..", "data", "polars", "ultim_bp_11.json")
	boatPolar, err := polar.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() n'a pas chargé la polaire Ultim: %v", err)
	}
	return boatPolar
}

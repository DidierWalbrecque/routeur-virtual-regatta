package equipment

import (
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
)

func TestRankAffordableAtWithUltimSample(t *testing.T) {
	path := filepath.Join("..", "..", "..", "data", "polars", "ultim_bp_11.json")
	boatPolar, err := polar.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() n'a pas chargé la polaire Ultim: %v", err)
	}

	configuration := RaceConfiguration{
		Budget: RaceBudget{
			Category:   Category6,
			TotalLimit: 1120,
		},
	}

	got, err := RankAffordableAt(boatPolar, configuration, 20, 90)
	if err != nil {
		t.Fatalf("RankAffordableAt() a retourné une erreur inattendue: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("RankAffordableAt() doit retourner au moins une combinaison")
	}

	best := got[0]
	wantActive, err := NewSet(Code0, Foils, HullPolish)
	if err != nil {
		t.Fatalf("NewSet() a retourné une erreur inattendue: %v", err)
	}

	if best.Active != wantActive {
		t.Errorf("meilleure combinaison = %08b, attendu %08b", best.Active, wantActive)
	}
	if best.BestSail.SailID != 5 || best.BestSail.SailName != "Code0" {
		t.Errorf(
			"meilleure voile = %q (%d), attendu Code0 (5)",
			best.BestSail.SailName,
			best.BestSail.SailID,
		)
	}
	if best.Cost.TotalCost != 920 {
		t.Errorf("coût total = %d, attendu 920", best.Cost.TotalCost)
	}

	const wantSpeed polar.BoatSpeed = 38.386816
	if difference := math.Abs(float64(best.BestSail.Speed - wantSpeed)); difference > 1e-9 {
		t.Errorf(
			"vitesse = %.12f, attendu %.12f",
			best.BestSail.Speed,
			wantSpeed,
		)
	}

	const wantGain polar.BoatSpeed = 3.426816
	if difference := math.Abs(float64(best.SpeedGain - wantGain)); difference > 1e-9 {
		t.Errorf("gain = %.12f, attendu %.12f", best.SpeedGain, wantGain)
	}

	assertRankingOrder(t, got)
}

func TestRankAffordableAtUsesOwnedEquipmentAsBaseline(t *testing.T) {
	path := filepath.Join("..", "..", "..", "data", "polars", "ultim_bp_11.json")
	boatPolar, err := polar.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() n'a pas chargé la polaire Ultim: %v", err)
	}

	owned, err := NewSet(Code0)
	if err != nil {
		t.Fatalf("NewSet() a retourné une erreur inattendue: %v", err)
	}

	got, err := RankAffordableAt(
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
		t.Fatalf("RankAffordableAt() a retourné une erreur inattendue: %v", err)
	}

	var baseline *RankedCombination
	for index := range got {
		if got[index].Active == owned {
			baseline = &got[index]
			break
		}
	}
	if baseline == nil {
		t.Fatal("la configuration possédée doit être présente")
	}
	if baseline.SpeedGain != 0 {
		t.Errorf("gain de la configuration possédée = %v, attendu 0", baseline.SpeedGain)
	}
	if baseline.PurchaseCost != 0 || baseline.AdditionalPersonalCredits != 0 {
		t.Errorf("la configuration possédée ne doit rien coûter: %+v", *baseline)
	}
}

func TestRankAffordableAtPrefersCheaperCombinationOnEqualSpeed(t *testing.T) {
	path := filepath.Join("..", "..", "..", "data", "polars", "ultim_bp_11.json")
	boatPolar, err := polar.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() n'a pas chargé la polaire Ultim: %v", err)
	}

	got, err := RankAffordableAt(
		boatPolar,
		RaceConfiguration{
			Budget: RaceBudget{
				Category:   Category6,
				TotalLimit: 1120,
			},
		},
		20,
		90,
	)
	if err != nil {
		t.Fatalf("RankAffordableAt() a retourné une erreur inattendue: %v", err)
	}

	best := got[0]
	if best.Active.Has(ProWinches) {
		t.Error("les winchs pro sans effet instantané ne doivent pas être ajoutés à égalité")
	}

	for _, result := range got[1:] {
		if result.BestSail.Speed != best.BestSail.Speed {
			break
		}
		if result.AdditionalPersonalCredits < best.AdditionalPersonalCredits {
			t.Errorf("une combinaison à égalité moins chère est classée après la première")
		}
	}
}

func TestRankAffordableAtRejectsInvalidInputs(t *testing.T) {
	validConfiguration := RaceConfiguration{
		Budget: RaceBudget{
			Category:   Category6,
			TotalLimit: 1000,
		},
	}

	tests := []struct {
		name          string
		boatPolar     *polar.Polar
		configuration RaceConfiguration
		windSpeed     polar.WindSpeed
		errorPart     string
	}{
		{
			name:          "polaire nil",
			boatPolar:     nil,
			configuration: validConfiguration,
			windSpeed:     20,
			errorPart:     "polaire est nil",
		},
		{
			name:      "configuration invalide",
			boatPolar: effectsTestPolar(),
			configuration: RaceConfiguration{
				Budget: RaceBudget{Category: 0, TotalLimit: 1000},
			},
			windSpeed: 20,
			errorPart: "catégorie",
		},
		{
			name: "voiles requises absentes",
			boatPolar: &polar.Polar{
				Sails: []polar.Sail{
					{ID: 1, Name: "Jib"},
					{ID: 2, Name: "Spi"},
				},
			},
			configuration: validConfiguration,
			windSpeed:     20,
			errorPart:     "absentes",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := RankAffordableAt(
				test.boatPolar,
				test.configuration,
				test.windSpeed,
				90,
			)
			if err == nil {
				t.Fatal("RankAffordableAt() doit refuser cette entrée")
			}
			if !strings.Contains(err.Error(), test.errorPart) {
				t.Errorf("erreur = %q, doit contenir %q", err, test.errorPart)
			}
		})
	}
}

func assertRankingOrder(t *testing.T, ranked []RankedCombination) {
	t.Helper()

	for index := 1; index < len(ranked); index++ {
		previous := ranked[index-1]
		current := ranked[index]

		if previous.BestSail.Speed < current.BestSail.Speed {
			t.Fatalf(
				"classement inversé entre les positions %d et %d",
				index-1,
				index,
			)
		}
		if previous.BestSail.Speed == current.BestSail.Speed &&
			previous.AdditionalPersonalCredits >
				current.AdditionalPersonalCredits {
			t.Fatalf(
				"coût personnel inversé entre les positions %d et %d",
				index-1,
				index,
			)
		}
	}
}

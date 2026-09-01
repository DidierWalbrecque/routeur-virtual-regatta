package equipment

import (
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
)

func TestResolveEffectsAvailableSails(t *testing.T) {
	boatPolar := effectsTestPolar()

	tests := []struct {
		name       string
		equipments []Equipment
		wantSails  []polar.SailID
	}{
		{
			name:      "voiles de base",
			wantSails: []polar.SailID{1, 2},
		},
		{
			name:       "pack light",
			equipments: []Equipment{Light},
			wantSails:  []polar.SailID{1, 2, 4, 7},
		},
		{
			name:       "pack heavy",
			equipments: []Equipment{Heavy},
			wantSails:  []polar.SailID{1, 2, 3, 6},
		},
		{
			name:       "code zero",
			equipments: []Equipment{Code0},
			wantSails:  []polar.SailID{1, 2, 5},
		},
		{
			name:       "toutes les voiles",
			equipments: []Equipment{Light, Heavy, Code0},
			wantSails:  []polar.SailID{1, 2, 3, 4, 5, 6, 7},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			set, err := NewSet(test.equipments...)
			if err != nil {
				t.Fatalf("NewSet() a retourné une erreur inattendue: %v", err)
			}

			got, err := ResolveEffects(boatPolar, set)
			if err != nil {
				t.Fatalf("ResolveEffects() a retourné une erreur inattendue: %v", err)
			}

			if !reflect.DeepEqual(got.AvailableSails, test.wantSails) {
				t.Errorf(
					"AvailableSails = %v, attendu %v",
					got.AvailableSails,
					test.wantSails,
				)
			}
		})
	}
}

func TestResolveEffectsPerformanceOptions(t *testing.T) {
	set, err := NewSet(Foils, HullPolish, ProWinches)
	if err != nil {
		t.Fatalf("NewSet() a retourné une erreur inattendue: %v", err)
	}

	got, err := ResolveEffects(effectsTestPolar(), set)
	if err != nil {
		t.Fatalf("ResolveEffects() a retourné une erreur inattendue: %v", err)
	}

	if !got.SpeedOptions.Foils {
		t.Error("les foils doivent être activés")
	}
	if !got.SpeedOptions.HullPolish {
		t.Error("le polish de coque doit être activé")
	}
	if !got.ProWinches {
		t.Error("les winchs pro doivent être activés")
	}
}

func TestResolveEffectsPreservesPolarSailOrder(t *testing.T) {
	boatPolar := effectsTestPolar()
	boatPolar.Sails[0], boatPolar.Sails[1] = boatPolar.Sails[1], boatPolar.Sails[0]

	got, err := ResolveEffects(boatPolar, 0)
	if err != nil {
		t.Fatalf("ResolveEffects() a retourné une erreur inattendue: %v", err)
	}

	want := []polar.SailID{2, 1}
	if !reflect.DeepEqual(got.AvailableSails, want) {
		t.Errorf("AvailableSails = %v, attendu %v", got.AvailableSails, want)
	}
}

func TestResolveEffectsRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name       string
		boatPolar  *polar.Polar
		equipments EquipmentSet
		errorPart  string
	}{
		{
			name:      "polaire nil",
			boatPolar: nil,
			errorPart: "polaire est nil",
		},
		{
			name:       "ensemble inconnu",
			boatPolar:  effectsTestPolar(),
			equipments: EquipmentSet(1 << 7),
			errorPart:  "option inconnue",
		},
		{
			name: "voile requise absente",
			boatPolar: &polar.Polar{
				Sails: []polar.Sail{
					{ID: 1, Name: "Jib"},
					{ID: 2, Name: "Spi"},
					{ID: 4, Name: "LightJib"},
				},
			},
			equipments: EquipmentSet(Light),
			errorPart:  "LightGnk",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ResolveEffects(test.boatPolar, test.equipments)
			if err == nil {
				t.Fatal("ResolveEffects() doit refuser cette entrée")
			}
			if !strings.Contains(err.Error(), test.errorPart) {
				t.Errorf("erreur = %q, doit contenir %q", err, test.errorPart)
			}
		})
	}
}

func TestResolvedEffectsWithUltimSample(t *testing.T) {
	path := filepath.Join("..", "..", "..", "data", "polars", "ultim_bp_11.json")
	boatPolar, err := polar.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() n'a pas chargé la polaire Ultim: %v", err)
	}

	set, err := NewSet(Light, Code0, Foils, HullPolish)
	if err != nil {
		t.Fatalf("NewSet() a retourné une erreur inattendue: %v", err)
	}

	effects, err := ResolveEffects(boatPolar, set)
	if err != nil {
		t.Fatalf("ResolveEffects() a retourné une erreur inattendue: %v", err)
	}

	got, err := boatPolar.BestEffectiveSailAt(
		20,
		90,
		effects.AvailableSails,
		effects.SpeedOptions,
	)
	if err != nil {
		t.Fatalf("BestEffectiveSailAt() a retourné une erreur inattendue: %v", err)
	}

	if got.SailID != 5 || got.SailName != "Code0" {
		t.Errorf(
			"voile = %q (%d), attendu %q (%d)",
			got.SailName,
			got.SailID,
			"Code0",
			5,
		)
	}

	const want polar.BoatSpeed = 38.386816
	if difference := math.Abs(float64(got.Speed - want)); difference > 1e-9 {
		t.Errorf("Speed = %.12f, attendu %.12f", got.Speed, want)
	}
}

func effectsTestPolar() *polar.Polar {
	return &polar.Polar{
		Sails: []polar.Sail{
			{ID: 1, Name: "Jib"},
			{ID: 2, Name: "Spi"},
			{ID: 3, Name: "Staysail"},
			{ID: 4, Name: "LightJib"},
			{ID: 5, Name: "Code0"},
			{ID: 6, Name: "HeavyGnk"},
			{ID: 7, Name: "LightGnk"},
		},
	}
}

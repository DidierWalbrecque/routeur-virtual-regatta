package navigation

import (
	"math"
	"strings"
	"testing"
)

func TestSpeedConversions(t *testing.T) {
	metersPerSecond, err := MetersPerSecondFromKnots(10)
	if err != nil {
		t.Fatalf("MetersPerSecondFromKnots() a échoué: %v", err)
	}
	if difference := math.Abs(
		float64(metersPerSecond) - 5.144444444444445,
	); difference > 1e-12 {
		t.Errorf(
			"vitesse = %.15f m/s, attendue %.15f",
			metersPerSecond,
			5.144444444444445,
		)
	}

	knots, err := KnotsFromMetersPerSecond(10)
	if err != nil {
		t.Fatalf("KnotsFromMetersPerSecond() a échoué: %v", err)
	}
	if difference := math.Abs(
		float64(knots) - 19.438444924406046,
	); difference > 1e-12 {
		t.Errorf(
			"vitesse = %.15f nd, attendue %.15f",
			knots,
			19.438444924406046,
		)
	}
}

func TestSpeedConversionRoundTrip(t *testing.T) {
	const original Knots = 8.4

	metersPerSecond, err := MetersPerSecondFromKnots(original)
	if err != nil {
		t.Fatalf("conversion en m/s impossible: %v", err)
	}
	converted, err := KnotsFromMetersPerSecond(metersPerSecond)
	if err != nil {
		t.Fatalf("conversion en nœuds impossible: %v", err)
	}

	if difference := math.Abs(
		float64(converted - original),
	); difference > 1e-12 {
		t.Errorf(
			"aller-retour = %.15f nd, attendu %.15f",
			converted,
			original,
		)
	}
}

func TestDistanceAtConstantSpeed(t *testing.T) {
	tests := []struct {
		name     string
		speed    Knots
		duration Seconds
		want     NauticalMiles
	}{
		{
			name:     "une heure",
			speed:    8,
			duration: 3600,
			want:     8,
		},
		{
			name:     "quatre minutes",
			speed:    8,
			duration: 240,
			want:     0.5333333333333333,
		},
		{
			name:     "durée nulle",
			speed:    8,
			duration: 0,
			want:     0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := DistanceAtConstantSpeed(
				test.speed,
				test.duration,
			)
			if err != nil {
				t.Fatalf(
					"DistanceAtConstantSpeed() a échoué: %v",
					err,
				)
			}
			if difference := math.Abs(
				float64(got - test.want),
			); difference > 1e-12 {
				t.Errorf(
					"distance = %.15f NM, attendue %.15f",
					got,
					test.want,
				)
			}
		})
	}
}

func TestUnitOperationsRejectInvalidValues(t *testing.T) {
	tests := []struct {
		name      string
		run       func() error
		errorPart string
	}{
		{
			name: "nœuds négatifs",
			run: func() error {
				_, err := MetersPerSecondFromKnots(-1)
				return err
			},
			errorPart: "nœuds",
		},
		{
			name: "mètres par seconde non finis",
			run: func() error {
				_, err := KnotsFromMetersPerSecond(
					MetersPerSecond(math.Inf(1)),
				)
				return err
			},
			errorPart: "mètres par seconde",
		},
		{
			name: "durée négative",
			run: func() error {
				_, err := DistanceAtConstantSpeed(8, -1)
				return err
			},
			errorPart: "secondes",
		},
		{
			name: "vitesse non finie",
			run: func() error {
				_, err := DistanceAtConstantSpeed(
					Knots(math.NaN()),
					60,
				)
				return err
			},
			errorPart: "nœuds",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.run()
			if err == nil {
				t.Fatal("l'opération doit refuser cette valeur")
			}
			if !strings.Contains(err.Error(), test.errorPart) {
				t.Errorf(
					"erreur = %q, doit contenir %q",
					err,
					test.errorPart,
				)
			}
		})
	}
}

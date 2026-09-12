package simulation

import (
	"math"
	"strings"
	"testing"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
)

func TestStaminaWindCurveFactorAt(t *testing.T) {
	curve := testStaminaWindCurve()
	tests := []struct {
		name      string
		windSpeed polar.WindSpeed
		want      StaminaWindFactor
	}{
		{
			name:      "sous le premier point",
			windSpeed: 0,
			want:      0.5,
		},
		{
			name:      "premier point exact",
			windSpeed: 5,
			want:      0.5,
		},
		{
			name:      "premier intervalle",
			windSpeed: 10,
			want:      0.75,
		},
		{
			name:      "point central exact",
			windSpeed: 15,
			want:      1,
		},
		{
			name:      "second intervalle",
			windSpeed: 25,
			want:      1.25,
		},
		{
			name:      "dernier point exact",
			windSpeed: 35,
			want:      1.5,
		},
		{
			name:      "au-dessus du dernier point",
			windSpeed: 70,
			want:      1.5,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := curve.FactorAt(test.windSpeed)
			if err != nil {
				t.Fatalf("interpolation refusée: %v", err)
			}
			if math.Abs(float64(got-test.want)) > 1e-12 {
				t.Errorf(
					"coefficient = %.12f, attendu %.12f",
					got,
					test.want,
				)
			}
		})
	}
}

func TestStaminaWindCurveAllowsConstantFactor(t *testing.T) {
	curve := StaminaWindCurve{
		Points: []StaminaWindPoint{
			{WindSpeed: 10, Factor: 1.2},
		},
	}
	for _, windSpeed := range []polar.WindSpeed{0, 10, 70} {
		got, err := curve.FactorAt(windSpeed)
		if err != nil {
			t.Fatalf("vent %.2f refusé: %v", windSpeed, err)
		}
		if got != 1.2 {
			t.Errorf(
				"vent %.2f: coefficient = %.6f, attendu 1.2",
				windSpeed,
				got,
			)
		}
	}
}

func TestStaminaConsumptionAdjustedForWind(t *testing.T) {
	consumption := StaminaConsumption{
		Nautical:   10,
		SailChange: 20,
	}
	got, err := consumption.AdjustedForWind(
		25,
		testStaminaWindCurve(),
	)
	if err != nil {
		t.Fatalf("ajustement refusé: %v", err)
	}
	want := StaminaConsumption{
		Nautical:   12.5,
		SailChange: 25,
	}
	if got != want {
		t.Errorf("consommation = %+v, attendue %+v", got, want)
	}
}

func TestStaminaConsumptionAdjustedForWindPreservesIndependentCosts(
	t *testing.T,
) {
	curve := StaminaWindCurve{
		Points: []StaminaWindPoint{
			{WindSpeed: 0, Factor: 2},
		},
	}
	tests := []struct {
		name        string
		consumption StaminaConsumption
		want        StaminaConsumption
	}{
		{
			name:        "coût nautique seul",
			consumption: StaminaConsumption{Nautical: 10},
			want:        StaminaConsumption{Nautical: 20},
		},
		{
			name:        "changement de voile seul",
			consumption: StaminaConsumption{SailChange: 20},
			want:        StaminaConsumption{SailChange: 40},
		},
		{
			name: "deux coûts indépendants",
			consumption: StaminaConsumption{
				Nautical:   10,
				SailChange: 20,
			},
			want: StaminaConsumption{
				Nautical:   20,
				SailChange: 40,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.consumption.AdjustedForWind(20, curve)
			if err != nil {
				t.Fatalf("ajustement refusé: %v", err)
			}
			if got != test.want {
				t.Errorf(
					"consommation = %+v, attendue %+v",
					got,
					test.want,
				)
			}
		})
	}
}

func TestStaminaWindCurveRejectsInvalidInputs(t *testing.T) {
	validCurve := testStaminaWindCurve()
	tests := []struct {
		name      string
		windSpeed polar.WindSpeed
		curve     StaminaWindCurve
		wantErr   string
	}{
		{
			name:      "vent négatif",
			windSpeed: -1,
			curve:     validCurve,
			wantErr:   "vitesse du vent",
		},
		{
			name:      "vent non fini",
			windSpeed: polar.WindSpeed(math.NaN()),
			curve:     validCurve,
			wantErr:   "vitesse du vent",
		},
		{
			name:      "courbe vide",
			windSpeed: 10,
			curve:     StaminaWindCurve{},
			wantErr:   "courbe",
		},
		{
			name:      "vitesse de point négative",
			windSpeed: 10,
			curve: StaminaWindCurve{
				Points: []StaminaWindPoint{
					{WindSpeed: -1, Factor: 1},
				},
			},
			wantErr: "vitesse du vent invalide",
		},
		{
			name:      "vitesses dupliquées",
			windSpeed: 10,
			curve: StaminaWindCurve{
				Points: []StaminaWindPoint{
					{WindSpeed: 5, Factor: 1},
					{WindSpeed: 5, Factor: 1.2},
				},
			},
			wantErr: "non strictement croissantes",
		},
		{
			name:      "vitesses décroissantes",
			windSpeed: 10,
			curve: StaminaWindCurve{
				Points: []StaminaWindPoint{
					{WindSpeed: 15, Factor: 1},
					{WindSpeed: 5, Factor: 1.2},
				},
			},
			wantErr: "non strictement croissantes",
		},
		{
			name:      "coefficient négatif",
			windSpeed: 10,
			curve: StaminaWindCurve{
				Points: []StaminaWindPoint{
					{WindSpeed: 5, Factor: -0.1},
				},
			},
			wantErr: "coefficient de vent",
		},
		{
			name:      "coefficient non fini",
			windSpeed: 10,
			curve: StaminaWindCurve{
				Points: []StaminaWindPoint{
					{
						WindSpeed: 5,
						Factor:    StaminaWindFactor(math.Inf(1)),
					},
				},
			},
			wantErr: "coefficient de vent",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.curve.FactorAt(test.windSpeed)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %v, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

func TestStaminaConsumptionAdjustedForWindRejectsInvalidResult(t *testing.T) {
	consumption := StaminaConsumption{
		Nautical: 60,
	}
	curve := StaminaWindCurve{
		Points: []StaminaWindPoint{
			{WindSpeed: 0, Factor: 2},
		},
	}
	_, err := consumption.AdjustedForWind(10, curve)
	if err == nil || !strings.Contains(err.Error(), "résultat invalide") {
		t.Fatalf(
			"erreur = %v, doit signaler un résultat invalide",
			err,
		)
	}
}

func TestStaminaConsumptionAdjustedForWindRejectsInvalidConsumption(
	t *testing.T,
) {
	consumption := StaminaConsumption{
		Nautical: -1,
	}
	_, err := consumption.AdjustedForWind(
		10,
		testStaminaWindCurve(),
	)
	if err == nil || !strings.Contains(err.Error(), "consommation nautique") {
		t.Fatalf(
			"erreur = %v, doit signaler la consommation nautique",
			err,
		)
	}
}

func testStaminaWindCurve() StaminaWindCurve {
	return StaminaWindCurve{
		Points: []StaminaWindPoint{
			{WindSpeed: 5, Factor: 0.5},
			{WindSpeed: 15, Factor: 1},
			{WindSpeed: 35, Factor: 1.5},
		},
	}
}

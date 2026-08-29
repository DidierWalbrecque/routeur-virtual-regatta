package polar

import (
	"math"
	"path/filepath"
	"strings"
	"testing"
)

func TestFoilRatioAt(t *testing.T) {
	foil := ultimFoilTestProfile()

	tests := []struct {
		name      string
		windSpeed WindSpeed
		windAngle TrueWindAngle
		want      SpeedRatio
	}{
		{
			name:      "hors zone sous le vent minimum",
			windSpeed: 10,
			windAngle: 100,
			want:      1,
		},
		{
			name:      "début de transition TWS basse",
			windSpeed: 11,
			windAngle: 100,
			want:      1,
		},
		{
			name:      "milieu de transition TWS basse",
			windSpeed: 13.5,
			windAngle: 100,
			want:      1.02,
		},
		{
			name:      "zone maximale",
			windSpeed: 20,
			windAngle: 100,
			want:      1.04,
		},
		{
			name:      "milieu de transition TWS haute",
			windSpeed: 37.5,
			windAngle: 100,
			want:      1.02,
		},
		{
			name:      "fin de transition TWS haute",
			windSpeed: 40,
			windAngle: 100,
			want:      1,
		},
		{
			name:      "milieu de transition TWA basse",
			windSpeed: 20,
			windAngle: 75,
			want:      1.02,
		},
		{
			name:      "milieu de transition TWA haute",
			windSpeed: 20,
			windAngle: 165,
			want:      1.02,
		},
		{
			name:      "deux transitions simultanées",
			windSpeed: 13.5,
			windAngle: 75,
			want:      1.01,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := foil.RatioAt(test.windSpeed, test.windAngle)
			if err != nil {
				t.Fatalf("RatioAt() a retourné une erreur inattendue: %v", err)
			}
			if difference := math.Abs(float64(got - test.want)); difference > 1e-9 {
				t.Errorf("RatioAt() = %.12f, attendu %.12f", got, test.want)
			}
		})
	}
}

func TestFoilRatioAtWithoutMerge(t *testing.T) {
	foil := ultimFoilTestProfile()
	foil.TWSMerge = 0
	foil.TWAMerge = 0

	tests := []struct {
		name      string
		windSpeed WindSpeed
		windAngle TrueWindAngle
		want      SpeedRatio
	}{
		{
			name:      "juste avant la zone",
			windSpeed: 15.9,
			windAngle: 100,
			want:      1,
		},
		{
			name:      "limite minimale incluse",
			windSpeed: 16,
			windAngle: 80,
			want:      1.04,
		},
		{
			name:      "limite maximale incluse",
			windSpeed: 35,
			windAngle: 160,
			want:      1.04,
		},
		{
			name:      "juste après la zone",
			windSpeed: 35.1,
			windAngle: 100,
			want:      1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := foil.RatioAt(test.windSpeed, test.windAngle)
			if err != nil {
				t.Fatalf("RatioAt() a retourné une erreur inattendue: %v", err)
			}
			if difference := math.Abs(float64(got - test.want)); difference > 1e-9 {
				t.Errorf("RatioAt() = %.12f, attendu %.12f", got, test.want)
			}
		})
	}
}

func TestFoilRatioAtRejectsInvalidProfiles(t *testing.T) {
	tests := []struct {
		name      string
		change    func(*FoilProfile)
		errorPart string
	}{
		{
			name: "coefficient inférieur à un",
			change: func(foil *FoilProfile) {
				foil.MaxSpeedRatio = 0.9
			},
			errorPart: "foil.speedRatio",
		},
		{
			name: "coefficient non fini",
			change: func(foil *FoilProfile) {
				foil.MaxSpeedRatio = SpeedRatio(math.NaN())
			},
			errorPart: "foil.speedRatio",
		},
		{
			name: "plage TWA inversée",
			change: func(foil *FoilProfile) {
				foil.TWAMin = 170
				foil.TWAMax = 160
			},
			errorPart: "plage TWA",
		},
		{
			name: "transition TWA trop grande",
			change: func(foil *FoilProfile) {
				foil.TWAMerge = 81
			},
			errorPart: "foil.twaMerge",
		},
		{
			name: "plage TWS inversée",
			change: func(foil *FoilProfile) {
				foil.TWSMin = 36
				foil.TWSMax = 35
			},
			errorPart: "plage TWS",
		},
		{
			name: "transition TWS négative",
			change: func(foil *FoilProfile) {
				foil.TWSMerge = -1
			},
			errorPart: "foil.twsMerge",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			foil := ultimFoilTestProfile()
			test.change(&foil)

			_, err := foil.RatioAt(20, 100)
			if err == nil {
				t.Fatal("RatioAt() doit refuser ce profil")
			}
			if !strings.Contains(err.Error(), test.errorPart) {
				t.Errorf("erreur = %q, doit contenir %q", err, test.errorPart)
			}
		})
	}
}

func TestFoilRatioAtRejectsInvalidWind(t *testing.T) {
	foil := ultimFoilTestProfile()

	tests := []struct {
		name      string
		windSpeed WindSpeed
		windAngle TrueWindAngle
		errorPart string
	}{
		{
			name:      "TWS négative",
			windSpeed: -1,
			windAngle: 100,
			errorPart: "TWS",
		},
		{
			name:      "TWA supérieure à 180",
			windSpeed: 20,
			windAngle: 181,
			errorPart: "TWA",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := foil.RatioAt(test.windSpeed, test.windAngle)
			if err == nil {
				t.Fatal("RatioAt() doit refuser cette valeur")
			}
			if !strings.Contains(err.Error(), test.errorPart) {
				t.Errorf("erreur = %q, doit contenir %q", err, test.errorPart)
			}
		})
	}
}

func TestEffectiveSpeedAtWithFoilsAndHull(t *testing.T) {
	path := filepath.Join("..", "..", "..", "data", "polars", "ultim_bp_11.json")
	polar, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() n'a pas chargé la polaire Ultim: %v", err)
	}

	tests := []struct {
		name    string
		options SpeedOptions
		want    BoatSpeed
	}{
		{
			name:    "foils uniquement",
			options: SpeedOptions{Foils: true},
			want:    38.272,
		},
		{
			name: "foils et polish",
			options: SpeedOptions{
				Foils:      true,
				HullPolish: true,
			},
			want: 38.386816,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, speedErr := polar.EffectiveSpeedAt(5, 20, 90, test.options)
			if speedErr != nil {
				t.Fatalf("EffectiveSpeedAt() a retourné une erreur inattendue: %v", speedErr)
			}
			if difference := math.Abs(float64(got - test.want)); difference > 1e-9 {
				t.Errorf("EffectiveSpeedAt() = %.12f, attendu %.12f", got, test.want)
			}
		})
	}
}

func ultimFoilTestProfile() FoilProfile {
	return FoilProfile{
		MaxSpeedRatio: 1.04,
		TWAMin:        80,
		TWAMax:        160,
		TWAMerge:      10,
		TWSMin:        16,
		TWSMax:        35,
		TWSMerge:      5,
	}
}

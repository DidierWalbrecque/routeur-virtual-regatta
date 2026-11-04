package navigation

import (
	"math"
	"strings"
	"testing"
)

func TestParseVirtualRegattaPosition(t *testing.T) {
	tests := []struct {
		name          string
		value         string
		wantLatitude  float64
		wantLongitude float64
	}{
		{
			name:          "nord et est",
			value:         "13°N 04'08\" - 121°E 37'42\"",
			wantLatitude:  13.068888888888889,
			wantLongitude: 121.62833333333333,
		},
		{
			name:          "sud et ouest typographiques",
			value:         "33°S 52′08″ — 151°W 12′33″",
			wantLatitude:  -33.86888888888889,
			wantLongitude: -151.20916666666668,
		},
		{
			name:          "hémisphère en fin de coordonnée",
			value:         "13°07'16\"N - 121°35'12\"E",
			wantLatitude:  13 + 7.0/60 + 16.0/3600,
			wantLongitude: 121 + 35.0/60 + 12.0/3600,
		},
		{
			name:          "secondes décimales, hémisphère en fin",
			value:         "47°26'11.95\"N - 24°54'55.32\"W",
			wantLatitude:  47 + 26.0/60 + 11.95/3600,
			wantLongitude: -(24 + 54.0/60 + 55.32/3600),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseVirtualRegattaPosition(test.value)
			if err != nil {
				t.Fatalf("conversion refusée: %v", err)
			}
			if difference := math.Abs(
				float64(got.Latitude) - test.wantLatitude,
			); difference > 1e-12 {
				t.Errorf(
					"latitude = %.12f, attendue %.12f",
					got.Latitude,
					test.wantLatitude,
				)
			}
			if difference := math.Abs(
				float64(got.Longitude) - test.wantLongitude,
			); difference > 1e-12 {
				t.Errorf(
					"longitude = %.12f, attendue %.12f",
					got.Longitude,
					test.wantLongitude,
				)
			}
		})
	}
}

func TestParseVirtualRegattaPositionRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{
		"13.068889, 121.628333",
		"13°N 60'00\" - 121°E 37'42\"",
		"91°N 00'00\" - 121°E 37'42\"",
	} {
		t.Run(value, func(t *testing.T) {
			_, err := ParseVirtualRegattaPosition(value)
			if err == nil {
				t.Fatal("la position doit être refusée")
			}
			if strings.TrimSpace(err.Error()) == "" {
				t.Fatal("l'erreur doit être explicite")
			}
		})
	}
}

func TestFormatVirtualRegattaPosition(t *testing.T) {
	tests := []struct {
		name     string
		position Position
		want     string
	}{
		{
			name: "nord et est",
			position: Position{
				Latitude:  13.068888888888889,
				Longitude: 121.62833333333333,
			},
			want: "13°N 04'08\" - 121°E 37'42\"",
		},
		{
			name: "sud et ouest",
			position: Position{
				Latitude:  -33.86888888888889,
				Longitude: -151.20916666666668,
			},
			want: "33°S 52'08\" - 151°W 12'33\"",
		},
		{
			name: "résultat décimal converti",
			position: Position{
				Latitude:  13.087225,
				Longitude: 121.613088,
			},
			want: "13°N 05'14\" - 121°E 36'47\"",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := FormatVirtualRegattaPosition(test.position)
			if err != nil {
				t.Fatalf("formatage refusé: %v", err)
			}
			if got != test.want {
				t.Errorf("format = %q, attendu %q", got, test.want)
			}
		})
	}
}

func TestFormatVirtualRegattaPositionRejectsInvalidPosition(t *testing.T) {
	_, err := FormatVirtualRegattaPosition(
		Position{Latitude: 91, Longitude: 0},
	)
	if err == nil {
		t.Fatal("erreur attendue, aucune reçue")
	}
}

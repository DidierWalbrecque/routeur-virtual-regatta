package routing

import (
	"math"
	"strings"
	"testing"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

func TestGenerateHeadingGrid(t *testing.T) {
	tests := []struct {
		name        string
		spacing     float64
		wantCount   int
		wantHeading map[int]navigation.Heading
	}{
		{
			name:      "quatre points cardinaux",
			spacing:   90,
			wantCount: 4,
			wantHeading: map[int]navigation.Heading{
				0: 0,
				1: 90,
				2: 180,
				3: 270,
			},
		},
		{
			name:      "pas décimal",
			spacing:   7.5,
			wantCount: 48,
			wantHeading: map[int]navigation.Heading{
				0:  0,
				1:  7.5,
				47: 352.5,
			},
		},
		{
			name:      "un seul cap",
			spacing:   360,
			wantCount: 1,
			wantHeading: map[int]navigation.Heading{
				0: 0,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			headings, err := GenerateHeadingGrid(test.spacing)
			if err != nil {
				t.Fatalf("génération refusée: %v", err)
			}
			if len(headings) != test.wantCount {
				t.Fatalf(
					"nombre de caps = %d, attendu %d",
					len(headings),
					test.wantCount,
				)
			}
			for index, want := range test.wantHeading {
				if headings[index] != want {
					t.Errorf(
						"cap %d = %.12g°, attendu %.12g°",
						index,
						headings[index],
						want,
					)
				}
			}
			assertRegularHeadingGrid(t, headings, test.spacing)
		})
	}
}

func TestGenerateHeadingGridReturnsIndependentSlices(t *testing.T) {
	first, err := GenerateHeadingGrid(90)
	if err != nil {
		t.Fatalf("première génération refusée: %v", err)
	}
	second, err := GenerateHeadingGrid(90)
	if err != nil {
		t.Fatalf("seconde génération refusée: %v", err)
	}

	first[0] = 45
	if second[0] != 0 {
		t.Fatalf(
			"la seconde grille a été modifiée: premier cap = %.1f°",
			second[0],
		)
	}
}

func TestGenerateHeadingGridRejectsInvalidSpacing(t *testing.T) {
	tests := []struct {
		name    string
		spacing float64
		wantErr string
	}{
		{name: "nul", spacing: 0, wantErr: "strictement positif"},
		{name: "négatif", spacing: -10, wantErr: "strictement positif"},
		{name: "non fini", spacing: math.Inf(1), wantErr: "nombre fini"},
		{name: "NaN", spacing: math.NaN(), wantErr: "nombre fini"},
		{name: "supérieur au cercle", spacing: 361, wantErr: "360"},
		{name: "ne divise pas 360", spacing: 7, wantErr: "diviser exactement"},
		{name: "grille trop fine", spacing: 0.05, wantErr: "maximum autorisé"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := GenerateHeadingGrid(test.spacing)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %v, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

func assertRegularHeadingGrid(
	t *testing.T,
	headings []navigation.Heading,
	spacing float64,
) {
	t.Helper()

	for index, heading := range headings {
		if err := heading.Validate(); err != nil {
			t.Errorf("cap %d invalide: %v", index, err)
		}
		if index == 0 {
			continue
		}
		difference := float64(heading - headings[index-1])
		if math.Abs(difference-spacing) > 1e-12 {
			t.Errorf(
				"intervalle %d = %.12g°, attendu %.12g°",
				index,
				difference,
				spacing,
			)
		}
	}

	wrapDifference := fullCircleDegrees -
		float64(headings[len(headings)-1])
	if math.Abs(wrapDifference-spacing) > 1e-12 {
		t.Errorf(
			"dernier intervalle = %.12g°, attendu %.12g°",
			wrapDifference,
			spacing,
		)
	}
}

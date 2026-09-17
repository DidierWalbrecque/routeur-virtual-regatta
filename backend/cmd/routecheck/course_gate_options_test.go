package main

import (
	"strings"
	"testing"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/routing"
)

func TestParseCourseGates(t *testing.T) {
	gates, err := parseCourseGates(courseGateTexts{
		"porte décimale|13.1,121.2|13.3,121.4|right-to-left",
		"porte VR|13°N 06'00\" - 121°E 12'00\"|13°N 18'00\" - 121°E 24'00\"|either",
	})
	if err != nil {
		t.Fatalf("portes refusées: %v", err)
	}
	if len(gates) != 2 {
		t.Fatalf("portes = %d, attendues 2", len(gates))
	}

	if gates[0] != (routing.CourseGate{
		Name:       "porte décimale",
		FirstMark:  navigation.Position{Latitude: 13.1, Longitude: 121.2},
		SecondMark: navigation.Position{Latitude: 13.3, Longitude: 121.4},
		Direction:  routing.GateRightToLeft,
	}) {
		t.Errorf("première porte = %+v", gates[0])
	}
	if gates[1].Name != "porte VR" {
		t.Errorf("nom de la seconde porte = %q", gates[1].Name)
	}
	if gates[1].FirstMark != (navigation.Position{
		Latitude:  13.1,
		Longitude: 121.2,
	}) {
		t.Errorf("première marque VR = %+v", gates[1].FirstMark)
	}
	if gates[1].SecondMark != (navigation.Position{
		Latitude:  13.3,
		Longitude: 121.4,
	}) {
		t.Errorf("seconde marque VR = %+v", gates[1].SecondMark)
	}
	if gates[1].Direction != routing.GateEitherDirection {
		t.Errorf("sens de la seconde porte = %d", gates[1].Direction)
	}
}

func TestParseCourseGatesRejectsInvalidDescriptions(t *testing.T) {
	for _, test := range []struct {
		name    string
		value   string
		wantErr string
	}{
		{
			name:    "nombre de champs",
			value:   "porte|13,121|14,122",
			wantErr: "format",
		},
		{
			name:    "nom absent",
			value:   " |13,121|14,122|either",
			wantErr: "nom de porte",
		},
		{
			name:    "première marque",
			value:   "porte|91,121|14,122|either",
			wantErr: "première marque",
		},
		{
			name:    "seconde marque",
			value:   "porte|13,121|14,181|either",
			wantErr: "seconde marque",
		},
		{
			name:    "sens",
			value:   "porte|13,121|14,122|nord",
			wantErr: "sens",
		},
		{
			name:    "marques identiques",
			value:   "porte|13,121|13,121|either",
			wantErr: "distinctes",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseCourseGates(courseGateTexts{test.value})
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %v, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

func TestCourseGateTextsRejectsEmptyValue(t *testing.T) {
	var values courseGateTexts
	if err := values.Set(" "); err == nil {
		t.Fatal("une description vide doit être refusée")
	}
}

func TestFormatCourseGateDirection(t *testing.T) {
	for direction, want := range map[routing.GateDirection]string{
		routing.GateEitherDirection: "either",
		routing.GateRightToLeft:     "right-to-left",
		routing.GateLeftToRight:     "left-to-right",
	} {
		if got := formatCourseGateDirection(direction); got != want {
			t.Errorf("sens %d = %q, attendu %q", direction, got, want)
		}
	}
}

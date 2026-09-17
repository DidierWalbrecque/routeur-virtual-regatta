package routing

import (
	"strings"
	"testing"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

func TestCourseGateMidpoint(t *testing.T) {
	tests := []struct {
		name string
		gate CourseGate
		want navigation.Position
	}{
		{
			name: "porte ordinaire",
			gate: CourseGate{
				Name:       "porte 1",
				FirstMark:  navigation.Position{Latitude: 10, Longitude: 20},
				SecondMark: navigation.Position{Latitude: 14, Longitude: 30},
			},
			want: navigation.Position{Latitude: 12, Longitude: 25},
		},
		{
			name: "antiméridien",
			gate: CourseGate{
				Name:       "porte 2",
				FirstMark:  navigation.Position{Latitude: -2, Longitude: 179},
				SecondMark: navigation.Position{Latitude: 2, Longitude: -179},
			},
			want: navigation.Position{Latitude: 0, Longitude: -180},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.gate.Midpoint()
			if err != nil {
				t.Fatalf("centre refusé: %v", err)
			}
			if got != test.want {
				t.Errorf("centre = %+v, attendu %+v", got, test.want)
			}
		})
	}
}

func TestCourseProgressTarget(t *testing.T) {
	gates := []CourseGate{
		{
			Name:       "porte 1",
			FirstMark:  navigation.Position{Latitude: 1, Longitude: -1},
			SecondMark: navigation.Position{Latitude: 1, Longitude: 1},
		},
		{
			Name:       "porte 2",
			FirstMark:  navigation.Position{Latitude: 3, Longitude: 1},
			SecondMark: navigation.Position{Latitude: 3, Longitude: 3},
		},
	}
	destination := navigation.Position{Latitude: 5, Longitude: 5}

	tests := []struct {
		name     string
		progress CourseProgress
		want     navigation.Position
	}{
		{
			name:     "première porte",
			progress: CourseProgress{},
			want:     navigation.Position{Latitude: 1, Longitude: 0},
		},
		{
			name:     "seconde porte",
			progress: CourseProgress{NextGateIndex: 1},
			want:     navigation.Position{Latitude: 3, Longitude: 2},
		},
		{
			name:     "destination finale",
			progress: CourseProgress{NextGateIndex: 2},
			want:     destination,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.progress.Target(gates, destination)
			if err != nil {
				t.Fatalf("cible refusée: %v", err)
			}
			if got != test.want {
				t.Errorf("cible = %+v, attendue %+v", got, test.want)
			}
		})
	}
}

func TestCourseProgressTargetRejectsInvalidInputs(t *testing.T) {
	gates := []CourseGate{{
		Name:       "porte",
		FirstMark:  navigation.Position{Latitude: 1, Longitude: -1},
		SecondMark: navigation.Position{Latitude: 1, Longitude: 1},
	}}

	for _, test := range []struct {
		name        string
		progress    CourseProgress
		destination navigation.Position
		wantErr     string
	}{
		{
			name:        "progression hors parcours",
			progress:    CourseProgress{NextGateIndex: 2},
			destination: navigation.Position{},
			wantErr:     "hors de la séquence",
		},
		{
			name:        "destination invalide",
			progress:    CourseProgress{NextGateIndex: 1},
			destination: navigation.Position{Latitude: 91},
			wantErr:     "destination finale",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.progress.Target(gates, test.destination)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %v, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

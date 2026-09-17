package routing

import (
	"strings"
	"testing"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

func TestCourseProgressAdvancesAcrossOrderedGates(t *testing.T) {
	gates := []CourseGate{
		longitudeTestGate("première", 1, GateRightToLeft),
		longitudeTestGate("seconde", -1, GateRightToLeft),
	}
	segment := gateTestSegment(
		navigation.Position{Latitude: 0, Longitude: 2},
		navigation.Position{Latitude: 0, Longitude: -2},
	)

	progress, err := (CourseProgress{}).Advance(gates, segment)
	if err != nil {
		t.Fatalf("progression refusée: %v", err)
	}
	if progress.NextGateIndex != 2 {
		t.Fatalf(
			"prochaine porte = %d, attendue 2",
			progress.NextGateIndex,
		)
	}
	complete, err := progress.Complete(gates)
	if err != nil {
		t.Fatalf("contrôle refusé: %v", err)
	}
	if !complete {
		t.Fatal("parcours non terminé")
	}
}

func TestCourseProgressDoesNotCountGatesInWrongOrder(t *testing.T) {
	gates := []CourseGate{
		longitudeTestGate("première", -1, GateRightToLeft),
		longitudeTestGate("seconde", 1, GateRightToLeft),
	}
	segment := gateTestSegment(
		navigation.Position{Latitude: 0, Longitude: 2},
		navigation.Position{Latitude: 0, Longitude: -2},
	)

	progress, err := (CourseProgress{}).Advance(gates, segment)
	if err != nil {
		t.Fatalf("progression refusée: %v", err)
	}
	if progress.NextGateIndex != 1 {
		t.Fatalf(
			"prochaine porte = %d, attendue 1",
			progress.NextGateIndex,
		)
	}
}

func TestCourseProgressStopsAtWrongDirection(t *testing.T) {
	gates := []CourseGate{
		longitudeTestGate("porte", 0, GateLeftToRight),
	}
	segment := gateTestSegment(
		navigation.Position{Latitude: 0, Longitude: 1},
		navigation.Position{Latitude: 0, Longitude: -1},
	)

	progress, err := (CourseProgress{}).Advance(gates, segment)
	if err != nil {
		t.Fatalf("progression refusée: %v", err)
	}
	if progress.NextGateIndex != 0 {
		t.Fatalf(
			"prochaine porte = %d, attendue 0",
			progress.NextGateIndex,
		)
	}
}

func TestCourseProgressKeepsProgressWithoutCrossing(t *testing.T) {
	gates := []CourseGate{
		longitudeTestGate("première", 1, GateRightToLeft),
		longitudeTestGate("seconde", -1, GateRightToLeft),
	}
	segment := gateTestSegment(
		navigation.Position{Latitude: 0, Longitude: 0},
		navigation.Position{Latitude: 0, Longitude: -2},
	)

	progress, err := (CourseProgress{
		NextGateIndex: 1,
	}).Advance(gates, segment)
	if err != nil {
		t.Fatalf("progression refusée: %v", err)
	}
	if progress.NextGateIndex != 2 {
		t.Fatalf(
			"prochaine porte = %d, attendue 2",
			progress.NextGateIndex,
		)
	}
}

func TestCourseProgressCompleteWithoutGates(t *testing.T) {
	complete, err := (CourseProgress{}).Complete(nil)
	if err != nil {
		t.Fatalf("contrôle refusé: %v", err)
	}
	if !complete {
		t.Fatal("parcours vide non terminé")
	}
}

func TestCourseProgressRejectsInvalidInputs(t *testing.T) {
	validGates := []CourseGate{
		longitudeTestGate("porte", 0, GateEitherDirection),
	}
	validSegment := gateTestSegment(
		navigation.Position{Latitude: 0, Longitude: 1},
		navigation.Position{Latitude: 0, Longitude: -1},
	)
	tests := []struct {
		name     string
		progress CourseProgress
		gates    []CourseGate
		segment  RouteSegment
		wantErr  string
	}{
		{
			name:     "index négatif",
			progress: CourseProgress{NextGateIndex: -1},
			gates:    validGates,
			segment:  validSegment,
			wantErr:  "hors",
		},
		{
			name:     "index après la fin",
			progress: CourseProgress{NextGateIndex: 2},
			gates:    validGates,
			segment:  validSegment,
			wantErr:  "hors",
		},
		{
			name:     "porte invalide",
			progress: CourseProgress{},
			gates: []CourseGate{{
				Name: "sans largeur",
			}},
			segment: validSegment,
			wantErr: "porte obligatoire",
		},
		{
			name:     "segment invalide",
			progress: CourseProgress{},
			gates:    validGates,
			segment: gateTestSegment(
				navigation.Position{Latitude: 91},
				navigation.Position{Latitude: 0, Longitude: -1},
			),
			wantErr: "segment",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.progress.Advance(test.gates, test.segment)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %v, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

func longitudeTestGate(
	name string,
	longitude navigation.Longitude,
	direction GateDirection,
) CourseGate {
	return CourseGate{
		Name: name,
		FirstMark: navigation.Position{
			Latitude:  -1,
			Longitude: longitude,
		},
		SecondMark: navigation.Position{
			Latitude:  1,
			Longitude: longitude,
		},
		Direction: direction,
	}
}

package routing

import (
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

func TestCourseGateEvaluateAcceptsRequiredDirection(t *testing.T) {
	gate := verticalTestGate(GateRightToLeft)

	status, err := gate.Evaluate(gateTestSegment(
		navigation.Position{Latitude: 0, Longitude: 1},
		navigation.Position{Latitude: 0, Longitude: -1},
	))
	if err != nil {
		t.Fatalf("franchissement refusé: %v", err)
	}
	if status != GateCrossedAllowed {
		t.Fatalf("statut = %d, franchissement autorisé attendu", status)
	}
}

func TestCourseGateEvaluateRejectsWrongDirection(t *testing.T) {
	gate := verticalTestGate(GateRightToLeft)

	status, err := gate.Evaluate(gateTestSegment(
		navigation.Position{Latitude: 0, Longitude: -1},
		navigation.Position{Latitude: 0, Longitude: 1},
	))
	if err != nil {
		t.Fatalf("franchissement refusé: %v", err)
	}
	if status != GateCrossedWrongDirection {
		t.Fatalf("statut = %d, mauvais sens attendu", status)
	}
}

func TestCourseGateEvaluateAcceptsEitherDirection(t *testing.T) {
	gate := verticalTestGate(GateEitherDirection)

	for _, segment := range []RouteSegment{
		gateTestSegment(
			navigation.Position{Latitude: 0, Longitude: 1},
			navigation.Position{Latitude: 0, Longitude: -1},
		),
		gateTestSegment(
			navigation.Position{Latitude: 0, Longitude: -1},
			navigation.Position{Latitude: 0, Longitude: 1},
		),
	} {
		status, err := gate.Evaluate(segment)
		if err != nil {
			t.Fatalf("franchissement refusé: %v", err)
		}
		if status != GateCrossedAllowed {
			t.Fatalf("statut = %d, franchissement autorisé attendu", status)
		}
	}
}

func TestCourseGateEvaluateIgnoresSegmentsOutsideGate(t *testing.T) {
	gate := verticalTestGate(GateEitherDirection)
	tests := []struct {
		name    string
		segment RouteSegment
	}{
		{
			name: "croisement de la ligne après la seconde marque",
			segment: gateTestSegment(
				navigation.Position{Latitude: 2, Longitude: 1},
				navigation.Position{Latitude: 2, Longitude: -1},
			),
		},
		{
			name: "même côté",
			segment: gateTestSegment(
				navigation.Position{Latitude: 0, Longitude: 2},
				navigation.Position{Latitude: 0, Longitude: 1},
			),
		},
		{
			name: "départ sur la porte",
			segment: gateTestSegment(
				navigation.Position{Latitude: 0, Longitude: 0},
				navigation.Position{Latitude: 0, Longitude: -1},
			),
		},
		{
			name: "arrivée sur la porte",
			segment: gateTestSegment(
				navigation.Position{Latitude: 0, Longitude: 1},
				navigation.Position{Latitude: 0, Longitude: 0},
			),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, err := gate.Evaluate(test.segment)
			if err != nil {
				t.Fatalf("évaluation refusée: %v", err)
			}
			if status != GateNotCrossed {
				t.Fatalf("statut = %d, absence de franchissement attendue", status)
			}
		})
	}
}

func TestCourseGateEvaluateHandlesAntimeridian(t *testing.T) {
	gate := CourseGate{
		Name: "porte antiméridienne",
		FirstMark: navigation.Position{
			Latitude:  -1,
			Longitude: 180,
		},
		SecondMark: navigation.Position{
			Latitude:  1,
			Longitude: -180,
		},
		Direction: GateRightToLeft,
	}

	status, err := gate.Evaluate(gateTestSegment(
		navigation.Position{Latitude: 0, Longitude: -179.5},
		navigation.Position{Latitude: 0, Longitude: 179.5},
	))
	if err != nil {
		t.Fatalf("franchissement refusé: %v", err)
	}
	if status != GateCrossedAllowed {
		t.Fatalf("statut = %d, franchissement autorisé attendu", status)
	}
}

func TestCourseGateValidateRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name    string
		gate    CourseGate
		wantErr string
	}{
		{
			name: "nom absent",
			gate: CourseGate{
				FirstMark:  navigation.Position{Latitude: -1},
				SecondMark: navigation.Position{Latitude: 1},
			},
			wantErr: "nom",
		},
		{
			name: "première marque invalide",
			gate: CourseGate{
				Name:       "porte",
				FirstMark:  navigation.Position{Latitude: 91},
				SecondMark: navigation.Position{Latitude: 1},
			},
			wantErr: "première marque",
		},
		{
			name: "seconde marque invalide",
			gate: CourseGate{
				Name:       "porte",
				FirstMark:  navigation.Position{Latitude: -1},
				SecondMark: navigation.Position{Latitude: -91},
			},
			wantErr: "seconde marque",
		},
		{
			name: "marques identiques",
			gate: CourseGate{
				Name:       "porte",
				FirstMark:  navigation.Position{Latitude: 1, Longitude: 2},
				SecondMark: navigation.Position{Latitude: 1, Longitude: 2},
			},
			wantErr: "distinctes",
		},
		{
			name: "sens invalide",
			gate: CourseGate{
				Name:       "porte",
				FirstMark:  navigation.Position{Latitude: -1},
				SecondMark: navigation.Position{Latitude: 1},
				Direction:  GateDirection(99),
			},
			wantErr: "sens",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.gate.Validate()
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %v, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

func TestCourseGateEvaluateRejectsInvalidSegment(t *testing.T) {
	segment := gateTestSegment(
		navigation.Position{Latitude: 91},
		navigation.Position{Latitude: 0, Longitude: -1},
	)

	_, err := verticalTestGate(GateEitherDirection).Evaluate(segment)
	if err == nil || !strings.Contains(err.Error(), "segment") {
		t.Fatalf("erreur = %v, segment invalide attendu", err)
	}
}

func verticalTestGate(direction GateDirection) CourseGate {
	return CourseGate{
		Name:       "porte nord-sud",
		FirstMark:  navigation.Position{Latitude: -1, Longitude: 0},
		SecondMark: navigation.Position{Latitude: 1, Longitude: 0},
		Direction:  direction,
	}
}

func gateTestSegment(from, to navigation.Position) RouteSegment {
	startedAt := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	return RouteSegment{
		From:       from,
		To:         to,
		StartedAt:  startedAt,
		FinishedAt: startedAt.Add(10 * time.Minute),
	}
}

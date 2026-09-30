package httpapi

import (
	"strings"
	"testing"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/land"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/routing"
)

func TestCourseGatesFromVRStatePreservesOrderAndDirection(t *testing.T) {
	state := vrState{
		Checkpoints: []vrStateCheckpoint{
			{
				Group: 1, ID: 2, Name: "Porte tribord", Side: "stbd",
				Start: vrStatePosition{Latitude: 50, Longitude: -4},
				End:   vrStatePosition{Latitude: 50.1, Longitude: -4},
			},
			{
				Group: 2, ID: 1, Name: "Porte port", Side: "port",
				Start: vrStatePosition{Latitude: 51, Longitude: -3},
				End:   vrStatePosition{Latitude: 51.1, Longitude: -3},
			},
			{
				Group: 3, ID: 1, Name: "Porte libre", Rhumb: true, Side: "stbd",
				Start: vrStatePosition{Latitude: 52, Longitude: -2},
				End:   vrStatePosition{Latitude: 52.1, Longitude: -2},
			},
		},
	}

	gates, err := courseGatesFromVRState(state)
	if err != nil {
		t.Fatalf("conversion refusée: %v", err)
	}
	if len(gates) != 3 {
		t.Fatalf("nombre de portes = %d, attendu 3", len(gates))
	}
	wantDirections := []routing.GateDirection{
		routing.GateLeftToRight,
		routing.GateRightToLeft,
		routing.GateEitherDirection,
	}
	for index, gate := range gates {
		if gate.Name != state.Checkpoints[index].Name {
			t.Errorf("porte %d: nom = %q, attendu %q", index, gate.Name, state.Checkpoints[index].Name)
		}
		if gate.Direction != wantDirections[index] {
			t.Errorf("porte %d: direction = %d, attendue %d", index, gate.Direction, wantDirections[index])
		}
		if float64(gate.FirstMark.Latitude) != state.Checkpoints[index].Start.Latitude ||
			float64(gate.SecondMark.Longitude) != state.Checkpoints[index].End.Longitude {
			t.Errorf("porte %d: coordonnées incorrectes: %+v", index, gate)
		}
	}
}

func TestCourseGatesFromVRStateRejectsUnknownSide(t *testing.T) {
	_, err := courseGatesFromVRState(vrState{Checkpoints: []vrStateCheckpoint{{
		Name: "Porte invalide", Side: "unknown",
		Start: vrStatePosition{Latitude: 50, Longitude: -4},
		End:   vrStatePosition{Latitude: 50.1, Longitude: -4},
	}}})
	if err == nil {
		t.Fatal("un côté de porte inconnu devait être refusé")
	}
}

func TestCourseGatesFromVRStateSkipsPassedGateGroups(t *testing.T) {
	state := vrState{
		GateGroupCounters: []bool{true, false},
		Checkpoints: []vrStateCheckpoint{
			{
				Group: 1, ID: 1, Name: "Porte franchie",
				Start: vrStatePosition{Latitude: 50, Longitude: -4},
				End:   vrStatePosition{Latitude: 50.1, Longitude: -4},
			},
			{
				Group: 2, ID: 1, Name: "Porte suivante",
				Start: vrStatePosition{Latitude: 51, Longitude: -3},
				End:   vrStatePosition{Latitude: 51.1, Longitude: -3},
			},
		},
	}

	gates, err := courseGatesFromVRState(state)
	if err != nil {
		t.Fatalf("conversion refusée: %v", err)
	}
	if len(gates) != 1 || gates[0].Name != "Porte suivante" {
		t.Fatalf("portes restantes = %+v", gates)
	}
}

func TestAnnotateVRStateCheckpointsMarksLandMidpoint(t *testing.T) {
	originalLandLoader := loadLandForRequest
	t.Cleanup(func() { loadLandForRequest = originalLandLoader })
	loadLandForRequest = func(string) (*land.GeoJSONConstraint, bool, error) {
		constraint, err := land.DecodeGeoJSON(strings.NewReader(`{"type":"FeatureCollection","features":[{"type":"Feature","geometry":{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,1],[0,0]]]}}]}`))
		return constraint, false, err
	}
	state := vrState{Checkpoints: []vrStateCheckpoint{
		{
			Name: "Terre", Start: vrStatePosition{Latitude: 0.5, Longitude: 0.25}, End: vrStatePosition{Latitude: 0.5, Longitude: 0.75},
		},
		{
			Name: "Eau", Start: vrStatePosition{Latitude: 0.5, Longitude: -0.75}, End: vrStatePosition{Latitude: 0.5, Longitude: -0.25},
		},
	}}
	annotateVRStateCheckpoints(&state)
	if state.Checkpoints[0].OnLand == nil || !*state.Checkpoints[0].OnLand {
		t.Fatal("le milieu de la première porte devait être marqué terrestre")
	}
	if state.Checkpoints[1].OnLand == nil || *state.Checkpoints[1].OnLand {
		t.Fatal("le milieu de la seconde porte devait être marqué maritime")
	}
}

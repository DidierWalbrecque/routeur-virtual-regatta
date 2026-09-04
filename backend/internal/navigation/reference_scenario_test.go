package navigation

import (
	"math"
	"testing"
)

const virtualRegattaCoordinateTolerance = 0.5 / 3600

func TestVirtualRegattaReferenceDisplacement(t *testing.T) {
	start := Position{
		Latitude:  13 + 7.0/60 + 16.0/3600,
		Longitude: 121 + 35.0/60 + 12.0/3600,
	}
	observedAfterFourMinutes := Position{
		Latitude:  13 + 7.0/60 + 41.0/3600,
		Longitude: 121 + 34.0/60 + 51.0/3600,
	}

	distance, err := DistanceAtConstantSpeed(8, 4*60)
	if err != nil {
		t.Fatalf(
			"DistanceAtConstantSpeed() a retourné une erreur: %v",
			err,
		)
	}
	if difference := math.Abs(
		float64(distance - 8.0*4.0/60),
	); difference > 1e-12 {
		t.Errorf(
			"distance = %.12f NM, attendue %.12f NM",
			distance,
			8.0*4.0/60,
		)
	}

	projected, err := DestinationPosition(start, 321, distance)
	if err != nil {
		t.Fatalf(
			"DestinationPosition() a retourné une erreur: %v",
			err,
		)
	}

	if difference := math.Abs(
		float64(projected.Latitude - observedAfterFourMinutes.Latitude),
	); difference > virtualRegattaCoordinateTolerance {
		t.Errorf(
			"latitude projetée = %.12f°, observation VR = %.12f°, écart = %.6f seconde d'arc",
			projected.Latitude,
			observedAfterFourMinutes.Latitude,
			difference*3600,
		)
	}
	if difference := math.Abs(
		float64(projected.Longitude - observedAfterFourMinutes.Longitude),
	); difference > virtualRegattaCoordinateTolerance {
		t.Errorf(
			"longitude projetée = %.12f°, observation VR = %.12f°, écart = %.6f seconde d'arc",
			projected.Longitude,
			observedAfterFourMinutes.Longitude,
			difference*3600,
		)
	}
}

func TestVirtualRegattaReferenceWindState(t *testing.T) {
	// Ces composantes correspondent à un vent de 4,4 nœuds venant de 222°,
	// soit l'observation VR au cap 321° avec un TWA de 99° bâbord.
	wind, err := WindFromComponents(
		1.514614301415851,
		1.682149598291720,
	)
	if err != nil {
		t.Fatalf(
			"WindFromComponents() a retourné une erreur: %v",
			err,
		)
	}

	speedInKnots, err := KnotsFromMetersPerSecond(wind.Speed)
	if err != nil {
		t.Fatalf(
			"KnotsFromMetersPerSecond() a retourné une erreur: %v",
			err,
		)
	}
	if difference := math.Abs(
		float64(speedInKnots - 4.4),
	); difference > 1e-12 {
		t.Errorf(
			"TWS = %.12f nd, attendu 4.4 nd",
			speedInKnots,
		)
	}
	if difference := math.Abs(
		float64(wind.Direction - 222),
	); difference > 1e-12 {
		t.Errorf(
			"direction = %.12f°, attendue 222°",
			wind.Direction,
		)
	}

	angle, err := TrueWindAngleFromHeading(321, wind.Direction)
	if err != nil {
		t.Fatalf(
			"TrueWindAngleFromHeading() a retourné une erreur: %v",
			err,
		)
	}
	if difference := math.Abs(
		float64(angle - 99),
	); difference > 1e-12 {
		t.Errorf("TWA = %.12f°, attendu 99°", angle)
	}

	tack, err := TackFromHeading(321, wind.Direction)
	if err != nil {
		t.Fatalf(
			"TackFromHeading() a retourné une erreur: %v",
			err,
		)
	}
	if tack != TackPort {
		t.Errorf("amure = %q, attendue %q", tack, TackPort)
	}
}

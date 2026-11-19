package httpapi

import (
	"strings"
	"testing"
	"time"
)

func TestDepartureBetterRanksArrivalThenGatesThenDistance(t *testing.T) {
	base := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	arrivedSlow := departureOutcome{Departure: base, Reached: true, ElapsedHours: 60}
	arrivedFast := departureOutcome{Departure: base.Add(6 * time.Hour), Reached: true, ElapsedHours: 50}
	twoGatesFar := departureOutcome{Departure: base, GatesCrossed: 2, DistanceNM: 900}
	oneGateNear := departureOutcome{Departure: base, GatesCrossed: 1, DistanceNM: 10}
	oneGateFar := departureOutcome{Departure: base, GatesCrossed: 1, DistanceNM: 400}

	cases := []struct {
		name string
		a, b departureOutcome
	}{
		{"l'arrivée l'emporte sur les portes", arrivedSlow, twoGatesFar},
		{"arrivé plus vite", arrivedFast, arrivedSlow},
		{"plus de portes malgré la distance", twoGatesFar, oneGateNear},
		{"plus près de la porte suivante", oneGateNear, oneGateFar},
	}
	for _, tc := range cases {
		if !departureBetter(tc.a, tc.b) || departureBetter(tc.b, tc.a) {
			t.Fatalf("%s : classement inattendu", tc.name)
		}
	}
	outcomes := []departureOutcome{oneGateFar, twoGatesFar, oneGateNear, arrivedSlow, arrivedFast}
	if best := bestDeparture(outcomes); best != 4 {
		t.Fatalf("meilleur départ = %d, attendu 4", best)
	}
	if best := bestDeparture(nil); best != -1 {
		t.Fatalf("sans départ : %d, attendu -1", best)
	}
}

func TestDepartureBetterKeepsTheEarlierDepartureOnATie(t *testing.T) {
	base := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	early := departureOutcome{Departure: base, GatesCrossed: 1, DistanceNM: 100}
	late := departureOutcome{Departure: base.Add(6 * time.Hour), GatesCrossed: 1, DistanceNM: 100}
	if !departureBetter(early, late) || departureBetter(late, early) {
		t.Fatal("à égalité, le départ le plus tôt doit l'emporter")
	}
}

func TestDepartureInstantsAlignOnTheStepAndStayInTheWindow(t *testing.T) {
	from := time.Date(2026, 11, 20, 7, 30, 0, 0, time.UTC)
	until := time.Date(2026, 11, 21, 6, 0, 0, 0, time.UTC)
	got := departureInstants(from, until, 6*time.Hour)
	want := []string{"2026-11-20T12:00:00Z", "2026-11-20T18:00:00Z", "2026-11-21T00:00:00Z", "2026-11-21T06:00:00Z"}
	if len(got) != len(want) {
		t.Fatalf("%d départs, attendu %d : %v", len(got), len(want), got)
	}
	for index, instant := range got {
		if instant.Format(time.RFC3339) != want[index] {
			t.Fatalf("départ %d = %s, attendu %s", index, instant.Format(time.RFC3339), want[index])
		}
	}
	if long := departureInstants(from, from.Add(1000*time.Hour), time.Hour); len(long) != maxDepartureCount {
		t.Fatalf("%d départs, attendu au plus %d", len(long), maxDepartureCount)
	}
	if none := departureInstants(until, from, 6*time.Hour); len(none) != 0 {
		t.Fatalf("plage inversée : %v", none)
	}
}

func TestDepartureWindowBounds(t *testing.T) {
	now := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	covered := now.Add(16 * 24 * time.Hour)
	millis := func(value time.Time) *int64 { ms := value.UnixMilli(); return &ms }

	from, until, err := departureWindowBounds(now, nil, covered, 72*time.Hour)
	if err != nil || !from.Equal(now) || !until.Equal(covered.Add(-72*time.Hour)) {
		t.Fatalf("sans fenêtre : %s → %s (%v)", from, until, err)
	}

	open := now.Add(24 * time.Hour)
	closing := now.Add(5 * 24 * time.Hour)
	from, until, err = departureWindowBounds(now, &vrStateStartWindow{Open: millis(open), Close: millis(closing)}, covered, 72*time.Hour)
	if err != nil || !from.Equal(open) || !until.Equal(closing) {
		t.Fatalf("fenêtre du leg : %s → %s (%v)", from, until, err)
	}

	_, _, err = departureWindowBounds(now, nil, now.Add(48*time.Hour), 72*time.Hour)
	if err == nil || !strings.Contains(err.Error(), "réduisez la durée comparée") {
		t.Fatalf("prévisions trop courtes : erreur %v", err)
	}

	_, _, err = departureWindowBounds(now, &vrStateStartWindow{Close: millis(now.Add(-time.Hour))}, covered, 72*time.Hour)
	if err == nil || !strings.Contains(err.Error(), "fermé") {
		t.Fatalf("leg fermé : erreur %v", err)
	}
}

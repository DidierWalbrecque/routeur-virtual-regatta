package simulation

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/equipment"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
)

type weatherCall struct {
	position navigation.Position
	instant  time.Time
}

type testWeatherProvider struct {
	calls  []weatherCall
	wind   navigation.Wind
	failAt int
}

func (provider *testWeatherProvider) WindAt(
	_ context.Context,
	position navigation.Position,
	instant time.Time,
) (navigation.Wind, error) {
	provider.calls = append(provider.calls, weatherCall{
		position: position,
		instant:  instant,
	})
	if provider.failAt > 0 && len(provider.calls) == provider.failAt {
		return navigation.Wind{}, errors.New("météo indisponible")
	}
	return provider.wind, nil
}

func TestEvaluateStep(t *testing.T) {
	at := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)
	position := navigation.Position{
		Latitude:  13.121111,
		Longitude: 121.586667,
	}
	provider := &testWeatherProvider{
		wind: navigation.Wind{
			Speed:     5,
			Direction: 180,
		},
	}

	result, err := EvaluateStep(
		context.Background(),
		provider,
		testPolar(),
		StepInput{
			Position: position,
			Instant:  at,
			Heading:  90,
			Duration: 30 * time.Minute,
		},
	)
	if err != nil {
		t.Fatalf("calcul refusé: %v", err)
	}

	wantWindKnots, err := navigation.KnotsFromMetersPerSecond(5)
	if err != nil {
		t.Fatalf("conversion du vent attendue impossible: %v", err)
	}
	assertClose(
		t,
		"vent en nœuds",
		float64(result.WindKnots),
		float64(wantWindKnots),
	)
	assertClose(t, "TWA", float64(result.WindAngle), 90)
	if result.Tack != navigation.TackStarboard {
		t.Fatalf("amure = %q, attendue tribord", result.Tack)
	}
	if result.BestSail.SailName != "Jib" {
		t.Fatalf("voile = %q, attendue Jib", result.BestSail.SailName)
	}
	assertClose(t, "vitesse", float64(result.BestSail.Speed), 8)
	assertClose(t, "distance", float64(result.Distance), 4)
	if result.Position != position {
		t.Fatal("la position initiale du pas n'est pas conservée")
	}
	if result.NextPosition == position {
		t.Fatal("la position suivante est identique à la position initiale")
	}
	wantNextAt := at.Add(30 * time.Minute)
	if !result.NextInstant.Equal(wantNextAt) {
		t.Fatalf(
			"instant suivant = %s, attendu %s",
			result.NextInstant,
			wantNextAt,
		)
	}
}

func TestEvaluateStepUsesAutomaticSailWhenFixedSailIDIsZero(t *testing.T) {
	provider := &testWeatherProvider{
		wind: navigation.Wind{Speed: 5, Direction: 180},
	}

	result, err := EvaluateStep(
		context.Background(),
		provider,
		testPolar(),
		StepInput{
			Position:    navigation.Position{},
			Instant:     time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
			Heading:     90,
			Duration:    10 * time.Minute,
			FixedSailID: 0,
		},
	)
	if err != nil {
		t.Fatalf("calcul refusé: %v", err)
	}
	if result.BestSail.SailID != 1 {
		t.Fatalf("voile = %d, attendue 1 en mode automatique", result.BestSail.SailID)
	}
}

func TestEvaluateStepUsesFixedSail(t *testing.T) {
	provider := &testWeatherProvider{
		wind: navigation.Wind{Speed: 5, Direction: 180},
	}

	result, err := EvaluateStep(
		context.Background(),
		provider,
		testPolar(),
		StepInput{
			Position:    navigation.Position{},
			Instant:     time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
			Heading:     90,
			Duration:    10 * time.Minute,
			FixedSailID: 2,
		},
	)
	if err != nil {
		t.Fatalf("calcul refusé: %v", err)
	}
	if result.BestSail.SailID != 2 {
		t.Fatalf("voile = %d, attendue 2", result.BestSail.SailID)
	}
	assertClose(t, "vitesse de la voile fixe", float64(result.BestSail.Speed), 7)
}

func TestEvaluateStepFixedSailKeepsEffectiveSpeedOptions(t *testing.T) {
	provider := &testWeatherProvider{
		wind: navigation.Wind{Speed: 5, Direction: 180},
	}
	boatPolar := testPolar()
	boatPolar.GlobalSpeedRatio = 1.1
	boatPolar.Foil = polar.FoilProfile{
		MaxSpeedRatio: 1.2,
		TWAMin:        0,
		TWAMax:        180,
		TWSMin:        0,
		TWSMax:        20,
	}
	boatPolar.Hull = polar.HullProfile{SpeedRatio: 1.05}
	owned, err := equipment.NewSet(equipment.Foils, equipment.HullPolish)
	if err != nil {
		t.Fatalf("équipements de test invalides: %v", err)
	}

	result, err := EvaluateStep(
		context.Background(),
		provider,
		boatPolar,
		StepInput{
			Position:    navigation.Position{},
			Instant:     time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
			Heading:     90,
			Duration:    10 * time.Minute,
			Owned:       owned,
			FixedSailID: 2,
		},
	)
	if err != nil {
		t.Fatalf("calcul refusé: %v", err)
	}
	if result.BestSail.SailID != 2 {
		t.Fatalf("voile = %d, attendue 2", result.BestSail.SailID)
	}
	assertClose(
		t,
		"vitesse effective de la voile fixe",
		float64(result.BestSail.Speed),
		7*1.1*1.2*1.05,
	)
}

func TestEvaluateStepRejectsInvalidFixedSail(t *testing.T) {
	base := StepInput{
		Position: navigation.Position{},
		Instant:  time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
		Heading:  90,
		Duration: 10 * time.Minute,
	}
	provider := &testWeatherProvider{
		wind: navigation.Wind{Speed: 5, Direction: 180},
	}

	for _, test := range []struct {
		name      string
		boatPolar *polar.Polar
		fixedSail polar.SailID
		wantErr   string
	}{
		{
			name:      "identifiant négatif",
			boatPolar: testPolar(),
			fixedSail: -1,
			wantErr:   "identifiant de voile fixe positif ou nul",
		},
		{
			name:      "voile inconnue",
			boatPolar: testPolar(),
			fixedSail: 99,
			wantErr:   "voile fixe 99 inconnue",
		},
		{
			name:      "voile indisponible",
			boatPolar: equipmentAwarePolar(),
			fixedSail: 3,
			wantErr:   "voile fixe 3 indisponible avec les équipements possédés",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := base
			input.FixedSailID = test.fixedSail
			_, err := EvaluateStep(
				context.Background(),
				provider,
				test.boatPolar,
				input,
			)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %v, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

func TestEvaluateStepRejectsMissingDependencies(t *testing.T) {
	input := StepInput{
		Position: navigation.Position{},
		Instant:  time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
		Heading:  90,
		Duration: 30 * time.Minute,
	}
	provider := &testWeatherProvider{
		wind: navigation.Wind{Speed: 5, Direction: 180},
	}

	if _, err := EvaluateStep(
		context.Background(),
		nil,
		testPolar(),
		input,
	); err == nil || !strings.Contains(err.Error(), "fournisseur météo") {
		t.Fatalf("erreur fournisseur = %v", err)
	}
	if _, err := EvaluateStep(
		context.Background(),
		provider,
		nil,
		input,
	); err == nil || !strings.Contains(err.Error(), "polaire") {
		t.Fatalf("erreur polaire = %v", err)
	}
}

func TestRunRecalculatesEveryStep(t *testing.T) {
	start := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)
	position := navigation.Position{
		Latitude:  13.121111,
		Longitude: 121.586667,
	}
	provider := &testWeatherProvider{
		wind: navigation.Wind{
			Speed:     5,
			Direction: 180,
		},
	}

	result, err := Run(
		context.Background(),
		provider,
		testPolar(),
		Input{
			Position:     position,
			Instant:      start,
			Heading:      90,
			Duration:     25 * time.Minute,
			StepDuration: 10 * time.Minute,
		},
	)
	if err != nil {
		t.Fatalf("simulation refusée: %v", err)
	}

	if len(result.Steps) != 3 {
		t.Fatalf("nombre de pas = %d, attendu 3", len(result.Steps))
	}
	if len(provider.calls) != 3 {
		t.Fatalf("appels météo = %d, attendus 3", len(provider.calls))
	}

	wantInstants := []time.Time{
		start,
		start.Add(10 * time.Minute),
		start.Add(20 * time.Minute),
	}
	for index, want := range wantInstants {
		if !provider.calls[index].instant.Equal(want) {
			t.Errorf(
				"instant météo du pas %d = %s, attendu %s",
				index+1,
				provider.calls[index].instant,
				want,
			)
		}
	}
	if provider.calls[1].position == provider.calls[0].position ||
		provider.calls[2].position == provider.calls[1].position {
		t.Fatal("la météo doit être recalculée aux positions successives")
	}

	wantDurations := []time.Duration{
		10 * time.Minute,
		10 * time.Minute,
		5 * time.Minute,
	}
	for index, want := range wantDurations {
		if got := result.Steps[index].Duration; got != want {
			t.Errorf(
				"durée du pas %d = %s, attendue %s",
				index+1,
				got,
				want,
			)
		}
	}

	assertClose(
		t,
		"distance totale",
		float64(result.Distance),
		8*25.0/60.0,
	)
	wantEnd := start.Add(25 * time.Minute)
	if !result.NextInstant.Equal(wantEnd) {
		t.Errorf(
			"instant final = %s, attendu %s",
			result.NextInstant,
			wantEnd,
		)
	}
	if result.NextPosition != result.Steps[2].NextPosition {
		t.Fatal("la position finale ne correspond pas au dernier pas")
	}
}

func TestRunKeepsFixedSailAcrossSteps(t *testing.T) {
	start := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)
	provider := &sequenceWeatherProvider{
		winds: []navigation.Wind{
			{Speed: 2.57222, Direction: 180},
			{Speed: 7.71666, Direction: 180},
		},
	}

	result, err := Run(
		context.Background(),
		provider,
		changingBestSailPolar(),
		Input{
			Position:     navigation.Position{},
			Instant:      start,
			Heading:      90,
			Duration:     20 * time.Minute,
			StepDuration: 10 * time.Minute,
			FixedSailID:  1,
		},
	)
	if err != nil {
		t.Fatalf("simulation refusée: %v", err)
	}
	for index, step := range result.Steps {
		if step.BestSail.SailID != 1 {
			t.Errorf("voile du pas %d = %d, attendue 1", index+1, step.BestSail.SailID)
		}
	}
}

func TestRunUsesOneShortStep(t *testing.T) {
	provider := &testWeatherProvider{
		wind: navigation.Wind{Speed: 5, Direction: 180},
	}
	start := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)

	result, err := Run(
		context.Background(),
		provider,
		testPolar(),
		Input{
			Position:     navigation.Position{},
			Instant:      start,
			Heading:      90,
			Duration:     5 * time.Minute,
			StepDuration: 10 * time.Minute,
		},
	)
	if err != nil {
		t.Fatalf("simulation refusée: %v", err)
	}
	if len(result.Steps) != 1 {
		t.Fatalf("nombre de pas = %d, attendu 1", len(result.Steps))
	}
	if got := result.NextInstant.Sub(start); got != 5*time.Minute {
		t.Fatalf("durée simulée = %s, attendue 5m", got)
	}
}

func TestRunRejectsInvalidDurations(t *testing.T) {
	provider := &testWeatherProvider{
		wind: navigation.Wind{Speed: 5, Direction: 180},
	}
	base := Input{
		Position:     navigation.Position{},
		Instant:      time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
		Heading:      90,
		Duration:     10 * time.Minute,
		StepDuration: 5 * time.Minute,
	}

	for _, test := range []struct {
		name     string
		duration time.Duration
		step     time.Duration
		wantErr  string
	}{
		{
			name:     "durée totale nulle",
			duration: 0,
			step:     5 * time.Minute,
			wantErr:  "durée totale",
		},
		{
			name:     "pas nul",
			duration: 10 * time.Minute,
			step:     0,
			wantErr:  "durée d'un pas",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := base
			input.Duration = test.duration
			input.StepDuration = test.step
			_, err := Run(
				context.Background(),
				provider,
				testPolar(),
				input,
			)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %v, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

func TestRunReportsFailingStep(t *testing.T) {
	provider := &testWeatherProvider{
		wind:   navigation.Wind{Speed: 5, Direction: 180},
		failAt: 2,
	}

	_, err := Run(
		context.Background(),
		provider,
		testPolar(),
		Input{
			Position:     navigation.Position{},
			Instant:      time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
			Heading:      90,
			Duration:     20 * time.Minute,
			StepDuration: 10 * time.Minute,
		},
	)
	if err == nil || !strings.Contains(err.Error(), "pas 2") {
		t.Fatalf("erreur = %v, doit identifier le pas 2", err)
	}
}

func TestRunPropagatesFixedSailError(t *testing.T) {
	_, err := Run(
		context.Background(),
		&testWeatherProvider{wind: navigation.Wind{Speed: 5, Direction: 180}},
		testPolar(),
		Input{
			Position:     navigation.Position{},
			Instant:      time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC),
			Heading:      90,
			Duration:     10 * time.Minute,
			StepDuration: 10 * time.Minute,
			FixedSailID:  -1,
		},
	)
	if err == nil || !strings.Contains(err.Error(), "pas 1") ||
		!strings.Contains(err.Error(), "identifiant de voile fixe") {
		t.Fatalf("erreur = %v, doit propager l'erreur de voile fixe", err)
	}
}

func testPolar() *polar.Polar {
	return &polar.Polar{
		Label:            "Test boat",
		GlobalSpeedRatio: 1,
		WindSpeeds:       []polar.WindSpeed{0, 20},
		WindAngles:       []polar.TrueWindAngle{0, 180},
		Sails: []polar.Sail{
			{
				ID:   1,
				Name: "Jib",
				Speeds: polar.SpeedGrid{
					Values: [][]polar.BoatSpeed{
						{8, 8},
						{8, 8},
					},
				},
			},
			{
				ID:   2,
				Name: "Spi",
				Speeds: polar.SpeedGrid{
					Values: [][]polar.BoatSpeed{
						{7, 7},
						{7, 7},
					},
				},
			},
		},
	}
}

type sequenceWeatherProvider struct {
	winds []navigation.Wind
	calls int
}

func (provider *sequenceWeatherProvider) WindAt(
	_ context.Context,
	_ navigation.Position,
	_ time.Time,
) (navigation.Wind, error) {
	windIndex := min(provider.calls, len(provider.winds)-1)
	provider.calls++
	return provider.winds[windIndex], nil
}

func equipmentAwarePolar() *polar.Polar {
	boatPolar := testPolar()
	boatPolar.Sails = append(boatPolar.Sails, polar.Sail{
		ID:   3,
		Name: "LightJib",
		Speeds: polar.SpeedGrid{Values: [][]polar.BoatSpeed{
			{9, 9},
			{9, 9},
		}},
	})
	return boatPolar
}

func changingBestSailPolar() *polar.Polar {
	return &polar.Polar{
		Label:            "Changing best sail",
		GlobalSpeedRatio: 1,
		WindSpeeds:       []polar.WindSpeed{0, 20},
		WindAngles:       []polar.TrueWindAngle{0, 180},
		Sails: []polar.Sail{
			{
				ID:   1,
				Name: "Jib",
				Speeds: polar.SpeedGrid{Values: [][]polar.BoatSpeed{
					{10, 10},
					{5, 5},
				}},
			},
			{
				ID:   2,
				Name: "Spi",
				Speeds: polar.SpeedGrid{Values: [][]polar.BoatSpeed{
					{5, 5},
					{10, 10},
				}},
			},
		},
	}
}

func assertClose(
	t *testing.T,
	name string,
	got,
	want float64,
) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s = %.12f, attendu %.12f", name, got, want)
	}
}

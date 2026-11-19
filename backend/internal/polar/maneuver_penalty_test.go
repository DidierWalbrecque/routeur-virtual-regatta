package polar

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestWinchProfileTackPenaltyAt(t *testing.T) {
	profile := testWinchProfile()
	tests := []struct {
		name         string
		windSpeed    WindSpeed
		professional bool
		wantRatio    SpeedRatio
		wantDuration time.Duration
	}{
		{
			name:         "standard sous le seuil faible",
			windSpeed:    5,
			wantRatio:    0.5,
			wantDuration: 3 * time.Minute,
		},
		{
			name:         "standard au seuil faible",
			windSpeed:    10,
			wantRatio:    0.5,
			wantDuration: 3 * time.Minute,
		},
		{
			name:         "standard entre les seuils",
			windSpeed:    20,
			wantRatio:    0.45,
			wantDuration: 4 * time.Minute,
		},
		{
			name:         "standard au seuil fort",
			windSpeed:    30,
			wantRatio:    0.4,
			wantDuration: 5 * time.Minute,
		},
		{
			name:         "standard au-dessus du seuil fort",
			windSpeed:    40,
			wantRatio:    0.4,
			wantDuration: 5 * time.Minute,
		},
		{
			name:         "professionnel entre les seuils",
			windSpeed:    20,
			professional: true,
			wantRatio:    0.65,
			wantDuration: 90 * time.Second,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := profile.TackPenaltyAt(
				test.windSpeed,
				test.professional,
			)
			if err != nil {
				t.Fatalf("calcul refusé: %v", err)
			}
			if math.Abs(float64(got.SpeedRatio-test.wantRatio)) > 1e-12 {
				t.Errorf(
					"ratio = %.12f, attendu %.12f",
					got.SpeedRatio,
					test.wantRatio,
				)
			}
			if got.Duration != test.wantDuration {
				t.Errorf(
					"durée = %s, attendue %s",
					got.Duration,
					test.wantDuration,
				)
			}
		})
	}
}

func TestWinchProfilePenaltyMethodsSelectManeuver(t *testing.T) {
	profile := testWinchProfile()
	profile.Gybe = ManeuverProfile{
		Standard: &WindDependentPenalty{
			LowWind: ManeuverPenalty{
				SpeedRatio: 0.6,
				Duration:   2 * time.Minute,
			},
			HighWind: ManeuverPenalty{
				SpeedRatio: 0.5,
				Duration:   4 * time.Minute,
			},
		},
	}
	profile.SailChange = ManeuverProfile{
		Standard: &WindDependentPenalty{
			LowWind: ManeuverPenalty{
				SpeedRatio: 0.4,
				Duration:   5 * time.Minute,
			},
			HighWind: ManeuverPenalty{
				SpeedRatio: 0.3,
				Duration:   8 * time.Minute,
			},
		},
	}

	gybe, err := profile.GybePenaltyAt(10, false)
	if err != nil {
		t.Fatalf("empannage refusé: %v", err)
	}
	if gybe.Duration != 2*time.Minute || gybe.SpeedRatio != 0.6 {
		t.Errorf("pénalité d'empannage = %+v", gybe)
	}

	sailChange, err := profile.SailChangePenaltyAt(30, false)
	if err != nil {
		t.Fatalf("changement de voile refusé: %v", err)
	}
	if sailChange.Duration != 8*time.Minute ||
		sailChange.SpeedRatio != 0.3 {
		t.Errorf("pénalité de changement de voile = %+v", sailChange)
	}
}

func TestWinchProfileUsesLegacyPenalty(t *testing.T) {
	profile := WinchProfile{
		Tack: ManeuverProfile{
			LegacyStd: ManeuverPenalty{
				SpeedRatio: 0.5,
				Duration:   5 * time.Minute,
			},
			LegacyPro: ManeuverPenalty{
				SpeedRatio: 0.7,
				Duration:   75 * time.Second,
			},
		},
	}

	standard, err := profile.TackPenaltyAt(20, false)
	if err != nil {
		t.Fatalf("pénalité historique standard refusée: %v", err)
	}
	if standard != profile.Tack.LegacyStd {
		t.Errorf(
			"pénalité standard = %+v, attendue %+v",
			standard,
			profile.Tack.LegacyStd,
		)
	}

	professional, err := profile.TackPenaltyAt(20, true)
	if err != nil {
		t.Fatalf("pénalité historique pro refusée: %v", err)
	}
	if professional != profile.Tack.LegacyPro {
		t.Errorf(
			"pénalité pro = %+v, attendue %+v",
			professional,
			profile.Tack.LegacyPro,
		)
	}
}

func TestWinchProfilePenaltyAtRejectsInvalidInputs(t *testing.T) {
	valid := testWinchProfile()
	tests := []struct {
		name         string
		windSpeed    WindSpeed
		professional bool
		mutate       func(*WinchProfile)
		wantErr      string
	}{
		{
			name:      "vent négatif",
			windSpeed: -1,
			mutate:    func(*WinchProfile) {},
			wantErr:   "vitesse du vent",
		},
		{
			name:      "vent non fini",
			windSpeed: WindSpeed(math.NaN()),
			mutate:    func(*WinchProfile) {},
			wantErr:   "vitesse du vent",
		},
		{
			name:      "seuils inversés",
			windSpeed: 20,
			mutate: func(profile *WinchProfile) {
				profile.HighWindThreshold = 10
			},
			wantErr: "seuils de vent",
		},
		{
			name:      "ratio faible invalide",
			windSpeed: 20,
			mutate: func(profile *WinchProfile) {
				profile.Tack.Standard.LowWind.SpeedRatio = 1.1
			},
			wantErr: "vent faible",
		},
		{
			name:      "durée forte invalide",
			windSpeed: 20,
			mutate: func(profile *WinchProfile) {
				profile.Tack.Standard.HighWind.Duration = 0
			},
			wantErr: "vent fort",
		},
		{
			name:         "profil professionnel absent",
			windSpeed:    20,
			professional: true,
			mutate: func(profile *WinchProfile) {
				profile.Tack.Professional = nil
				profile.Tack.LegacyPro = ManeuverPenalty{}
			},
			wantErr: "historique invalide",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			profile := valid
			standard := *valid.Tack.Standard
			professional := *valid.Tack.Professional
			profile.Tack.Standard = &standard
			profile.Tack.Professional = &professional
			test.mutate(&profile)

			_, err := profile.TackPenaltyAt(
				test.windSpeed,
				test.professional,
			)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %v, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

func TestUltimWinchPenaltiesAtReferenceWind(t *testing.T) {
	boatPolar, err := LoadFile(
		"../../../data/polars/ultim_bp_11.json",
	)
	if err != nil {
		t.Fatalf("chargement de la polaire impossible: %v", err)
	}

	standard, err := boatPolar.Winch.TackPenaltyAt(20, false)
	if err != nil {
		t.Fatalf("pénalité standard refusée: %v", err)
	}
	if standard.SpeedRatio != 0.5 ||
		standard.Duration != 4*time.Minute {
		t.Errorf("pénalité standard = %+v", standard)
	}

	professional, err := boatPolar.Winch.TackPenaltyAt(20, true)
	if err != nil {
		t.Fatalf("pénalité pro refusée: %v", err)
	}
	if professional.SpeedRatio != 0.7 ||
		professional.Duration != 4*time.Minute {
		t.Errorf("pénalité pro = %+v", professional)
	}

	sailChange, err := boatPolar.Winch.SailChangePenaltyAt(20, false)
	if err != nil {
		t.Fatalf("changement de voile refusé: %v", err)
	}
	if sailChange.SpeedRatio != 0.5 ||
		sailChange.Duration != 390*time.Second {
		t.Errorf("changement de voile = %+v", sailChange)
	}
}

func testWinchProfile() WinchProfile {
	return WinchProfile{
		LowWindThreshold:  10,
		HighWindThreshold: 30,
		Tack: ManeuverProfile{
			Standard: &WindDependentPenalty{
				LowWind: ManeuverPenalty{
					SpeedRatio: 0.5,
					Duration:   3 * time.Minute,
				},
				HighWind: ManeuverPenalty{
					SpeedRatio: 0.4,
					Duration:   5 * time.Minute,
				},
			},
			Professional: &WindDependentPenalty{
				LowWind: ManeuverPenalty{
					SpeedRatio: 0.7,
					Duration:   time.Minute,
				},
				HighWind: ManeuverPenalty{
					SpeedRatio: 0.6,
					Duration:   2 * time.Minute,
				},
			},
		},
	}
}

func TestWinchProfileTackPenaltyUsesCosineInterpolation(t *testing.T) {
	profile := testWinchProfile() // lws 10, hws 30 ; vent faible 180 s, vent fort 300 s

	// Au milieu (20 kn), cosinus et linéaire coïncident.
	mid, err := profile.TackPenaltyAt(20, false)
	if err != nil {
		t.Fatalf("calcul refusé: %v", err)
	}
	if mid.Duration != 4*time.Minute {
		t.Errorf("durée à 20 kn = %s, attendue 4m (point milieu)", mid.Duration)
	}

	// Sous le milieu (15 kn), le cosinus reste plus proche du vent faible que
	// l'interpolation linéaire (qui donnerait 210 s).
	low, err := profile.TackPenaltyAt(15, false)
	if err != nil {
		t.Fatalf("calcul refusé: %v", err)
	}
	if !(low.Duration > 3*time.Minute && low.Duration < 210*time.Second) {
		t.Errorf("durée à 15 kn = %s, attendue dans ]180s, 210s[ (cosinus < linéaire)", low.Duration)
	}

	// Au-dessus du milieu (25 kn), le cosinus dépasse le linéaire (270 s).
	high, err := profile.TackPenaltyAt(25, false)
	if err != nil {
		t.Fatalf("calcul refusé: %v", err)
	}
	if !(high.Duration > 270*time.Second && high.Duration < 5*time.Minute) {
		t.Errorf("durée à 25 kn = %s, attendue dans ]270s, 300s[ (cosinus > linéaire)", high.Duration)
	}
}

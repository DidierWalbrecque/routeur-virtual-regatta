package polar

import (
	"errors"
	"fmt"
	"math"
	"time"
)

// TackPenaltyAt returns the tack penalty for the requested wind speed and
// winch equipment.
func (profile WinchProfile) TackPenaltyAt(
	windSpeed WindSpeed,
	professional bool,
) (ManeuverPenalty, error) {
	return profile.penaltyAt(
		"virement",
		profile.Tack,
		windSpeed,
		professional,
	)
}

// GybePenaltyAt returns the gybe penalty for the requested wind speed and
// winch equipment.
func (profile WinchProfile) GybePenaltyAt(
	windSpeed WindSpeed,
	professional bool,
) (ManeuverPenalty, error) {
	return profile.penaltyAt(
		"empannage",
		profile.Gybe,
		windSpeed,
		professional,
	)
}

// SailChangePenaltyAt returns the sail-change penalty for the requested wind
// speed and winch equipment.
func (profile WinchProfile) SailChangePenaltyAt(
	windSpeed WindSpeed,
	professional bool,
) (ManeuverPenalty, error) {
	return profile.penaltyAt(
		"changement de voile",
		profile.SailChange,
		windSpeed,
		professional,
	)
}

func (profile WinchProfile) penaltyAt(
	name string,
	maneuver ManeuverProfile,
	windSpeed WindSpeed,
	professional bool,
) (ManeuverPenalty, error) {
	if err := validateManeuverWindSpeed(windSpeed); err != nil {
		return ManeuverPenalty{}, fmt.Errorf(
			"polar: pénalité de %s impossible: %w",
			name,
			err,
		)
	}

	detailed := maneuver.Standard
	legacy := maneuver.LegacyStd
	if professional {
		detailed = maneuver.Professional
		legacy = maneuver.LegacyPro
	}
	if detailed == nil {
		if err := validateManeuverPenalty(legacy); err != nil {
			return ManeuverPenalty{}, fmt.Errorf(
				"polar: pénalité de %s historique invalide: %w",
				name,
				err,
			)
		}
		return legacy, nil
	}

	if err := validateManeuverThresholds(
		profile.LowWindThreshold,
		profile.HighWindThreshold,
	); err != nil {
		return ManeuverPenalty{}, fmt.Errorf(
			"polar: pénalité de %s impossible: %w",
			name,
			err,
		)
	}
	if err := validateManeuverPenalty(detailed.LowWind); err != nil {
		return ManeuverPenalty{}, fmt.Errorf(
			"polar: pénalité de %s par vent faible invalide: %w",
			name,
			err,
		)
	}
	if err := validateManeuverPenalty(detailed.HighWind); err != nil {
		return ManeuverPenalty{}, fmt.Errorf(
			"polar: pénalité de %s par vent fort invalide: %w",
			name,
			err,
		)
	}

	switch {
	case windSpeed <= profile.LowWindThreshold:
		return detailed.LowWind, nil
	case windSpeed >= profile.HighWindThreshold:
		return detailed.HighWind, nil
	}

	linearPosition := float64(
		(windSpeed - profile.LowWindThreshold) /
			(profile.HighWindThreshold - profile.LowWindThreshold),
	)
	// Virtual Regatta interpole le timer winch en cosinus (ease-in-out) entre
	// les seuils de vent faible et fort, non linéairement.
	position := 0.5 - math.Cos(linearPosition*math.Pi)*0.5
	result := ManeuverPenalty{
		SpeedRatio: SpeedRatio(
			float64(detailed.LowWind.SpeedRatio) +
				position*float64(
					detailed.HighWind.SpeedRatio-
						detailed.LowWind.SpeedRatio,
				),
		),
		Duration: time.Duration(
			float64(detailed.LowWind.Duration) +
				position*float64(
					detailed.HighWind.Duration-
						detailed.LowWind.Duration,
				),
		),
	}
	if err := validateManeuverPenalty(result); err != nil {
		return ManeuverPenalty{}, fmt.Errorf(
			"polar: pénalité de %s interpolée invalide: %w",
			name,
			err,
		)
	}
	return result, nil
}

func validateManeuverWindSpeed(windSpeed WindSpeed) error {
	if !isFinite(float64(windSpeed)) || windSpeed < 0 {
		return errors.New(
			"vitesse du vent finie positive ou nulle requise",
		)
	}
	return nil
}

func validateManeuverThresholds(
	low,
	high WindSpeed,
) error {
	if !isFinite(float64(low)) ||
		!isFinite(float64(high)) ||
		low < 0 ||
		high <= low {
		return errors.New(
			"seuils de vent finis requis avec 0 <= seuil faible < seuil fort",
		)
	}
	return nil
}

func validateManeuverPenalty(penalty ManeuverPenalty) error {
	ratio := float64(penalty.SpeedRatio)
	if !isFinite(ratio) || ratio < 0 || ratio > 1 {
		return errors.New(
			"coefficient de vitesse fini compris entre 0 et 1 requis",
		)
	}
	if penalty.Duration <= 0 {
		return errors.New(
			"durée strictement positive requise",
		)
	}
	return nil
}

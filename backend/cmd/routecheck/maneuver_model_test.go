package main

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
)

func TestParseManeuverModel(t *testing.T) {
	for _, value := range []maneuverModel{
		maneuverModelOff,
		maneuverModelExperimentalAdditive,
	} {
		got, err := parseManeuverModel(string(value))
		if err != nil {
			t.Fatalf("modèle %q refusé: %v", value, err)
		}
		if got != value {
			t.Errorf("modèle = %q, attendu %q", got, value)
		}
	}

	_, err := parseManeuverModel("official")
	if err == nil || !strings.Contains(err.Error(), "-maneuver-model") {
		t.Fatalf("erreur = %v, option invalide attendue", err)
	}
}

func TestNewManeuverRuntimeOff(t *testing.T) {
	runtime, err := newManeuverRuntime(
		maneuverModelOff,
		polar.WinchProfile{},
		0,
	)
	if err != nil {
		t.Fatalf("modèle désactivé refusé: %v", err)
	}
	if runtime.evaluator != nil {
		t.Fatal("évaluateur inattendu lorsque le modèle est désactivé")
	}
	if runtime.combineSpeedPenalties != nil {
		t.Fatal("règle de cumul inattendue lorsque le modèle est désactivé")
	}
}

func TestNewManeuverRuntimeExperimentalAdditive(t *testing.T) {
	penalty := &polar.WindDependentPenalty{
		LowWind: polar.ManeuverPenalty{
			SpeedRatio: 0.5,
			Duration:   15 * time.Minute,
		},
		HighWind: polar.ManeuverPenalty{
			SpeedRatio: 0.5,
			Duration:   15 * time.Minute,
		},
	}
	runtime, err := newManeuverRuntime(
		maneuverModelExperimentalAdditive,
		polar.WinchProfile{
			LowWindThreshold:  10,
			HighWindThreshold: 30,
			Tack: polar.ManeuverProfile{
				Standard:     penalty,
				Professional: penalty,
			},
			SailChange: polar.ManeuverProfile{
				Standard:     penalty,
				Professional: penalty,
			},
		},
		12,
	)
	if err != nil {
		t.Fatalf("construction refusée: %v", err)
	}
	if runtime.evaluator == nil || runtime.combineSpeedPenalties == nil {
		t.Fatal("profil expérimental incomplet")
	}

	evaluation, err := runtime.evaluator.Evaluate(
		simulation.TransitionEvaluationRequest{
			Transition: simulation.ManeuverTransition{
				PreviousTack:      navigation.TackStarboard,
				NextTack:          navigation.TackPort,
				PreviousWindAngle: 60,
				NextWindAngle:     50,
				PreviousSailID:    1,
				NextSailID:        2,
			},
			WindSpeed: 20,
			Stamina:   simulation.StaminaState{Level: 100},
		},
	)
	if err != nil {
		t.Fatalf("évaluation refusée: %v", err)
	}
	if evaluation.TotalConsumption != 30 {
		t.Errorf(
			"consommation = %.3f, attendue 30",
			evaluation.TotalConsumption,
		)
	}
	if evaluation.NextStamina.Level != 70 {
		t.Errorf(
			"stamina suivante = %.3f, attendue 70",
			evaluation.NextStamina.Level,
		)
	}

	combined, err := evaluation.CombinedSpeedPenalty(
		runtime.combineSpeedPenalties,
	)
	if err != nil {
		t.Fatalf("cumul de pénalités refusé: %v", err)
	}
	if combined == nil {
		t.Fatal("pénalité combinée absente")
	}
	if combined.SpeedRatio != 0.25 {
		t.Errorf("ratio = %.3f, attendu 0.25", combined.SpeedRatio)
	}
	if combined.Duration != 30*time.Minute {
		t.Errorf("durée = %s, attendue 30m", combined.Duration)
	}
}

func TestNewManeuverRuntimeConfiguresStaminaRecovery(t *testing.T) {
	runtime, err := newManeuverRuntime(
		maneuverModelExperimentalAdditive,
		polar.WinchProfile{},
		12,
	)
	if err != nil {
		t.Fatalf("construction refusée: %v", err)
	}
	if runtime.staminaRecoveryRules == nil {
		t.Fatal("règles de récupération absentes")
	}
	if runtime.staminaRecoveryRules.PointsPerHour != 12 {
		t.Errorf(
			"récupération = %.3f, attendue 12",
			runtime.staminaRecoveryRules.PointsPerHour,
		)
	}

	_, err = newManeuverRuntime(
		maneuverModelOff,
		polar.WinchProfile{},
		12,
	)
	if err == nil || !strings.Contains(err.Error(), "-stamina-recovery-rate") {
		t.Fatalf("erreur = %v, combinaison invalide attendue", err)
	}
}

func TestParseOptionsAcceptsManeuverModel(t *testing.T) {
	base := []string{
		"-file", "f003.grb2",
		"-polar", "boat.json",
		"-position", "13°N 07'16\" - 121°E 35'12\"",
		"-destination", "14°N 00'00\" - 120°E 00'00\"",
		"-at", "2026-09-05T09:00:00Z",
		"-minutes", "60",
	}

	defaultOptions, err := parseOptions(base, io.Discard)
	if err != nil {
		t.Fatalf("arguments par défaut refusés: %v", err)
	}
	if defaultOptions.maneuverModel != maneuverModelOff {
		t.Errorf(
			"modèle par défaut = %q, attendu %q",
			defaultOptions.maneuverModel,
			maneuverModelOff,
		)
	}

	experimental, err := parseOptions(
		append(
			append([]string{}, base...),
			"-maneuver-model",
			string(maneuverModelExperimentalAdditive),
			"-stamina-recovery-rate",
			"12",
		),
		io.Discard,
	)
	if err != nil {
		t.Fatalf("modèle expérimental refusé: %v", err)
	}
	if experimental.maneuverModel != maneuverModelExperimentalAdditive {
		t.Errorf(
			"modèle = %q, attendu %q",
			experimental.maneuverModel,
			maneuverModelExperimentalAdditive,
		)
	}
	if experimental.staminaRecoveryRate != 12 {
		t.Errorf(
			"récupération = %.3f, attendue 12",
			experimental.staminaRecoveryRate,
		)
	}

	_, err = parseOptions(
		append(append([]string{}, base...), "-maneuver-model", "unknown"),
		io.Discard,
	)
	if err == nil || !strings.Contains(err.Error(), "-maneuver-model") {
		t.Fatalf("erreur = %v, modèle invalide attendu", err)
	}
}

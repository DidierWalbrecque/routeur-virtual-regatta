package main

import (
	"io"
	"strings"
	"testing"
)

func TestParseOptionsAcceptsInitialStamina(t *testing.T) {
	args := append(routecheckStaminaBaseArgs(), "-stamina", "73.5")
	options, err := parseOptions(args, io.Discard)
	if err != nil {
		t.Fatalf("arguments refusés: %v", err)
	}
	if options.stamina.Level != 73.5 {
		t.Errorf(
			"stamina initiale = %.6f, attendue 73.5",
			options.stamina.Level,
		)
	}
}

func TestParseOptionsDefaultsInitialStaminaToFull(t *testing.T) {
	options, err := parseOptions(routecheckStaminaBaseArgs(), io.Discard)
	if err != nil {
		t.Fatalf("arguments refusés: %v", err)
	}
	if options.stamina.Level != 100 {
		t.Errorf(
			"stamina initiale = %.6f, attendue 100",
			options.stamina.Level,
		)
	}
}

func TestParseOptionsRejectsInvalidInitialStamina(t *testing.T) {
	for _, value := range []string{"-1", "101", "NaN"} {
		t.Run(value, func(t *testing.T) {
			args := append(routecheckStaminaBaseArgs(), "-stamina", value)
			_, err := parseOptions(args, io.Discard)
			if err == nil || !strings.Contains(err.Error(), "-stamina") {
				t.Fatalf(
					"erreur = %v, doit signaler -stamina",
					err,
				)
			}
		})
	}
}

func routecheckStaminaBaseArgs() []string {
	return []string{
		"-file", "f003.grb2",
		"-polar", "boat.json",
		"-position", "13°N 07'16\" - 121°E 35'12\"",
		"-destination", "14°N 00'00\" - 120°E 00'00\"",
		"-at", "2026-09-05T09:00:00Z",
		"-minutes", "60",
	}
}

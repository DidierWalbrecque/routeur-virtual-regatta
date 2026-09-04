package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDisplaysCurrentPerformanceAndPurchases(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := run(
		[]string{
			"-polar", ultimPolarPath(),
			"-tws", "20",
			"-twa", "90",
			"-category", "6",
			"-budget", "440",
		},
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("run() a retourné une erreur inattendue: %v", err)
	}

	output := stdout.String()
	for _, expected := range []string{
		"Bateau : Ultim_BP_XI",
		"Équipements possédés : aucun",
		"Meilleure voile : Jib",
		"Vitesse attendue : 34.960000 nd",
		"1. code0 | achat 340 | personnel +0 | voile Code0",
		"gain +1.840000 nd",
	} {
		if !strings.Contains(output, expected) {
			t.Errorf("sortie absente %q dans:
%s", expected, output)
		}
	}
	if stderr.Len() != 0 {
		t.Errorf("sortie d'erreur inattendue: %s", stderr.String())
	}
}

func TestRunUsesOwnedEquipment(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := run(
		[]string{
			"-polar", ultimPolarPath(),
			"-tws", "20",
			"-twa", "90",
			"-category", "6",
			"-budget", "780",
			"-owned", "code0",
		},
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("run() a retourné une erreur inattendue: %v", err)
	}

	output := stdout.String()
	for _, expected := range []string{
		"Équipements possédés : code0",
		"Meilleure voile : Code0",
		"Vitesse attendue : 36.800000 nd",
		"1. foils | achat 440 | personnel +380",
		"gain +1.472000 nd",
	} {
		if !strings.Contains(output, expected) {
			t.Errorf("sortie absente %q dans:
%s", expected, output)
		}
	}
}

func TestRunRejectsMissingRequiredFlags(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		errorPart string
	}{
		{
			name:      "polaire absente",
			args:      []string{"-tws", "20", "-twa", "90", "-budget", "440"},
			errorPart: "-polar",
		},
		{
			name:      "TWS absente",
			args:      []string{"-polar", ultimPolarPath(), "-twa", "90", "-budget", "440"},
			errorPart: "-tws",
		},
		{
			name:      "TWA absente",
			args:      []string{"-polar", ultimPolarPath(), "-tws", "20", "-budget", "440"},
			errorPart: "-twa",
		},
		{
			name: "budget absent",
			args: []string{
				"-polar", ultimPolarPath(),
				"-tws", "20",
				"-twa", "90",
			},
			errorPart: "-budget",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := run(test.args, &bytes.Buffer{}, &bytes.Buffer{})
			if err == nil {
				t.Fatal("run() doit refuser cet appel")
			}
			if !strings.Contains(err.Error(), test.errorPart) {
				t.Errorf("erreur = %q, doit contenir %q", err, test.errorPart)
			}
		})
	}
}

func TestRunRejectsUnknownOwnedEquipment(t *testing.T) {
	err := run(
		[]string{
			"-polar", ultimPolarPath(),
			"-tws", "20",
			"-twa", "90",
			"-budget", "440",
			"-owned", "radio",
		},
		&bytes.Buffer{},
		&bytes.Buffer{},
	)
	if err == nil {
		t.Fatal("run() doit refuser un équipement inconnu")
	}
	if !strings.Contains(err.Error(), "radio") {
		t.Errorf("erreur inattendue: %v", err)
	}
}

func ultimPolarPath() string {
	return filepath.Join("..", "..", "..", "data", "polars", "ultim_bp_11.json")
}

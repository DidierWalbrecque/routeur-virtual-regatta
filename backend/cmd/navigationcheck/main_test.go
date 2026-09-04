package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunDisplaysProjectedPosition(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := run(
		[]string{
			"-latitude", "48.5",
			"-longitude", "-5.2",
			"-heading", "240",
			"-speed", "35",
			"-minutes", "10",
		},
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("run() a retourné une erreur inattendue: %v", err)
	}

	output := stdout.String()
	for _, expected := range []string{
		"Position initiale : 48.500000°, -5.200000°",
		"Cap constant : 240.00°",
		"Vitesse constante : 35.000000 nd",
		"Durée : 10.00 min",
		"Distance parcourue : 5.833333 NM",
		"Position théorique : 48.451352°, -5.326859°",
	} {
		if !strings.Contains(output, expected) {
			t.Errorf("sortie absente %q dans:\n%s", expected, output)
		}
	}
	if stderr.Len() != 0 {
		t.Errorf("sortie d'erreur inattendue: %s", stderr.String())
	}
}

func TestRunAcceptsVirtualRegattaPosition(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := run(
		[]string{
			"-position", "13°N 04'08\" - 121°E 37'42\"",
			"-heading", "346",
			"-speed", "5.8",
			"-minutes", "10",
		},
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatalf("run() a retourné une erreur inattendue: %v", err)
	}

	output := stdout.String()
	for _, expected := range []string{
		"Position initiale : 13.068889°, 121.628333°",
		"Cap constant : 346.00°",
		"Vitesse constante : 5.800000 nd",
		"Distance parcourue : 0.966667 NM",
		"Position théorique : 13.084511°, 121.624335°",
	} {
		if !strings.Contains(output, expected) {
			t.Errorf("sortie absente %q dans:\n%s", expected, output)
		}
	}
	if stderr.Len() != 0 {
		t.Errorf("sortie d'erreur inattendue: %s", stderr.String())
	}
}

func TestRunRejectsMixedPositionFormats(t *testing.T) {
	err := run(
		[]string{
			"-position", "13°N 04'08\" - 121°E 37'42\"",
			"-latitude", "13.068889",
			"-heading", "346",
			"-speed", "5.8",
			"-minutes", "10",
		},
		&bytes.Buffer{},
		&bytes.Buffer{},
	)
	if err == nil {
		t.Fatal("run() doit refuser deux formats de position simultanés")
	}
	if !strings.Contains(err.Error(), "ne peut pas être combiné") {
		t.Errorf("erreur inattendue: %v", err)
	}
}

func TestRunRejectsMissingRequiredFlags(t *testing.T) {
	complete := []string{
		"-latitude", "48.5",
		"-longitude", "-5.2",
		"-heading", "240",
		"-speed", "35",
		"-minutes", "10",
	}
	tests := []struct {
		name      string
		args      []string
		errorPart string
	}{
		{
			name:      "latitude absente",
			args:      complete[2:],
			errorPart: "-latitude",
		},
		{
			name: "longitude absente",
			args: []string{
				"-latitude", "48.5",
				"-heading", "240",
				"-speed", "35",
				"-minutes", "10",
			},
			errorPart: "-longitude",
		},
		{
			name: "cap absent",
			args: []string{
				"-latitude", "48.5",
				"-longitude", "-5.2",
				"-speed", "35",
				"-minutes", "10",
			},
			errorPart: "-heading",
		},
		{
			name: "vitesse absente",
			args: []string{
				"-latitude", "48.5",
				"-longitude", "-5.2",
				"-heading", "240",
				"-minutes", "10",
			},
			errorPart: "-speed",
		},
		{
			name: "durée absente",
			args: []string{
				"-latitude", "48.5",
				"-longitude", "-5.2",
				"-heading", "240",
				"-speed", "35",
			},
			errorPart: "-minutes",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := run(test.args, &bytes.Buffer{}, &bytes.Buffer{})
			if err == nil {
				t.Fatal("run() doit refuser cet appel")
			}
			if !strings.Contains(err.Error(), test.errorPart) {
				t.Errorf(
					"erreur = %q, doit contenir %q",
					err,
					test.errorPart,
				)
			}
		})
	}
}

func TestRunRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		errorPart string
	}{
		{
			name: "latitude invalide",
			args: []string{
				"-latitude", "91",
				"-longitude", "0",
				"-heading", "0",
				"-speed", "1",
				"-minutes", "1",
			},
			errorPart: "position initiale",
		},
		{
			name: "cap invalide",
			args: []string{
				"-latitude", "0",
				"-longitude", "0",
				"-heading", "360",
				"-speed", "1",
				"-minutes", "1",
			},
			errorPart: "cap invalide",
		},
		{
			name: "vitesse négative",
			args: []string{
				"-latitude", "0",
				"-longitude", "0",
				"-heading", "0",
				"-speed", "-1",
				"-minutes", "1",
			},
			errorPart: "vitesse",
		},
		{
			name: "durée négative",
			args: []string{
				"-latitude", "0",
				"-longitude", "0",
				"-heading", "0",
				"-speed", "1",
				"-minutes", "-1",
			},
			errorPart: "durée",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := run(test.args, &bytes.Buffer{}, &bytes.Buffer{})
			if err == nil {
				t.Fatal("run() doit refuser ces valeurs")
			}
			if !strings.Contains(err.Error(), test.errorPart) {
				t.Errorf(
					"erreur = %q, doit contenir %q",
					err,
					test.errorPart,
				)
			}
		})
	}
}

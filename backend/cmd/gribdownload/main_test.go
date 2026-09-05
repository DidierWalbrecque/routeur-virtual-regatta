package main

import (
	"strings"
	"testing"
)

func TestParseOptionsAcceptsDownloadSeries(t *testing.T) {
	options, err := parseOptions([]string{
		"-date", "20260905",
		"-cycle", "06",
		"-hours", "6,0,3",
		"-output", "../weather",
	})
	if err != nil {
		t.Fatalf("arguments refusés: %v", err)
	}

	if options.date != "20260905" {
		t.Fatalf("date = %q", options.date)
	}
	if options.cycle != 6 {
		t.Fatalf("cycle = %d", options.cycle)
	}
	wantHours := []int{0, 3, 6}
	if len(options.forecastHours) != len(wantHours) {
		t.Fatalf("échéances = %v", options.forecastHours)
	}
	for index := range wantHours {
		if options.forecastHours[index] != wantHours[index] {
			t.Fatalf("échéances = %v", options.forecastHours)
		}
	}
	if options.outputDirectory != "../weather" {
		t.Fatalf("dossier = %q", options.outputDirectory)
	}
}

func TestParseOptionsUsesDefaultHoursAndOutput(t *testing.T) {
	options, err := parseOptions([]string{
		"-date", "20260905",
		"-cycle", "18",
	})
	if err != nil {
		t.Fatalf("arguments refusés: %v", err)
	}

	wantHours := []int{0, 3, 6}
	for index := range wantHours {
		if options.forecastHours[index] != wantHours[index] {
			t.Fatalf("échéances = %v", options.forecastHours)
		}
	}
	if options.outputDirectory != "../data/weather" {
		t.Fatalf("dossier = %q", options.outputDirectory)
	}
}

func TestParseOptionsRejectsInvalidArguments(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "date absente",
			args:    []string{"-cycle", "06"},
			wantErr: "-date est requis",
		},
		{
			name:    "cycle absent",
			args:    []string{"-date", "20260905"},
			wantErr: "-cycle est requis",
		},
		{
			name: "cycle invalide",
			args: []string{
				"-date", "20260905",
				"-cycle", "05",
			},
			wantErr: "cycle GFS",
		},
		{
			name: "date invalide",
			args: []string{
				"-date", "2026-09-05",
				"-cycle", "06",
			},
			wantErr: "format YYYYMMDD",
		},
		{
			name: "échéance invalide",
			args: []string{
				"-date", "20260905",
				"-cycle", "06",
				"-hours", "0,abc",
			},
			wantErr: "échéance",
		},
		{
			name: "échéance dupliquée",
			args: []string{
				"-date", "20260905",
				"-cycle", "06",
				"-hours", "0,3,3",
			},
			wantErr: "plusieurs fois",
		},
		{
			name: "échéance hors limites",
			args: []string{
				"-date", "20260905",
				"-cycle", "06",
				"-hours", "385",
			},
			wantErr: "hors limites",
		},
		{
			name: "dossier vide",
			args: []string{
				"-date", "20260905",
				"-cycle", "06",
				"-output", "",
			},
			wantErr: "-output",
		},
		{
			name: "argument positionnel",
			args: []string{
				"-date", "20260905",
				"-cycle", "06",
				"unexpected",
			},
			wantErr: "arguments inattendus",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseOptions(test.args)
			if err == nil {
				t.Fatal("erreur attendue, aucune reçue")
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf(
					"erreur = %q, doit contenir %q",
					err,
					test.wantErr,
				)
			}
		})
	}
}

func TestParseForecastHoursSortsValues(t *testing.T) {
	hours, err := parseForecastHours("12, 0, 6")
	if err != nil {
		t.Fatalf("liste refusée: %v", err)
	}
	want := []int{0, 6, 12}
	for index := range want {
		if hours[index] != want[index] {
			t.Fatalf("échéances = %v", hours)
		}
	}
}

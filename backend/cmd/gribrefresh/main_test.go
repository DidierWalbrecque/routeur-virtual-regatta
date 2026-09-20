package main

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/weather"
)

type fakeDownloader struct {
	availableCycle int
	requests       []weather.GFSForecastRequest
}

func (fake *fakeDownloader) Download(_ context.Context, request weather.GFSForecastRequest, _ string) (string, error) {
	fake.requests = append(fake.requests, request)
	if request.Cycle != fake.availableCycle {
		return "", errors.New("run unavailable")
	}
	return request.LocalFileName(), nil
}

func TestParseOptionsDefaults(t *testing.T) {
	options, err := parseOptions(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(intStrings(options.forecastHours), ","); got != "0,3,6,9,12" {
		t.Fatalf("échéances = %q, attendu 0,3,6,9,12", got)
	}
	if options.lookbackHours != 48 || options.outputDirectory != "../data/weather" {
		t.Fatalf("options par défaut inattendues: %+v", options)
	}
}

func TestParseOptionsRequiresDateAndCycleTogether(t *testing.T) {
	if _, err := parseOptions([]string{"-date", "20260920"}); err == nil {
		t.Fatal("une date sans cycle devait être refusée")
	}
	if _, err := parseOptions([]string{"-cycle", "06"}); err == nil {
		t.Fatal("un cycle sans date devait être refusé")
	}
}

func TestCandidateRunsAreNewestFirst(t *testing.T) {
	now := time.Date(2026, 9, 20, 10, 15, 0, 0, time.UTC)
	candidates := candidateRuns(options{lookbackHours: 12}, now)
	if len(candidates) != 3 {
		t.Fatalf("nombre de runs = %d, attendu 3", len(candidates))
	}
	if candidates[0].Cycle != 6 || candidates[1].Cycle != 0 || candidates[2].Date.Day() != 19 || candidates[2].Cycle != 18 {
		t.Fatalf("ordre des runs inattendu: %+v", candidates)
	}
}

func TestRunFallsBackToPreviousAvailableRun(t *testing.T) {
	fake := &fakeDownloader{availableCycle: 0}
	stdout := new(strings.Builder)
	now := func() time.Time { return time.Date(2026, 9, 20, 10, 15, 0, 0, time.UTC) }

	err := run([]string{"-hours", "0"}, stdout, fake, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.requests) != 2 || fake.requests[0].Cycle != 6 || fake.requests[1].Cycle != 0 {
		t.Fatalf("tentatives inattendues: %+v", fake.requests)
	}
	if !strings.Contains(stdout.String(), "Run disponible : 2026-09-20 00Z") {
		t.Fatalf("sortie sans run final: %s", stdout.String())
	}
}

func intStrings(values []int) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = strconv.Itoa(value)
	}
	return result
}

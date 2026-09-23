package httpapi

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPracticalHeadingsStreamWritesSSEEvents(t *testing.T) {
	response := httptest.NewRecorder()
	stream := &practicalHeadingsStream{}

	if err := stream.start(response, practicalHeadingsStartEvent{TotalLayers: 4}); err != nil {
		t.Fatal(err)
	}
	if err := stream.event(response, "frontier", practicalHeadingsFrontierEvent{
		CompletedLayers: 1,
		TotalLayers:     4,
		Progress:        0.25,
	}); err != nil {
		t.Fatal(err)
	}
	if err := stream.event(response, "complete", map[string]any{
		"destinationReached": true,
	}); err != nil {
		t.Fatal(err)
	}

	if response.Code != 200 {
		t.Fatalf("statut = %d, attendu 200", response.Code)
	}
	if got := response.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q, attendu text/event-stream", got)
	}
	body := response.Body.String()
	for _, expected := range []string{
		"event: start\ndata:",
		"\"totalLayers\":4",
		"event: frontier\ndata:",
		"\"completedLayers\":1",
		"event: complete\ndata:",
		"\"destinationReached\":true",
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("flux SSE = %q, fragment absent %q", body, expected)
		}
	}
}

func TestPracticalHeadingsStreamWritesCancellationEvent(t *testing.T) {
	response := httptest.NewRecorder()
	stream := &practicalHeadingsStream{}
	if err := stream.start(response, practicalHeadingsStartEvent{TotalLayers: 1}); err != nil {
		t.Fatal(err)
	}

	stream.fail(response, "calcul interrompu", context.Canceled)

	if !strings.Contains(response.Body.String(), "event: cancelled\ndata:") {
		t.Fatalf("flux SSE = %q, événement cancelled absent", response.Body.String())
	}
}

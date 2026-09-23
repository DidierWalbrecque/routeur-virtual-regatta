package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

type practicalHeadingsStream struct {
	started bool
}

type practicalHeadingsStartEvent struct {
	Position    positionRequest `json:"position"`
	Destination positionRequest `json:"destination"`
	StartedAt   time.Time       `json:"startedAt"`
	TotalLayers int             `json:"totalLayers"`
}

type practicalHeadingsFrontierEvent struct {
	Instant            string            `json:"instant"`
	Positions          []positionRequest `json:"positions"`
	Contour            []positionRequest `json:"contour"`
	CompletedLayers    int               `json:"completedLayers"`
	TotalLayers        int               `json:"totalLayers"`
	Progress           float64           `json:"progress"`
	DestinationReached bool              `json:"destinationReached"`
}

func (stream *practicalHeadingsStream) start(
	response http.ResponseWriter,
	value practicalHeadingsStartEvent,
) error {
	response.Header().Set("Content-Type", "text/event-stream")
	response.Header().Set("Cache-Control", "no-cache")
	response.Header().Set("Connection", "keep-alive")
	response.Header().Set("X-Accel-Buffering", "no")
	response.WriteHeader(http.StatusOK)
	stream.started = true
	return stream.event(response, "start", value)
}

func (stream *practicalHeadingsStream) event(
	response http.ResponseWriter,
	event string,
	value any,
) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if _, err := response.Write([]byte("event: " + event + "\n")); err != nil {
		return err
	}
	if _, err := response.Write([]byte("data: ")); err != nil {
		return err
	}
	if _, err := response.Write(encoded); err != nil {
		return err
	}
	if _, err := response.Write([]byte("\n\n")); err != nil {
		return err
	}
	if flusher, ok := response.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}

func (stream *practicalHeadingsStream) fail(
	response http.ResponseWriter,
	message string,
	cause error,
) {
	if !stream.started {
		writeJSONError(response, http.StatusBadRequest, message)
		return
	}
	event := "error"
	if errors.Is(cause, context.Canceled) {
		event = "cancelled"
	}
	_ = stream.event(response, event, map[string]string{"error": message})
}

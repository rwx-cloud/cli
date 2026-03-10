package telemetry

import (
	"bytes"
	"encoding/json"
	"net/http"
)

const telemetryEndpoint = "/api/telemetry"

// Sender sends collected telemetry events to the server. Call Flush at CLI exit
// to deliver events as a non-blocking fire-and-forget POST.
type Sender struct {
	collector    *Collector
	roundTripper http.RoundTripper
}

// NewSender creates a Sender that will POST events using the given RoundTripper.
// The RoundTripper is expected to handle host, auth, and User-Agent headers.
func NewSender(collector *Collector, rt http.RoundTripper) *Sender {
	return &Sender{
		collector:    collector,
		roundTripper: rt,
	}
}

// Flush sends any queued events to the telemetry endpoint. It blocks until
// delivery completes (or fails), so call it just before CLI exit.
func (s *Sender) Flush() {
	if s == nil {
		return
	}

	events := s.collector.Drain()
	if len(events) == 0 {
		return
	}

	s.send(events)
}

func (s *Sender) send(events []Event) {
	data, err := json.Marshal(events)
	if err != nil {
		return
	}

	req, err := http.NewRequest(http.MethodPost, telemetryEndpoint, bytes.NewReader(data))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.roundTripper.RoundTrip(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

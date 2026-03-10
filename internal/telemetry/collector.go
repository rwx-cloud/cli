package telemetry

import (
	"runtime"
	"sync"
	"time"
)

// Event is a single telemetry event with its envelope fields.
type Event struct {
	Event     string         `json:"event"`
	Timestamp string         `json:"ts"`
	OS        string         `json:"os"`
	Arch      string         `json:"arch"`
	Props     map[string]any `json:"props,omitempty"`
}

// Collector accumulates telemetry events in a thread-safe queue.
type Collector struct {
	mu     sync.Mutex
	events []Event
}

// NewCollector creates a Collector.
func NewCollector() *Collector {
	return &Collector{}
}

// Record enqueues a telemetry event.
func (c *Collector) Record(event string, props map[string]any) {
	e := Event{
		Event:     event,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Props:     props,
	}

	c.mu.Lock()
	c.events = append(c.events, e)
	c.mu.Unlock()
}

// Drain returns all queued events and resets the queue.
func (c *Collector) Drain() []Event {
	c.mu.Lock()
	events := c.events
	c.events = nil
	c.mu.Unlock()
	return events
}

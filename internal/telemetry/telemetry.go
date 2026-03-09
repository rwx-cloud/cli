package telemetry

// Telemetry provides a unified interface for recording and flushing telemetry events.
type Telemetry struct {
	collector *Collector
	sender    *Sender
	stats     *StatsRoundTripper
}

// New creates a Telemetry instance that coordinates event recording, API stats
// aggregation, and flushing.
func New(collector *Collector, sender *Sender, stats *StatsRoundTripper) *Telemetry {
	return &Telemetry{
		collector: collector,
		sender:    sender,
		stats:     stats,
	}
}

// Record enqueues a telemetry event.
func (t *Telemetry) Record(event string, props map[string]any) {
	if t == nil {
		return
	}
	t.collector.Record(event, props)
}

// Flush aggregates API stats, then sends all queued events. Safe to call on nil.
func (t *Telemetry) Flush() {
	if t == nil {
		return
	}
	if t.stats != nil {
		t.stats.RecordSummary(t.collector)
	}
	t.sender.Flush()
}

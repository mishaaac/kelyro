// Package audit defines the boundary for recording security- and
// integrity-relevant actions.
package audit

// Event describes a critical action without coupling callers to a log format or
// event transport.
type Event struct {
	Action   string
	Subject  string
	Metadata map[string]string
}

// Recorder persists critical actions. A complete event bus is intentionally
// outside this contract.
type Recorder interface {
	Record(event Event) error
}

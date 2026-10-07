package monitor

import "time"

type SystemState int

const (
	StateInitializing SystemState = iota
	StateHealthy
	StateHalting
	StateHalted
)

type HeartbeatMessage struct {
	Sender    string `json:"sender"`
	Timestamp int64  `json:"timestamp"`
	Status    string `json:"status"`
}

type HeartbeatTracker struct {
	LastGatewayBeat time.Time
	LastEngineBeat  time.Time
}

func (h *HeartbeatTracker) BothReceived() bool {
	return !h.LastGatewayBeat.IsZero() && !h.LastEngineBeat.IsZero()
}

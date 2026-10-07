package monitor

import (
	"context"
	"testing"
	"time"
)

type haltRecorder struct {
	called bool
	ch     chan struct{}
}

func newHaltRecorder() *haltRecorder {
	return &haltRecorder{ch: make(chan struct{}, 1)}
}

func (h *haltRecorder) fn() {
	h.called = true
	select {
	case h.ch <- struct{}{}:
	default:
	}
}

type testCfg struct {
	allowedMissedSeconds int
	balancePollSeconds   int
	equityFloorFraction  float64
	startingCapital      float64
}

func coordinatorConfig() *testCfg {
	return &testCfg{
		allowedMissedSeconds: 2,
		balancePollSeconds:   999,
		equityFloorFraction:  0.85,
		startingCapital:      10000,
	}
}

func runCoordinatorForTest(
	ctx context.Context,
	cfg *testCfg,
	beatChan chan HeartbeatMessage,
	equityChan chan float64,
	haltFn func(),
) {
	state := StateInitializing
	tracker := HeartbeatTracker{}

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case beat := <-beatChan:
			if state == StateHalted {
				continue
			}
			now := time.Now()
			switch beat.Sender {
			case "gateway-go":
				tracker.LastGatewayBeat = now
			case "engine-net":
				tracker.LastEngineBeat = now
			}
			if state == StateInitializing && tracker.BothReceived() {
				state = StateHealthy
				tracker.LastGatewayBeat = now
				tracker.LastEngineBeat = now
			}
		case equity := <-equityChan:
			if state == StateHalted {
				continue
			}
			if state == StateHealthy {
				floorLimit := cfg.startingCapital * cfg.equityFloorFraction
				if equity < floorLimit {
					state = StateHalting
					if haltFn != nil {
						haltFn()
					}
					state = StateHalted
				}
			}
		case <-ticker.C:
			if state != StateHealthy {
				continue
			}
			now := time.Now()
			missedLimit := time.Duration(cfg.allowedMissedSeconds) * time.Second
			if now.Sub(tracker.LastGatewayBeat) > missedLimit {
				state = StateHalting
				if haltFn != nil {
					haltFn()
				}
				state = StateHalted
			} else if now.Sub(tracker.LastEngineBeat) > missedLimit {
				state = StateHalting
				if haltFn != nil {
					haltFn()
				}
				state = StateHalted
			}
		}
	}
}

func TestStateMachine_DynamicArming(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	beatChan := make(chan HeartbeatMessage, 10)
	equityChan := make(chan float64, 10)
	hr := newHaltRecorder()

	go runCoordinatorForTest(ctx, coordinatorConfig(), beatChan, equityChan, hr.fn)

	beatChan <- HeartbeatMessage{Sender: "gateway-go", Timestamp: time.Now().Unix()}
	beatChan <- HeartbeatMessage{Sender: "engine-net", Timestamp: time.Now().Unix()}

	time.Sleep(200 * time.Millisecond)

	if hr.called {
		t.Fatal("halt triggered during normal arming")
	}
}

func TestStateMachine_NotArmedUntilBothReceived(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	beatChan := make(chan HeartbeatMessage, 10)
	equityChan := make(chan float64, 10)
	hr := newHaltRecorder()

	go runCoordinatorForTest(ctx, coordinatorConfig(), beatChan, equityChan, hr.fn)

	beatChan <- HeartbeatMessage{Sender: "gateway-go", Timestamp: time.Now().Unix()}

	time.Sleep(300 * time.Millisecond)

	if hr.called {
		t.Fatal("halt should not trigger with only one service sending heartbeats")
	}
}

func TestHeartbeatLost_Gateway(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	beatChan := make(chan HeartbeatMessage, 10)
	equityChan := make(chan float64, 10)
	hr := newHaltRecorder()

	cfg := coordinatorConfig()
	cfg.allowedMissedSeconds = 1

	go runCoordinatorForTest(ctx, cfg, beatChan, equityChan, hr.fn)

	beatChan <- HeartbeatMessage{Sender: "gateway-go", Timestamp: time.Now().Unix()}
	beatChan <- HeartbeatMessage{Sender: "engine-net", Timestamp: time.Now().Unix()}
	time.Sleep(100 * time.Millisecond)

	go func() {
		tk := time.NewTicker(200 * time.Millisecond)
		defer tk.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tk.C:
				beatChan <- HeartbeatMessage{Sender: "engine-net", Timestamp: time.Now().Unix()}
			}
		}
	}()

	select {
	case <-hr.ch:
	case <-time.After(3 * time.Second):
		t.Fatal("expected halt for missing gateway heartbeat")
	}
}

func TestHeartbeatLost_Engine(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	beatChan := make(chan HeartbeatMessage, 10)
	equityChan := make(chan float64, 10)
	hr := newHaltRecorder()

	cfg := coordinatorConfig()
	cfg.allowedMissedSeconds = 1

	go runCoordinatorForTest(ctx, cfg, beatChan, equityChan, hr.fn)

	beatChan <- HeartbeatMessage{Sender: "gateway-go", Timestamp: time.Now().Unix()}
	beatChan <- HeartbeatMessage{Sender: "engine-net", Timestamp: time.Now().Unix()}
	time.Sleep(100 * time.Millisecond)

	go func() {
		tk := time.NewTicker(200 * time.Millisecond)
		defer tk.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tk.C:
				beatChan <- HeartbeatMessage{Sender: "gateway-go", Timestamp: time.Now().Unix()}
			}
		}
	}()

	select {
	case <-hr.ch:
	case <-time.After(3 * time.Second):
		t.Fatal("expected halt for missing engine heartbeat")
	}
}

func TestEquityBreach_ImmediateHalt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	beatChan := make(chan HeartbeatMessage, 10)
	equityChan := make(chan float64, 10)
	hr := newHaltRecorder()

	cfg := coordinatorConfig()
	cfg.allowedMissedSeconds = 999
	cfg.equityFloorFraction = 0.85
	cfg.startingCapital = 10000

	go runCoordinatorForTest(ctx, cfg, beatChan, equityChan, hr.fn)

	beatChan <- HeartbeatMessage{Sender: "gateway-go", Timestamp: time.Now().Unix()}
	beatChan <- HeartbeatMessage{Sender: "engine-net", Timestamp: time.Now().Unix()}
	time.Sleep(100 * time.Millisecond)

	equityChan <- 9000
	time.Sleep(100 * time.Millisecond)
	if hr.called {
		t.Fatal("halt should not trigger above floor (9000 >= 8500)")
	}

	equityChan <- 8000

	select {
	case <-hr.ch:
	case <-time.After(2 * time.Second):
		t.Fatal("expected halt for equity breach")
	}
}

func TestHaltedState_IgnoresNewBeats(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	beatChan := make(chan HeartbeatMessage, 10)
	equityChan := make(chan float64, 10)
	hr := newHaltRecorder()

	cfg := coordinatorConfig()
	cfg.allowedMissedSeconds = 1

	go runCoordinatorForTest(ctx, cfg, beatChan, equityChan, hr.fn)

	beatChan <- HeartbeatMessage{Sender: "gateway-go", Timestamp: time.Now().Unix()}
	beatChan <- HeartbeatMessage{Sender: "engine-net", Timestamp: time.Now().Unix()}
	time.Sleep(100 * time.Millisecond)

	beatChan <- HeartbeatMessage{Sender: "engine-net", Timestamp: time.Now().Unix()}
	beatChan <- HeartbeatMessage{Sender: "engine-net", Timestamp: time.Now().Unix()}

	select {
	case <-hr.ch:
	case <-time.After(3 * time.Second):
		t.Fatal("expected halt for engine heartbeat loss")
	}

	hr.called = false

	equityChan <- 5000
	time.Sleep(200 * time.Millisecond)
	if hr.called {
		t.Fatal("halt should not trigger again after already halted")
	}
}

func TestEquityAboveFloor_DoesNotHalt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	beatChan := make(chan HeartbeatMessage, 10)
	equityChan := make(chan float64, 10)
	hr := newHaltRecorder()

	cfg := coordinatorConfig()
	cfg.allowedMissedSeconds = 999
	cfg.equityFloorFraction = 0.50

	go runCoordinatorForTest(ctx, cfg, beatChan, equityChan, hr.fn)

	beatChan <- HeartbeatMessage{Sender: "gateway-go", Timestamp: time.Now().Unix()}
	beatChan <- HeartbeatMessage{Sender: "engine-net", Timestamp: time.Now().Unix()}
	time.Sleep(100 * time.Millisecond)

	for i := 0; i < 5; i++ {
		equityChan <- 5500
		time.Sleep(50 * time.Millisecond)
	}

	if hr.called {
		t.Fatal("halt triggered when equity was above floor")
	}
}

func TestHeartbeatTracker_BothReceived(t *testing.T) {
	var tracker HeartbeatTracker

	if tracker.BothReceived() {
		t.Fatal("expected false with zero values")
	}

	now := time.Now()
	tracker.LastGatewayBeat = now
	if tracker.BothReceived() {
		t.Fatal("expected false with only gateway set")
	}

	tracker.LastEngineBeat = now
	if !tracker.BothReceived() {
		t.Fatal("expected true with both set")
	}
}

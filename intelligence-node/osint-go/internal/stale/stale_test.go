package stale

import (
	"testing"
	"time"
)

func TestSourceStaleness_AlertsAfterThreeMissedCycles_NotOne(t *testing.T) {
	tracker := NewTracker(3)

	src := "feedA"

	tracker.RecordFailure(src)
	if tracker.IsStale(src) {
		t.Error("expected no staleness after 1 miss")
	}

	tracker.RecordFailure(src)
	if tracker.IsStale(src) {
		t.Error("expected no staleness after 2 misses")
	}

	tracker.RecordFailure(src)
	if !tracker.IsStale(src) {
		t.Error("expected staleness after 3 misses")
	}
}

func TestSourceStaleness_SuccessResetsCounter(t *testing.T) {
	tracker := NewTracker(3)

	src := "feedB"

	tracker.RecordFailure(src)
	tracker.RecordFailure(src)

	tracker.RecordSuccess(src)

	tracker.RecordFailure(src)
	if tracker.IsStale(src) {
		t.Error("expected success to reset counter; staleness should not trigger after 1 miss post-reset")
	}
}

func TestSourceStaleness_StateSnapshot(t *testing.T) {
	tracker := NewTracker(3)

	src := "feedC"

	state := tracker.State(src)
	if state.Failures != 0 {
		t.Errorf("expected 0 failures initially, got %d", state.Failures)
	}
	if state.IsStale {
		t.Error("expected not stale initially")
	}

	tracker.RecordFailure(src)
	tracker.RecordFailure(src)

	state = tracker.State(src)
	if state.Failures != 2 {
		t.Errorf("expected 2 failures, got %d", state.Failures)
	}
	if state.IsStale {
		t.Error("expected not stale after 2 failures")
	}

	tracker.RecordSuccess(src)
	state = tracker.State(src)
	if state.Failures != 0 {
		t.Errorf("expected 0 failures after success, got %d", state.Failures)
	}
	if state.IsStale {
		t.Error("expected not stale after success reset")
	}
	if state.LastSuccess.IsZero() {
		t.Error("expected LastSuccess to be set after RecordSuccess")
	}

	tracker.RecordFailure(src)
	tracker.RecordFailure(src)
	tracker.RecordFailure(src)
	state = tracker.State(src)
	if state.Failures != 3 {
		t.Errorf("expected 3 failures, got %d", state.Failures)
	}
	if !state.IsStale {
		t.Error("expected stale after 3 failures")
	}
}

func TestSourceStaleness_LastSuccessOnRecordSuccess(t *testing.T) {
	tracker := NewTracker(3)
	src := "feedD"

	before := time.Now()
	tracker.RecordSuccess(src)
	after := time.Now()

	state := tracker.State(src)
	if state.LastSuccess.Before(before) || state.LastSuccess.After(after) {
		t.Error("LastSuccess should be set to current time on RecordSuccess")
	}
}

func TestSourceStaleness_ZeroThreshold_SetsStaleOnFirstFail(t *testing.T) {
	tracker := NewTracker(0)

	src := "feedE"
	tracker.RecordFailure(src)
	if !tracker.IsStale(src) {
		t.Error("expected stale immediately with threshold 0")
	}
}

func TestSourceStaleness_IndependentPerSource(t *testing.T) {
	tracker := NewTracker(3)

	srcA := "feedA"
	srcB := "feedB"

	tracker.RecordFailure(srcA)
	tracker.RecordFailure(srcA)
	tracker.RecordFailure(srcA)

	if !tracker.IsStale(srcA) {
		t.Error("expected srcA to be stale after 3 failures")
	}
	if tracker.IsStale(srcB) {
		t.Error("expected srcB not to be stale with no failures")
	}

	tracker.RecordFailure(srcB)
	if tracker.IsStale(srcB) {
		t.Error("expected srcB not to be stale after only 1 failure")
	}
}

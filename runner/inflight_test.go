package runner

import "testing"

func TestInFlightLimiterEnforcesGlobalAndLaneCaps(t *testing.T) {
	limiter := NewInFlightLimiter(3, 2)

	if !limiter.TryAcquire("lane-a") {
		t.Fatalf("expected first lane-a acquire to succeed")
	}
	if !limiter.TryAcquire("lane-a") {
		t.Fatalf("expected second lane-a acquire to succeed")
	}
	if limiter.TryAcquire("lane-a") {
		t.Fatalf("expected third lane-a acquire to be capped by per-lane limit")
	}
	if !limiter.TryAcquire("lane-b") {
		t.Fatalf("expected first lane-b acquire to succeed while global cap remains")
	}
	if limiter.TryAcquire("lane-b") {
		t.Fatalf("expected additional acquire to be capped by global limit")
	}

	snapshot := limiter.Snapshot()
	if snapshot.Global != 3 {
		t.Fatalf("expected global in-flight 3, got %d", snapshot.Global)
	}
	if snapshot.PerLane["lane-a"] != 2 || snapshot.PerLane["lane-b"] != 1 {
		t.Fatalf("unexpected per-lane counts: %#v", snapshot.PerLane)
	}
	if snapshot.MaxGlobalObserved != 3 || snapshot.MaxLaneObserved["lane-a"] != 2 {
		t.Fatalf("unexpected max observed counts: %#v", snapshot)
	}
}

func TestInFlightLimiterReleaseMakesCapacityVerifiableAndReusable(t *testing.T) {
	limiter := NewInFlightLimiter(2, 1)

	if !limiter.TryAcquire("lane-a") || !limiter.TryAcquire("lane-b") {
		t.Fatalf("expected initial acquires to succeed")
	}
	if limiter.TryAcquire("lane-a") {
		t.Fatalf("expected acquires to stop while globally backpressured")
	}

	limiter.Release("lane-a")
	snapshot := limiter.Snapshot()
	if snapshot.Global != 1 || snapshot.PerLane["lane-b"] != 1 {
		t.Fatalf("unexpected counts after release: %#v", snapshot)
	}
	if _, ok := snapshot.PerLane["lane-a"]; ok {
		t.Fatalf("expected released lane to be removed from active counts: %#v", snapshot.PerLane)
	}

	if !limiter.TryAcquire("lane-a") {
		t.Fatalf("expected released capacity to allow progress to resume")
	}
	snapshot = limiter.Snapshot()
	if snapshot.Global != 2 || snapshot.PerLane["lane-a"] != 1 || snapshot.MaxGlobalObserved != 2 {
		t.Fatalf("unexpected counts after reacquire: %#v", snapshot)
	}
}

func TestInFlightLimiterIgnoresOverRelease(t *testing.T) {
	limiter := NewInFlightLimiter(1, 1)

	limiter.Release("lane-a")
	if !limiter.TryAcquire("lane-a") {
		t.Fatalf("expected acquire after over-release noop to succeed")
	}
	limiter.Release("lane-a")
	limiter.Release("lane-a")

	snapshot := limiter.Snapshot()
	if snapshot.Global != 0 {
		t.Fatalf("expected global count to stay at 0 after extra releases, got %d", snapshot.Global)
	}
	if len(snapshot.PerLane) != 0 {
		t.Fatalf("expected no active per-lane counts after release, got %#v", snapshot.PerLane)
	}
}

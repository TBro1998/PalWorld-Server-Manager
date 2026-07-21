package sysstat

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestHost verifies the host snapshot reports plausible fixed facts (core count
// and total memory) that never depend on a CPU baseline.
func TestHost(t *testing.T) {
	c := New()
	h := c.Host(context.Background())
	if h.NumCPU <= 0 {
		t.Errorf("NumCPU = %d, want > 0", h.NumCPU)
	}
	if h.MemTotal == 0 {
		t.Error("MemTotal = 0, want > 0")
	}
}

// TestProcessSelf runs against the test binary's own PID. The first frame must
// report CPU 0 (no baseline) without panicking; a second frame after brief work
// must stay non-negative and mark the process running with a non-zero RSS.
func TestProcessSelf(t *testing.T) {
	c := New()
	ctx := context.Background()
	key := "self"
	pid := os.Getpid()

	first := c.Process(ctx, key, pid)
	if !first.Running {
		t.Fatal("first frame: Running = false, want true for self pid")
	}
	if first.CPUPercent != 0 {
		t.Errorf("first frame: CPUPercent = %v, want 0 (no baseline)", first.CPUPercent)
	}
	if first.MemoryRSS == 0 {
		t.Error("first frame: MemoryRSS = 0, want > 0")
	}
	if first.ProcessCount == 0 {
		t.Error("first frame: ProcessCount = 0, want >= 1")
	}

	// Burn a little CPU so the second frame has a real delta to measure.
	deadline := time.Now().Add(50 * time.Millisecond)
	x := 0
	for time.Now().Before(deadline) {
		x++
	}
	_ = x

	second := c.Process(ctx, key, pid)
	if !second.Running {
		t.Fatal("second frame: Running = false, want true")
	}
	if second.CPUPercent < 0 {
		t.Errorf("second frame: CPUPercent = %v, want >= 0", second.CPUPercent)
	}
}

// TestProcessMissing verifies an impossible PID degrades to not-running without
// an error and without panicking.
func TestProcessMissing(t *testing.T) {
	c := New()
	got := c.Process(context.Background(), "missing", 1<<30)
	if got.Running {
		t.Error("Running = true, want false for impossible pid")
	}
	if got.Reason != "not_running" {
		t.Errorf("Reason = %q, want \"not_running\"", got.Reason)
	}
}

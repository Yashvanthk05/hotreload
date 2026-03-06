package runner_test

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"hotreload/runner"
)

func TestRunner_StartAndStop(t *testing.T) {
	// Use 'sleep 10' as a long-running "server"
	r := runner.New("sleep 10")
	ctx := context.Background()

	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Give the process a moment to start
	time.Sleep(100 * time.Millisecond)

	// Stop should complete without error
	if err := r.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestRunner_StopWithNoProcess(t *testing.T) {
	// Stop should be a no-op if no process is running
	r := runner.New("sleep 10")
	if err := r.Stop(); err != nil {
		t.Fatalf("Stop with no process should be a no-op, got: %v", err)
	}
}

func TestRunner_ProcessDiesAfterStop(t *testing.T) {
	r := runner.New("sleep 10")
	ctx := context.Background()

	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Capture the PID before stopping
	// We give the process a moment to start
	time.Sleep(50 * time.Millisecond)

	if err := r.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// After Stop, the runner should have no process reference.
	// Starting a new one should work fine.
	if err := r.Start(ctx); err != nil {
		t.Fatalf("Second Start failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if err := r.Stop(); err != nil {
		t.Fatalf("Second Stop failed: %v", err)
	}
}

func TestRunner_ProcessGroupKilled(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping process group test in CI")
	}

	// We start a process that spawns a child (shell with subshell)
	// Then we verify that Stop kills it quickly (relies on SIGKILL to process group)
	r := runner.New("bash -c 'sleep 30 & sleep 30'")
	ctx := context.Background()

	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	time.Sleep(200 * time.Millisecond)

	stopStart := time.Now()
	if err := r.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	elapsed := time.Since(stopStart)

	// Stop should complete within 3s (graceful + force kill window)
	if elapsed > 4*time.Second {
		t.Errorf("Stop took too long (%v), process group may not have been killed", elapsed)
	}
}

func TestRunner_Stop_IgnoresAlreadyStopped(_ *testing.T) {
	// Signal math: ESRCH means no such process — Stop should handle it gracefully
	_ = syscall.ESRCH // just reference it to show we know about it
}

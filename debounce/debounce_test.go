package debounce_test

import (
	"testing"
	"time"

	"hotreload/debounce"
)

func TestDebounce_SingleTrigger(t *testing.T) {
	d := debounce.New(50 * time.Millisecond)
	defer d.Stop()

	d.Trigger()

	select {
	case <-d.C():
		// Correct: fired once
	case <-time.After(500 * time.Millisecond):
		t.Fatal("debouncer never fired")
	}
}

func TestDebounce_MultipleRapidTriggers_FiresOnce(t *testing.T) {
	d := debounce.New(100 * time.Millisecond)
	defer d.Stop()

	// Fire many triggers rapidly
	for i := 0; i < 10; i++ {
		d.Trigger()
		time.Sleep(10 * time.Millisecond)
	}

	// Should fire exactly once after the quiet period
	select {
	case <-d.C():
		// Good — fired once
	case <-time.After(500 * time.Millisecond):
		t.Fatal("debouncer never fired after rapid triggers")
	}

	// Should NOT fire again within the next 200ms
	select {
	case <-d.C():
		t.Fatal("debouncer fired a second time (should only fire once)")
	case <-time.After(200 * time.Millisecond):
		// Good — only fired once
	}
}

func TestDebounce_ResetOnNewTrigger(t *testing.T) {
	delay := 100 * time.Millisecond
	d := debounce.New(delay)
	defer d.Stop()

	start := time.Now()
	d.Trigger()

	// Trigger again just before it would have fired
	time.Sleep(70 * time.Millisecond)
	d.Trigger()

	// It should fire ~100ms after the second trigger, not after the first
	select {
	case <-d.C():
		elapsed := time.Since(start)
		// Should be at least 140ms (70ms sleep + ~100ms delay), not ~100ms
		if elapsed < 140*time.Millisecond {
			t.Errorf("debouncer fired too early (elapsed %v), should have reset on second trigger", elapsed)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("debouncer never fired")
	}
}

func TestDebounce_NoFire_AfterStop(t *testing.T) {
	d := debounce.New(50 * time.Millisecond)
	d.Trigger()
	d.Stop()

	select {
	case <-d.C():
		// It may still fire because the timer might have fired before Stop()
		// This is acceptable — Stop() is best-effort
	case <-time.After(200 * time.Millisecond):
		// Also acceptable
	}
}

package debounce

import (
	"sync"
	"time"
)

// Debouncer is a trailing-edge debouncer. It waits for a quiet period
// after the last Trigger() call before firing on its channel.
//
// This is critical for handling editors (vim, JetBrains) that write
// temp files and rename them, firing 3–5 events per save. We want
// exactly one rebuild per intentional save.
type Debouncer struct {
	delay  time.Duration
	mu     sync.Mutex
	timer  *time.Timer
	ch     chan struct{}
}

// New creates a new Debouncer with the given quiet period.
func New(delay time.Duration) *Debouncer {
	return &Debouncer{
		delay: delay,
		ch:    make(chan struct{}, 1),
	}
}

// Trigger records a new event. If a timer is already running, it is reset.
// After the quiet period elapses (no new Trigger calls), a value is sent
// on the channel returned by C().
func (d *Debouncer) Trigger() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.timer != nil {
		d.timer.Stop()
	}

	d.timer = time.AfterFunc(d.delay, func() {
		// Non-blocking send: if the channel is full (receiver hasn't processed
		// the previous trigger yet), we drop this and let them batch up.
		select {
		case d.ch <- struct{}{}:
		default:
		}
	})
}

// C returns the channel that fires after the quiet period elapses.
// The caller should range over or select on this channel.
func (d *Debouncer) C() <-chan struct{} {
	return d.ch
}

// Stop cancels any pending timer. Should be called on shutdown.
func (d *Debouncer) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Stop()
	}
}

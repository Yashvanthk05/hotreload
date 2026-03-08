package debounce

import (
	"sync"
	"time"
)

type Debouncer struct {
	delay  time.Duration
	mu     sync.Mutex
	timer  *time.Timer
	ch     chan struct{}
}

func New(delay time.Duration) *Debouncer {
	return &Debouncer{
		delay: delay,
		ch:    make(chan struct{}, 1),
	}
}

func (d *Debouncer) Trigger() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.timer != nil {
		d.timer.Stop()
	}

	d.timer = time.AfterFunc(d.delay, func() {
		select {
		case d.ch <- struct{}{}:
		default:
		}
	})
}

func (d *Debouncer) C() <-chan struct{} {
	return d.ch
}

func (d *Debouncer) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.timer != nil {
		d.timer.Stop()
	}
}

// tools/leakdetector/detector.go
package leakdetector

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

// Detector tracks goroutines and helps identify leaks
type Detector struct {
	mu       sync.Mutex
	routines map[uint64]routine
	done     chan struct{}
}

type routine struct {
	stack    string
	created  time.Time
}

// New creates a new goroutine leak detector
func New(checkInterval time.Duration) *Detector {
	d := &Detector{
		routines: make(map[uint64]routine),
		done:     make(chan struct{}),
	}

	// Start background monitoring
	go d.monitor(checkInterval)
	return d
}

// Track starts tracking a new goroutine
func (d *Detector) Track() uint64 {
	d.mu.Lock()
	defer d.mu.Unlock()

	id := uint64(time.Now().UnixNano())
	stack := make([]byte, 4096)
	n := runtime.Stack(stack, false)

	d.routines[id] = routine{
		stack:   string(stack[:n]),
		created: time.Now(),
	}

	return id
}

// Done marks a goroutine as completed
func (d *Detector) Done(id uint64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.routines, id)
}

// monitor periodically checks for potential leaks
func (d *Detector) monitor(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.checkLeaks()
		case <-d.done:
			return
		}
	}
}

// checkLeaks looks for long-running goroutines
func (d *Detector) checkLeaks() {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	for id, r := range d.routines {
		if now.Sub(r.created) > 5*time.Minute {
			fmt.Printf("Potential goroutine leak detected:\nID: %d\nAge: %v\nStack:\n%s\n\n",
				id, now.Sub(r.created), r.stack)
		}
	}
}

// Close stops the leak detector
func (d *Detector) Close() error {
	close(d.done)
	return nil
}

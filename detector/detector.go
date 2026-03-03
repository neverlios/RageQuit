package detector

import (
	"math"
	"sync"
	"time"
)

// Detector triggers when acceleration deviates from 1g by more than minAmplitude
// and enforces a cooldown between triggers.
type Detector struct {
	minAmplitude float64
	cooldown     time.Duration
	mu           sync.Mutex
	lastTrigger  time.Time
}

// New creates a Detector with the given sensitivity and cooldown.
func New(minAmplitude float64, cooldown time.Duration) *Detector {
	return &Detector{
		minAmplitude: minAmplitude,
		cooldown:     cooldown,
	}
}

// Check returns true if the acceleration sample represents an impact
// that should trigger the display. Thread-safe.
func (d *Detector) Check(x, y, z float64) bool {
	magnitude := math.Sqrt(x*x + y*y + z*z)
	impact := math.Abs(magnitude - 1.0) // subtract gravity
	if impact < d.minAmplitude {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now()
	if now.Sub(d.lastTrigger) < d.cooldown {
		return false
	}
	d.lastTrigger = now
	return true
}

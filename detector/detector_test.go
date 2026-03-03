package detector

import (
	"testing"
	"time"
)

func TestDetector_triggersAboveThreshold(t *testing.T) {
	d := New(0.3, 750*time.Millisecond)
	// Z=1.5g → deviation from 1g = 0.5g → above 0.3g threshold
	if !d.Check(0, 0, 1.5) {
		t.Error("expected trigger on large impact, got false")
	}
}

func TestDetector_doesNotTriggerBelowThreshold(t *testing.T) {
	d := New(0.3, 750*time.Millisecond)
	// Z=1.1g → deviation = 0.1g → below 0.3g threshold
	if d.Check(0, 0, 1.1) {
		t.Error("expected no trigger on small impact, got true")
	}
}

func TestDetector_cooldownPreventsImmedateRetrigger(t *testing.T) {
	d := New(0.3, 750*time.Millisecond)
	d.Check(0, 0, 1.5) // first trigger
	if d.Check(0, 0, 1.5) {
		t.Error("expected cooldown to block second trigger, got true")
	}
}

func TestDetector_triggersAgainAfterCooldownExpires(t *testing.T) {
	d := New(0.3, 5*time.Millisecond)
	d.Check(0, 0, 1.5) // first trigger
	time.Sleep(10 * time.Millisecond)
	if !d.Check(0, 0, 1.5) {
		t.Error("expected trigger after cooldown expired, got false")
	}
}

func TestDetector_detectionUsesVectorMagnitude(t *testing.T) {
	d := New(0.3, 1*time.Millisecond)
	// Diagonal impact: sqrt(0.8^2 + 0.8^2 + 0.8^2) ≈ 1.386g → deviation ≈ 0.386g
	if !d.Check(0.8, 0.8, 0.8) {
		t.Error("expected trigger on diagonal impact, got false")
	}
}

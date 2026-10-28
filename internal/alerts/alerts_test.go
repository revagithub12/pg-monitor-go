package alerts

import (
	"testing"
	"time"

	"github.com/YOUR_GITHUB_USERNAME/pg-monitor-go/internal/metrics"
)

func TestAddValidation(t *testing.T) {
	s := NewStore()
	if _, err := s.Add(Rule{Metric: "nope", Operator: ">", Threshold: 1}); err == nil {
		t.Fatal("expected error for unknown metric")
	}
	if _, err := s.Add(Rule{Metric: "connections", Operator: "=", Threshold: 1}); err == nil {
		t.Fatal("expected error for bad operator")
	}
	r, err := s.Add(Rule{Metric: "connections", Operator: ">", Threshold: 80})
	if err != nil || r.ID != 1 {
		t.Fatalf("unexpected result: %+v, %v", r, err)
	}
}

func TestEvaluateAndCooldown(t *testing.T) {
	s := NewStore()
	s.Add(Rule{Metric: "connections", Operator: ">", Threshold: 80})
	s.Add(Rule{Metric: "cache_hit_ratio", Operator: "<", Threshold: 0.9})

	now := time.Now()
	snap := metrics.Snapshot{Time: now, Connections: 95, CacheHitRatio: 0.99}

	events := s.Evaluate(snap, time.Minute)
	if len(events) != 1 || events[0].Metric != "connections" {
		t.Fatalf("expected one connections alert, got %+v", events)
	}

	// same condition inside the cooldown window must not fire again
	snap.Time = now.Add(10 * time.Second)
	if events = s.Evaluate(snap, time.Minute); len(events) != 0 {
		t.Fatalf("expected cooldown to suppress alert, got %+v", events)
	}

	// after the cooldown it fires again
	snap.Time = now.Add(2 * time.Minute)
	if events = s.Evaluate(snap, time.Minute); len(events) != 1 {
		t.Fatalf("expected alert after cooldown, got %+v", events)
	}
}

func TestDelete(t *testing.T) {
	s := NewStore()
	r, _ := s.Add(Rule{Metric: "connections", Operator: ">", Threshold: 1})
	if !s.Delete(r.ID) || s.Delete(r.ID) {
		t.Fatal("delete should succeed once, then report not found")
	}
}

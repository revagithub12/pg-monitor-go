// Package monitor runs the background collection loop.
package monitor

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YOUR_GITHUB_USERNAME/pg-monitor-go/internal/alerts"
	"github.com/YOUR_GITHUB_USERNAME/pg-monitor-go/internal/hub"
	"github.com/YOUR_GITHUB_USERNAME/pg-monitor-go/internal/metrics"
	"github.com/YOUR_GITHUB_USERNAME/pg-monitor-go/internal/pubsub"
)

const (
	historySize   = 120
	alertChannel  = "pgmonitor.alerts"
	alertCooldown = time.Minute
)

type Monitor struct {
	pool     *pgxpool.Pool
	interval time.Duration
	rules    *alerts.Store
	hub      *hub.Hub
	pub      pubsub.Publisher

	mu      sync.RWMutex
	history []metrics.Snapshot
}

func New(pool *pgxpool.Pool, interval time.Duration, rules *alerts.Store, h *hub.Hub, pub pubsub.Publisher) *Monitor {
	return &Monitor{pool: pool, interval: interval, rules: rules, hub: h, pub: pub}
}

// Run collects metrics on every tick until ctx is cancelled (graceful shutdown).
func (m *Monitor) Run(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	m.tick(ctx) // collect once immediately
	for {
		select {
		case <-ctx.Done():
			log.Println("monitor stopped")
			return
		case <-ticker.C:
			m.tick(ctx)
		}
	}
}

func (m *Monitor) tick(ctx context.Context) {
	cctx, cancel := context.WithTimeout(ctx, m.interval)
	defer cancel()

	snap, err := metrics.Collect(cctx, m.pool)
	if err != nil {
		log.Printf("collect failed: %v", err)
		return
	}

	m.mu.Lock()
	m.history = append(m.history, snap)
	if len(m.history) > historySize {
		m.history = m.history[len(m.history)-historySize:]
	}
	m.mu.Unlock()

	m.broadcast("metrics", snap)

	for _, event := range m.rules.Evaluate(snap, alertCooldown) {
		log.Printf("ALERT: %s", event.Message)
		m.broadcast("alert", event)
		payload, _ := json.Marshal(event)
		if err := m.pub.Publish(ctx, alertChannel, payload); err != nil {
			log.Printf("publish alert failed: %v", err)
		}
	}
}

func (m *Monitor) broadcast(kind string, data any) {
	msg, err := json.Marshal(map[string]any{"type": kind, "data": data})
	if err != nil {
		return
	}
	m.hub.Broadcast(msg)
}

// Latest returns the most recent snapshot, if any has been collected.
func (m *Monitor) Latest() (metrics.Snapshot, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.history) == 0 {
		return metrics.Snapshot{}, false
	}
	return m.history[len(m.history)-1], true
}

// History returns a copy of the recent snapshots, oldest first.
func (m *Monitor) History() []metrics.Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]metrics.Snapshot, len(m.history))
	copy(out, m.history)
	return out
}

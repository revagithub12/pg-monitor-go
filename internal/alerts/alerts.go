// Package alerts stores alert rules and evaluates them against metric snapshots.
package alerts

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/YOUR_GITHUB_USERNAME/pg-monitor-go/internal/metrics"
)

type Rule struct {
	ID        int     `json:"id"`
	Metric    string  `json:"metric"`
	Operator  string  `json:"operator"` // ">" or "<"
	Threshold float64 `json:"threshold"`
}

type Event struct {
	RuleID    int       `json:"rule_id"`
	Metric    string    `json:"metric"`
	Value     float64   `json:"value"`
	Operator  string    `json:"operator"`
	Threshold float64   `json:"threshold"`
	Message   string    `json:"message"`
	Time      time.Time `json:"time"`
}

// Store keeps rules in memory. It is safe for concurrent use.
type Store struct {
	mu        sync.RWMutex
	rules     map[int]Rule
	lastFired map[int]time.Time
	nextID    int
}

func NewStore() *Store {
	return &Store{rules: map[int]Rule{}, lastFired: map[int]time.Time{}, nextID: 1}
}

func (s *Store) Add(r Rule) (Rule, error) {
	if !metrics.IsValidName(r.Metric) {
		return Rule{}, fmt.Errorf("unknown metric %q (valid: %v)", r.Metric, metrics.Names)
	}
	if r.Operator != ">" && r.Operator != "<" {
		return Rule{}, fmt.Errorf("operator must be > or <")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r.ID = s.nextID
	s.nextID++
	s.rules[r.ID] = r
	return r, nil
}

func (s *Store) Delete(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.rules[id]
	delete(s.rules, id)
	delete(s.lastFired, id)
	return ok
}

func (s *Store) List() []Rule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Rule, 0, len(s.rules))
	for _, r := range s.rules {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Evaluate checks every rule against the snapshot. A rule that already fired
// within the cooldown period is skipped, so users are not flooded with alerts.
func (s *Store) Evaluate(snap metrics.Snapshot, cooldown time.Duration) []Event {
	s.mu.Lock()
	defer s.mu.Unlock()

	var events []Event
	for _, r := range s.rules {
		value, ok := snap.Value(r.Metric)
		if !ok {
			continue
		}
		triggered := (r.Operator == ">" && value > r.Threshold) || (r.Operator == "<" && value < r.Threshold)
		if !triggered {
			continue
		}
		if last, seen := s.lastFired[r.ID]; seen && snap.Time.Sub(last) < cooldown {
			continue
		}
		s.lastFired[r.ID] = snap.Time
		events = append(events, Event{
			RuleID:    r.ID,
			Metric:    r.Metric,
			Value:     value,
			Operator:  r.Operator,
			Threshold: r.Threshold,
			Message:   fmt.Sprintf("%s is %.2f (rule: %s %.2f)", r.Metric, value, r.Operator, r.Threshold),
			Time:      snap.Time,
		})
	}
	sort.Slice(events, func(i, j int) bool { return events[i].RuleID < events[j].RuleID })
	return events
}

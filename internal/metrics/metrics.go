// Package metrics collects health metrics from a PostgreSQL database.
package metrics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Names lists the metrics that alert rules can target.
var Names = []string{
	"connections",
	"active_queries",
	"idle_connections",
	"longest_query_seconds",
	"database_size_bytes",
	"cache_hit_ratio",
}

type Snapshot struct {
	Time                time.Time `json:"time"`
	Connections         int64     `json:"connections"`
	ActiveQueries       int64     `json:"active_queries"`
	IdleConnections     int64     `json:"idle_connections"`
	LongestQuerySeconds float64   `json:"longest_query_seconds"`
	DatabaseSizeBytes   int64     `json:"database_size_bytes"`
	CacheHitRatio       float64   `json:"cache_hit_ratio"`
}

// Value returns the value of a metric by name.
func (s Snapshot) Value(name string) (float64, bool) {
	switch name {
	case "connections":
		return float64(s.Connections), true
	case "active_queries":
		return float64(s.ActiveQueries), true
	case "idle_connections":
		return float64(s.IdleConnections), true
	case "longest_query_seconds":
		return s.LongestQuerySeconds, true
	case "database_size_bytes":
		return float64(s.DatabaseSizeBytes), true
	case "cache_hit_ratio":
		return s.CacheHitRatio, true
	}
	return 0, false
}

// IsValidName reports whether name is a known metric.
func IsValidName(name string) bool {
	_, ok := Snapshot{}.Value(name)
	return ok
}

const activityQuery = `
SELECT
  count(*)::bigint,
  (count(*) FILTER (WHERE state = 'active' AND pid <> pg_backend_pid()))::bigint,
  (count(*) FILTER (WHERE state = 'idle'))::bigint,
  COALESCE(max(EXTRACT(EPOCH FROM (now() - query_start)))
           FILTER (WHERE state = 'active' AND pid <> pg_backend_pid()), 0)::float8
FROM pg_stat_activity
WHERE datname = current_database()`

const sizeQuery = `SELECT pg_database_size(current_database())::bigint`

const cacheQuery = `
SELECT COALESCE(blks_hit::float8 / NULLIF(blks_hit + blks_read, 0), 0)::float8
FROM pg_stat_database
WHERE datname = current_database()`

// Collect reads one snapshot of metrics from the database.
func Collect(ctx context.Context, pool *pgxpool.Pool) (Snapshot, error) {
	snap := Snapshot{Time: time.Now().UTC()}

	err := pool.QueryRow(ctx, activityQuery).Scan(
		&snap.Connections, &snap.ActiveQueries, &snap.IdleConnections, &snap.LongestQuerySeconds,
	)
	if err != nil {
		return snap, err
	}
	if err = pool.QueryRow(ctx, sizeQuery).Scan(&snap.DatabaseSizeBytes); err != nil {
		return snap, err
	}
	if err = pool.QueryRow(ctx, cacheQuery).Scan(&snap.CacheHitRatio); err != nil {
		return snap, err
	}
	return snap, nil
}

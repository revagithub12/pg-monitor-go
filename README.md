# pg-monitor-go

A small **PostgreSQL monitoring service written in Go**. It collects database health metrics on a schedule,
evaluates alert rules, and pushes live updates to clients over **WebSocket**. Alerts are also published to **Redis**.
REST APIs are protected with **JWT** and **role-based access control**.

## Problem it solves
Teams need to know when a database is unhealthy (too many connections, a query running too long, low cache hit
ratio) before users complain. This service watches those signals continuously and notifies you in real time.

## Architecture

```
            +-------------------+        +-----------------+
 PostgreSQL | collector goroutine|------>| alert evaluator |
 (target)-->| (every N seconds)  |        +--------+--------+
            +---------+---------+                  |
                      |                            |
              WebSocket hub  <----- alerts ---------+-----> Redis channel
                      |                                    "pgmonitor.alerts"
              browser / clients
                      ^
          REST API (JWT + RBAC): metrics, history, alert rules
```

## Features
- Metrics: total / active / idle connections, longest running query, database size, cache hit ratio
- Background collector in a goroutine with graceful shutdown (`SIGINT` / `SIGTERM`)
- Alert rules (`metric`, `>` or `<`, `threshold`) with a cooldown so alerts do not flood
- Live metrics and alerts over WebSocket (`/ws`)
- Alerts published to Redis through a `Publisher` interface (easy to add Kafka later)
- JWT login with two roles: **admin** (manage rules) and **viewer** (read only)
- Unit tests for alert evaluation and auth
- One-command run with Docker Compose

## Run with Docker Compose
```bash
# 1. replace the module path with your GitHub username
grep -rl YOUR_GITHUB_USERNAME . | xargs sed -i 's/YOUR_GITHUB_USERNAME/<your-username>/g'

# 2. start everything
docker compose up --build
```
Open http://localhost:8080, log in (`admin` / `admin12345`) and watch live metrics.

## Run locally
```bash
go mod tidy
export TARGET_DB_URL="postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
go run ./cmd/server
```
Redis is optional: leave `REDIS_ADDR` empty to skip publishing.

## Run the tests
```bash
go test ./...
```

## API
| Method | Endpoint | Role | Description |
|---|---|---|---|
| POST | `/auth/login` | public | Body `{"username","password"}`, returns `{"token"}` |
| GET | `/metrics/latest` | admin, viewer | Latest snapshot |
| GET | `/metrics/history` | admin, viewer | Last 120 snapshots |
| GET | `/alerts/rules` | admin, viewer | List rules |
| POST | `/alerts/rules` | admin | Create a rule |
| DELETE | `/alerts/rules/{id}` | admin | Delete a rule |
| GET | `/ws?token=...` | admin, viewer | Live metrics and alerts |

```bash
TOKEN=$(curl -s -X POST localhost:8080/auth/login -d '{"username":"admin","password":"admin12345"}' | jq -r .token)

curl -H "Authorization: Bearer $TOKEN" localhost:8080/metrics/latest

curl -X POST localhost:8080/alerts/rules -H "Authorization: Bearer $TOKEN" \
  -d '{"metric":"connections","operator":">","threshold":80}'
```

Valid metrics: `connections`, `active_queries`, `idle_connections`, `longest_query_seconds`,
`database_size_bytes`, `cache_hit_ratio`.

Subscribe to alerts from Redis:
```bash
docker compose exec redis redis-cli SUBSCRIBE pgmonitor.alerts
```

## Project structure
```
cmd/server/main.go     # wiring, graceful shutdown
internal/config        # environment configuration
internal/metrics       # PostgreSQL queries and snapshot type
internal/alerts        # rule store and evaluation (+ tests)
internal/monitor       # collection loop, history
internal/hub           # WebSocket broadcast
internal/pubsub        # Publisher interface, Redis implementation
internal/auth          # JWT and role middleware (+ tests)
internal/api           # HTTP handlers and demo page
```

## Notes and what I would improve next
- Demo users come from environment variables; a real system would use a users table with hashed passwords.
- Alert rules are in memory; persist them in PostgreSQL.
- Add a Kafka implementation of `Publisher`.
- Add `pg_stat_statements` for slow-query analysis and Prometheus metrics.
- Restrict WebSocket origins in production.

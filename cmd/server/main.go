package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YOUR_GITHUB_USERNAME/pg-monitor-go/internal/alerts"
	"github.com/YOUR_GITHUB_USERNAME/pg-monitor-go/internal/api"
	"github.com/YOUR_GITHUB_USERNAME/pg-monitor-go/internal/auth"
	"github.com/YOUR_GITHUB_USERNAME/pg-monitor-go/internal/config"
	"github.com/YOUR_GITHUB_USERNAME/pg-monitor-go/internal/hub"
	"github.com/YOUR_GITHUB_USERNAME/pg-monitor-go/internal/monitor"
	"github.com/YOUR_GITHUB_USERNAME/pg-monitor-go/internal/pubsub"
)

func main() {
	cfg := config.Load()

	// ctx is cancelled on Ctrl+C or SIGTERM, which triggers a graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := connectWithRetry(ctx, cfg.TargetDBURL)
	if err != nil {
		log.Fatalf("cannot connect to PostgreSQL: %v", err)
	}
	defer pool.Close()

	var publisher pubsub.Publisher = pubsub.Noop{}
	if cfg.RedisAddr != "" {
		publisher = pubsub.NewRedis(cfg.RedisAddr)
		log.Printf("publishing alerts to Redis at %s", cfg.RedisAddr)
	}

	rules := alerts.NewStore()
	rules.Add(alerts.Rule{Metric: "longest_query_seconds", Operator: ">", Threshold: 30}) // default rule

	h := hub.New()
	mon := monitor.New(pool, cfg.CollectInterval, rules, h, publisher)
	go mon.Run(ctx) // collector runs in its own goroutine

	server := &api.Server{
		Auth: auth.NewService(cfg.JWTSecret,
			auth.User{Username: cfg.AdminUser, Password: cfg.AdminPassword, Role: auth.RoleAdmin},
			auth.User{Username: cfg.ViewerUser, Password: cfg.ViewerPassword, Role: auth.RoleViewer},
		),
		Monitor: mon,
		Rules:   rules,
		Hub:     h,
	}

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("listening on http://localhost:%s", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

// connectWithRetry waits for the database to become ready (useful with Docker Compose).
func connectWithRetry(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	for attempt := 1; attempt <= 30; attempt++ {
		if err = pool.Ping(ctx); err == nil {
			return pool, nil
		}
		log.Printf("waiting for database (attempt %d): %v", attempt, err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return nil, err
}

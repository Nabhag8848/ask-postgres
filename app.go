package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type appConfig struct {
	DSN          string
	Model        string
	OpenAIBase   string
	MaxRows      int
	QueryTimeout time.Duration
}

type app struct {
	cfg  appConfig
	pool *pgxpool.Pool

	model tuiModel
}

func newApp(ctx context.Context, cfg appConfig) (*app, error) {
	if cfg.MaxRows <= 0 {
		cfg.MaxRows = 200
	}
	if cfg.QueryTimeout <= 0 {
		cfg.QueryTimeout = 5 * time.Second
	}

	pool, err := pgxpool.New(ctx, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	if err := pingWithTimeout(ctx, pool, 5*time.Second); err != nil {
		pool.Close()
		return nil, err
	}

	agent, err := newAgent(ctx, agentConfig{
		Model:      cfg.Model,
		OpenAIBase: cfg.OpenAIBase,
		MaxRows:    cfg.MaxRows,
		Timeout:    cfg.QueryTimeout,
		Pool:       pool,
	})
	if err != nil {
		pool.Close()
		return nil, err
	}

	m := newTUImodel(cfg, agent)
	return &app{
		cfg:   cfg,
		pool:  pool,
		model: m,
	}, nil
}

func (a *app) Close() {
	if a.pool != nil {
		a.pool.Close()
	}
}

func pingWithTimeout(ctx context.Context, pool *pgxpool.Pool, d time.Duration) error {
	pingCtx, cancel := context.WithTimeout(ctx, d)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("db ping timeout after %s", d)
		}
		return fmt.Errorf("db ping: %w", err)
	}
	return nil
}

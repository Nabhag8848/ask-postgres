package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"pgwatch-copilot/internal/agent"
	"pgwatch-copilot/internal/session"
	"pgwatch-copilot/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds the CLI flags / env vars for the application.
type Config struct {
	DSN          string
	Model        string
	OpenAIBase   string
	MaxRows      int
	QueryTimeout time.Duration
}

// App owns the database pool and the Bubble Tea model.
type App struct {
	pool  *pgxpool.Pool
	model tea.Model
}

// New wires together the database pool, agent, session store, and TUI.
func New(ctx context.Context, cfg Config) (*App, error) {
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

	ag, err := agent.New(ctx, agent.Config{
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

	store, err := session.NewStore()
	if err != nil {
		pool.Close()
		return nil, err
	}

	var sess session.Session
	if latest, ok := store.Latest(); ok && latest.IsEmpty() {
		sess = latest
	} else {
		sess, err = store.New()
		if err != nil {
			pool.Close()
			return nil, err
		}
	}

	m := tui.New(tui.Config{DSN: cfg.DSN, Model: cfg.Model}, ag, store, sess)
	return &App{
		pool:  pool,
		model: m,
	}, nil
}

// Close releases the database pool.
func (a *App) Close() {
	if a.pool != nil {
		a.pool.Close()
	}
}

// Model returns the Bubble Tea model to run.
func (a *App) Model() tea.Model {
	return a.model
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

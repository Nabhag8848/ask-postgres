package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"pgwatch-copilot/internal/app"
	"pgwatch-copilot/internal/config"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	var (
		dsn          string
		model        string
		openAIBase   string
		maxRows      int
		queryTimeout time.Duration
	)

	flag.StringVar(&dsn, "db", "", "Postgres connection string (or set DATABASE_URL)")
	flag.StringVar(&model, "model", "gpt-4.1-mini", "LLM model name (OpenAI-compatible)")
	flag.StringVar(&openAIBase, "openai-base-url", "", "Optional OpenAI-compatible base URL")
	flag.IntVar(&maxRows, "max-rows", 200, "Max rows returned by SQL tool")
	flag.DurationVar(&queryTimeout, "query-timeout", 5*time.Second, "Per-query timeout for SQL tool")
	flag.Parse()

	// Priority: --model flag > MODEL env var > global config file > default.
	if model == "gpt-4.1-mini" {
		if envModel := strings.TrimSpace(os.Getenv("MODEL")); envModel != "" {
			model = envModel
		} else if gc := config.Load(); gc.Model != "" {
			model = gc.Model
		}
	}

	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
		if dsn == "" {
			fmt.Fprintln(os.Stderr, "missing --db (or set DATABASE_URL)")
			os.Exit(2)
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	a, err := app.New(ctx, app.Config{
		DSN:          dsn,
		Model:        model,
		OpenAIBase:   openAIBase,
		MaxRows:      maxRows,
		QueryTimeout: queryTimeout,
	})
	if err != nil {
		log.Fatalf("init: %v", err)
	}
	defer a.Close()

	p := tea.NewProgram(a.Model(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatalf("run: %v", err)
	}
}

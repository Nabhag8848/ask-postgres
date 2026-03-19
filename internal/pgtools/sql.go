package pgtools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SQLReadonly implements the langchaingo tools.Tool interface and executes
// safe, read-only SQL queries against the pool.
type SQLReadonly struct {
	pool    *pgxpool.Pool
	maxRows int
	timeout time.Duration
}

func NewSQLReadonly(pool *pgxpool.Pool, maxRows int, timeout time.Duration) *SQLReadonly {
	return &SQLReadonly{
		pool:    pool,
		maxRows: maxRows,
		timeout: timeout,
	}
}

func (t *SQLReadonly) Name() string { return "sql_readonly" }
func (t *SQLReadonly) Description() string {
	return `Run a safe, read-only SQL query (SELECT only).
Input JSON: {"query":"select ...","limit":100,"timeout_ms":5000}`
}

type sqlInput struct {
	Query     string `json:"query"`
	Limit     int    `json:"limit"`
	TimeoutMS int    `json:"timeout_ms"`
}

func (t *SQLReadonly) Call(ctx context.Context, input string) (string, error) {
	in := sqlInput{Limit: t.maxRows, TimeoutMS: int(t.timeout / time.Millisecond)}
	if strings.TrimSpace(input) != "" {
		if err := json.Unmarshal([]byte(input), &in); err != nil {
			in.Query = strings.TrimSpace(input)
		}
	}
	q := strings.TrimSpace(in.Query)
	if q == "" {
		return ErrorJSON("sql_readonly", errors.New("missing query")), nil
	}
	// Super-basic guardrail; for anything serious add pg_query_go.
	upper := strings.ToUpper(strings.TrimSpace(q))
	if strings.HasPrefix(upper, "WITH") {
		// allowed; ensure it ultimately SELECTs.
	} else if !strings.HasPrefix(upper, "SELECT") && !strings.HasPrefix(upper, "SHOW") && !strings.HasPrefix(upper, "EXPLAIN") {
		return ErrorJSON("sql_readonly", errors.New("only SELECT/EXPLAIN/SHOW are allowed in sql_readonly")), nil
	}

	limit := in.Limit
	if limit <= 0 || limit > t.maxRows {
		limit = t.maxRows
	}
	timeout := time.Duration(in.TimeoutMS) * time.Millisecond
	if timeout <= 0 || timeout > 30*time.Second {
		timeout = t.timeout
	}

	qctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	conn, err := t.pool.Acquire(qctx)
	if err != nil {
		return ErrorJSON("sql_readonly", err), nil
	}
	defer conn.Release()

	// Enforce read-only transaction at the session level for safety.
	if _, err := conn.Exec(qctx, "set default_transaction_read_only = on"); err != nil {
		return ErrorJSON("sql_readonly", err), nil
	}
	if _, err := conn.Exec(qctx, fmt.Sprintf("set statement_timeout = %d", int(timeout.Milliseconds()))); err != nil {
		return ErrorJSON("sql_readonly", err), nil
	}

	finalQuery := q
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(q)), "SELECT") && !strings.Contains(strings.ToUpper(q), "LIMIT") {
		finalQuery = fmt.Sprintf("%s LIMIT %d", strings.TrimRight(q, "; "), limit)
	}

	rows, err := conn.Query(qctx, finalQuery)
	if err != nil {
		return ErrorJSON("sql_readonly", err), nil
	}
	defer rows.Close()

	out, err := RowsToJSON(rows, limit)
	if err != nil {
		return ErrorJSON("sql_readonly", err), nil
	}
	return out, nil
}

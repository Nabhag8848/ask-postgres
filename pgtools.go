package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type schemaOverviewTool struct {
	pool *pgxpool.Pool
}

func newSchemaOverviewTool(pool *pgxpool.Pool) *schemaOverviewTool {
	return &schemaOverviewTool{pool: pool}
}
func (t *schemaOverviewTool) Name() string { return "schema_overview" }
func (t *schemaOverviewTool) Description() string {
	return "Get an overview of schemas/tables with estimated rows and total size. Input: empty string."
}
func (t *schemaOverviewTool) Call(ctx context.Context, _ string) (string, error) {
	const q = `
select
  n.nspname as schema,
  c.relname as table,
  pg_total_relation_size(c.oid) as bytes,
  c.reltuples::bigint as est_rows
from pg_class c
join pg_namespace n on n.oid = c.relnamespace
where c.relkind = 'r'
  and n.nspname not in ('pg_catalog','information_schema')
order by bytes desc
limit 30;
`
	rows, err := t.pool.Query(ctx, q)
	if err != nil {
		return toolErrorJSON("schema_overview", err), nil
	}
	defer rows.Close()

	type row struct {
		Schema  string `json:"schema"`
		Table   string `json:"table"`
		Bytes   int64  `json:"bytes"`
		EstRows int64  `json:"est_rows"`
	}
	var out []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.Schema, &r.Table, &r.Bytes, &r.EstRows); err != nil {
			return toolErrorJSON("schema_overview", err), nil
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return toolErrorJSON("schema_overview", err), nil
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	return string(b), nil
}

type describeTableTool struct {
	pool *pgxpool.Pool
}

func newDescribeTableTool(pool *pgxpool.Pool) *describeTableTool {
	return &describeTableTool{pool: pool}
}
func (t *describeTableTool) Name() string { return "describe_table" }
func (t *describeTableTool) Description() string {
	return "Describe a table's columns and indexes. Input: table name, optionally schema-qualified (e.g. public.users)."
}
func (t *describeTableTool) Call(ctx context.Context, input string) (string, error) {
	table := strings.TrimSpace(input)
	if table == "" {
		return toolErrorJSON("describe_table", errors.New("missing table name")), nil
	}
	schema := "public"
	name := table
	if strings.Contains(table, ".") {
		parts := strings.SplitN(table, ".", 2)
		schema, name = parts[0], parts[1]
	}

	const colQ = `
select
  column_name,
  data_type,
  is_nullable,
  column_default
from information_schema.columns
where table_schema = $1 and table_name = $2
order by ordinal_position;
`
	const idxQ = `
select
  indexname,
  indexdef
from pg_indexes
where schemaname = $1 and tablename = $2
order by indexname;
`

	cols, err := t.pool.Query(ctx, colQ, schema, name)
	if err != nil {
		return toolErrorJSON("describe_table", err), nil
	}
	defer cols.Close()
	type col struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Nullable string `json:"nullable"`
		Default  any    `json:"default"`
	}
	var colOut []col
	for cols.Next() {
		var c col
		if err := cols.Scan(&c.Name, &c.Type, &c.Nullable, &c.Default); err != nil {
			return toolErrorJSON("describe_table", err), nil
		}
		colOut = append(colOut, c)
	}
	if err := cols.Err(); err != nil {
		return toolErrorJSON("describe_table", err), nil
	}

	idx, err := t.pool.Query(ctx, idxQ, schema, name)
	if err != nil {
		return toolErrorJSON("describe_table", err), nil
	}
	defer idx.Close()
	type index struct {
		Name string `json:"name"`
		Def  string `json:"def"`
	}
	var idxOut []index
	for idx.Next() {
		var i index
		if err := idx.Scan(&i.Name, &i.Def); err != nil {
			return toolErrorJSON("describe_table", err), nil
		}
		idxOut = append(idxOut, i)
	}
	if err := idx.Err(); err != nil {
		return toolErrorJSON("describe_table", err), nil
	}

	resp := map[string]any{
		"table":   fmt.Sprintf("%s.%s", schema, name),
		"columns": colOut,
		"indexes": idxOut,
	}
	b, _ := json.MarshalIndent(resp, "", "  ")
	return string(b), nil
}

type sqlReadonlyTool struct {
	pool       *pgxpool.Pool
	maxRows    int
	timeout    time.Duration
	safePrefix string
}

func newSQLReadonlyTool(pool *pgxpool.Pool, maxRows int, timeout time.Duration) *sqlReadonlyTool {
	return &sqlReadonlyTool{
		pool:    pool,
		maxRows: maxRows,
		timeout: timeout,
	}
}

func (t *sqlReadonlyTool) Name() string { return "sql_readonly" }
func (t *sqlReadonlyTool) Description() string {
	return `Run a safe, read-only SQL query (SELECT only).
Input JSON: {"query":"select ...","limit":100,"timeout_ms":5000}`
}

type sqlInput struct {
	Query     string `json:"query"`
	Limit     int    `json:"limit"`
	TimeoutMS int    `json:"timeout_ms"`
}

func (t *sqlReadonlyTool) Call(ctx context.Context, input string) (string, error) {
	in := sqlInput{Limit: t.maxRows, TimeoutMS: int(t.timeout / time.Millisecond)}
	if strings.TrimSpace(input) != "" {
		if err := json.Unmarshal([]byte(input), &in); err != nil {
			// Allow a raw query string as a fallback.
			in.Query = strings.TrimSpace(input)
		}
	}
	q := strings.TrimSpace(in.Query)
	if q == "" {
		return toolErrorJSON("sql_readonly", errors.New("missing query")), nil
	}
	// Super-basic guardrail; for anything serious add pg_query_go.
	upper := strings.ToUpper(strings.TrimSpace(q))
	if strings.HasPrefix(upper, "WITH") {
		// allowed; ensure it ultimately SELECTs.
	} else if !strings.HasPrefix(upper, "SELECT") && !strings.HasPrefix(upper, "SHOW") && !strings.HasPrefix(upper, "EXPLAIN") {
		return toolErrorJSON("sql_readonly", errors.New("only SELECT/EXPLAIN/SHOW are allowed in sql_readonly")), nil
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
		return toolErrorJSON("sql_readonly", err), nil
	}
	defer conn.Release()

	// Enforce read-only transaction at the session level for safety.
	if _, err := conn.Exec(qctx, "set default_transaction_read_only = on"); err != nil {
		return toolErrorJSON("sql_readonly", err), nil
	}
	if _, err := conn.Exec(qctx, fmt.Sprintf("set statement_timeout = %d", int(timeout.Milliseconds()))); err != nil {
		return toolErrorJSON("sql_readonly", err), nil
	}

	finalQuery := q
	// Add LIMIT if not present and query seems like a plain SELECT.
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(q)), "SELECT") && !strings.Contains(strings.ToUpper(q), "LIMIT") {
		finalQuery = fmt.Sprintf("%s LIMIT %d", strings.TrimRight(q, "; "), limit)
	}

	rows, err := conn.Query(qctx, finalQuery)
	if err != nil {
		return toolErrorJSON("sql_readonly", err), nil
	}
	defer rows.Close()

	out, err := rowsToJSON(rows, limit)
	if err != nil {
		return toolErrorJSON("sql_readonly", err), nil
	}
	return out, nil
}

func rowsToJSON(rows pgx.Rows, maxRows int) (string, error) {
	fds := rows.FieldDescriptions()
	cols := make([]string, len(fds))
	for i := range fds {
		cols[i] = string(fds[i].Name)
	}

	var outRows []map[string]any
	count := 0
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return "", err
		}
		m := make(map[string]any, len(cols))
		for i := range cols {
			m[cols[i]] = vals[i]
		}
		outRows = append(outRows, m)
		count++
		if count >= maxRows {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}

	resp := map[string]any{
		"columns": cols,
		"rows":    outRows,
		"trunc":   count >= maxRows,
	}
	b, _ := json.MarshalIndent(resp, "", "  ")
	return string(b), nil
}

func toolErrorJSON(tool string, err error) string {
	msg := err.Error()
	code := ""
	if pe := (*pgconn.PgError)(nil); errors.As(err, &pe) && pe != nil {
		code = pe.Code
		if pe.Message != "" {
			msg = pe.Message
		}
	}
	resp := map[string]any{
		"tool":   tool,
		"ok":     false,
		"error":  msg,
		"code":   code,
		"detail": safeErrDetail(err),
	}
	b, _ := json.MarshalIndent(resp, "", "  ")
	return string(b)
}

func safeErrDetail(err error) string {
	// Keep it short; avoids dumping driver internals.
	s := strings.TrimSpace(err.Error())
	if len(s) > 400 {
		return s[:400] + "…"
	}
	return s
}

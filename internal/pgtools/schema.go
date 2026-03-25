package pgtools

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SchemaOverview implements the langchaingo tools.Tool interface and returns
// an overview of schemas/tables with estimated rows and total size.
type SchemaOverview struct {
	pool *pgxpool.Pool
}

func NewSchemaOverview(pool *pgxpool.Pool) *SchemaOverview {
	return &SchemaOverview{pool: pool}
}

func (t *SchemaOverview) Name() string { return "schema_overview" }
func (t *SchemaOverview) Description() string {
	return "Read-only: list user tables with estimated row counts and size. Use first to see what data exists. Input: empty string."
}

func (t *SchemaOverview) Call(ctx context.Context, _ string) (string, error) {
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
		return ErrorJSON("schema_overview", err), nil
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
			return ErrorJSON("schema_overview", err), nil
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return ErrorJSON("schema_overview", err), nil
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	return string(b), nil
}

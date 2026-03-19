package pgtools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DescribeTable implements the langchaingo tools.Tool interface and returns
// column and index information for a given table.
type DescribeTable struct {
	pool *pgxpool.Pool
}

func NewDescribeTable(pool *pgxpool.Pool) *DescribeTable {
	return &DescribeTable{pool: pool}
}

func (t *DescribeTable) Name() string { return "describe_table" }
func (t *DescribeTable) Description() string {
	return "Describe a table's columns and indexes. Input: table name, optionally schema-qualified (e.g. public.users)."
}

func (t *DescribeTable) Call(ctx context.Context, input string) (string, error) {
	table := strings.TrimSpace(input)
	if table == "" {
		return ErrorJSON("describe_table", errors.New("missing table name")), nil
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
		return ErrorJSON("describe_table", err), nil
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
			return ErrorJSON("describe_table", err), nil
		}
		colOut = append(colOut, c)
	}
	if err := cols.Err(); err != nil {
		return ErrorJSON("describe_table", err), nil
	}

	idx, err := t.pool.Query(ctx, idxQ, schema, name)
	if err != nil {
		return ErrorJSON("describe_table", err), nil
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
			return ErrorJSON("describe_table", err), nil
		}
		idxOut = append(idxOut, i)
	}
	if err := idx.Err(); err != nil {
		return ErrorJSON("describe_table", err), nil
	}

	resp := map[string]any{
		"table":   fmt.Sprintf("%s.%s", schema, name),
		"columns": colOut,
		"indexes": idxOut,
	}
	b, _ := json.MarshalIndent(resp, "", "  ")
	return string(b), nil
}

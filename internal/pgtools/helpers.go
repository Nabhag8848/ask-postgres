package pgtools

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
)

// RowsToJSON serialises pgx rows into a JSON object with columns, rows, and
// a truncation flag.
func RowsToJSON(rows pgx.Rows, maxRows int) (string, error) {
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

// ErrorJSON builds a structured JSON error response for tool failures.
func ErrorJSON(tool string, err error) string {
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
	s := strings.TrimSpace(err.Error())
	if len(s) > 400 {
		return s[:400] + "…"
	}
	return s
}

package database

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/jahrulnr/gosite/pkg/apperror"
)

var tableNamePattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// allowedTables is the whitelist of SQLite tables exposed in the viewer.
var allowedTables = map[string]struct{}{
	"users":           {},
	"websites":        {},
	"cronjobs":        {},
	"settings":        {},
	"audit_logs":      {},
	"job_runs":        {},
	"log_events":      {},
	"traffic_metrics": {},
	"saved_queries":   {},
}

// DB exposes read-only table browsing.
type DB interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// Service provides a read-only SQLite table viewer.
type Service struct {
	db   DB
	path string
}

// NewService returns a database viewer service.
func NewService(db DB, path string) *Service {
	return &Service{db: db, path: path}
}

// TablesResult lists whitelisted tables and the database path.
type TablesResult struct {
	Path   string   `json:"path"`
	Tables []string `json:"tables"`
}

// TableData is column names and row values for one table page.
type TableData struct {
	Name    string     `json:"name"`
	Columns []string   `json:"columns"`
	Rows    [][]any    `json:"rows"`
	Limit   int        `json:"limit"`
	Offset  int        `json:"offset"`
	Count   int        `json:"count"`
}

// ListTables returns whitelisted tables sorted by name.
func (s *Service) ListTables(_ context.Context) (TablesResult, error) {
	names := make([]string, 0, len(allowedTables))
	for name := range allowedTables {
		names = append(names, name)
	}
	sortStrings(names)
	return TablesResult{
		Path:   s.path,
		Tables: names,
	}, nil
}

// GetTable returns rows for a whitelisted table.
func (s *Service) GetTable(ctx context.Context, name string, limit, offset int) (TableData, error) {
	if err := validateTableName(name); err != nil {
		return TableData{}, err
	}
	if _, ok := allowedTables[name]; !ok {
		return TableData{}, apperror.New(apperror.CodeForbidden, "table is not allowed")
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}

	cols, err := s.tableColumns(ctx, name)
	if err != nil {
		return TableData{}, apperror.Wrap(apperror.CodeDatabase, "read table columns failed", err)
	}
	if cols == nil {
		cols = []string{}
	}

	query := fmt.Sprintf("SELECT * FROM %s LIMIT ? OFFSET ?", quoteIdent(name))
	rows, err := s.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return TableData{}, apperror.Wrap(apperror.CodeDatabase, "read table rows failed", err)
	}
	defer rows.Close()

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return TableData{}, apperror.Wrap(apperror.CodeDatabase, "read column types failed", err)
	}

	var outRows = make([][]any, 0)
	for rows.Next() {
		values := make([]any, len(colTypes))
		ptrs := make([]any, len(colTypes))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return TableData{}, apperror.Wrap(apperror.CodeDatabase, "scan row failed", err)
		}
		for i, v := range values {
			values[i] = normalizeCell(v)
		}
		outRows = append(outRows, values)
	}
	if err := rows.Err(); err != nil {
		return TableData{}, apperror.Wrap(apperror.CodeDatabase, "iterate rows failed", err)
	}

	return TableData{
		Name:    name,
		Columns: cols,
		Rows:    outRows,
		Limit:   limit,
		Offset:  offset,
		Count:   len(outRows),
	}, nil
}

func (s *Service) tableColumns(ctx context.Context, name string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", quoteIdent(name)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var cid int
		var colName, colType string
		var notNull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &colName, &colType, &notNull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols = append(cols, colName)
	}
	return cols, rows.Err()
}

func validateTableName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return apperror.New(apperror.CodeInvalidInput, "table name is required")
	}
	if !tableNamePattern.MatchString(name) {
		return apperror.New(apperror.CodeInvalidInput, "invalid table name")
	}
	return nil
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func normalizeCell(v any) any {
	switch val := v.(type) {
	case []byte:
		return string(val)
	case nil:
		return nil
	default:
		return val
	}
}

func sortStrings(values []string) {
	for i := 0; i < len(values); i++ {
		for j := i + 1; j < len(values); j++ {
			if values[j] < values[i] {
				values[i], values[j] = values[j], values[i]
			}
		}
	}
}

package sqlite

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	sqlitedrv "modernc.org/sqlite"
)

// DefaultMigrationsDir is the relative path to SQL migration files.
const DefaultMigrationsDir = "migrations"

// regexpRegistered tracks whether the REGEXP() user function has been
// registered for the process. modernc.org/sqlite's RegisterScalarFunction is
// process-global, so we register it once at startup.
var regexpRegistered bool

// init ensures the REGEXP scalar function is registered before any
// connection is opened.
func init() {
	RegisterRegexpOnce()
}

// RegisterRegexpOnce installs a Go-regexp-backed REGEXP() function on the
// process-wide sqlite driver. Subsequent calls are no-ops. Exposed publicly
// so tests can ensure the function is available.
func RegisterRegexpOnce() {
	if regexpRegistered {
		return
	}
	if err := sqlitedrv.RegisterDeterministicScalarFunction("REGEXP", 2, regexpSQLFn); err == nil {
		regexpRegistered = true
	}
}

// regexpSQLFn implements SQLite's REGEXP(pattern, value) scalar function using
// Go regexp semantics.
func regexpSQLFn(_ *sqlitedrv.FunctionContext, args []driver.Value) (driver.Value, error) {
	if len(args) < 2 {
		return false, nil
	}
	pattern, _ := args[0].(string)
	value, _ := args[1].(string)
	if pattern == "" {
		return true, nil
	}
	if value == "" {
		return false, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, nil
	}
	return re.MatchString(value), nil
}

// Open opens a SQLite database at the given path.
func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	dsn := fmt.Sprintf("file:%s?_txlock=immediate&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	// SQLite does not support concurrent writers. Limit connections to avoid
	// SQLITE_BUSY errors. Allow a small pool for concurrent reads in WAL mode.
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite database: %w", err)
	}

	return db, nil
}

// Migrate applies pending SQL migrations from dir in lexical order.
func Migrate(db *sql.DB, dir string) error {
	if dir == "" {
		dir = DefaultMigrationsDir
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations dir %s: %w", dir, err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		files = append(files, entry.Name())
	}
	sort.Strings(files)

	for _, name := range files {
		var exists int
		if err := db.QueryRow(`SELECT COUNT(1) FROM schema_migrations WHERE version = ?`, name).Scan(&exists); err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if exists > 0 {
			continue
		}

		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", name, err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", name, err)
		}

		if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, name); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", name, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}

	return nil
}

// Vacuum reclaims free pages from the SQLite database file, shrinking it on
// disk. It should be called after bulk deletes (e.g. retention purge) to
// prevent the database file from growing indefinitely. VACUUM requires an
// exclusive lock, so callers should ensure no concurrent writes are in
// flight.
func Vacuum(db *sql.DB) error {
	if _, err := db.Exec(`VACUUM`); err != nil {
		return fmt.Errorf("vacuum sqlite: %w", err)
	}
	return nil
}

// ListTables returns user-visible table names from sqlite_master.
func ListTables(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

package sqlite_test

import (
	"path/filepath"
	"testing"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func migrationsDir(t *testing.T) string {
	t.Helper()
	return filepath.Clean(filepath.Join("..", "..", "..", "migrations"))
}

func TestMigrate_ApplyAllTables(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "gosite.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	require.NoError(t, sqlite.Migrate(db, migrationsDir(t)))

	tables, err := sqlite.ListTables(db)
	require.NoError(t, err)

	for _, expected := range []string{
		"users",
		"websites",
		"cronjobs",
		"settings",
		"audit_logs",
		"job_runs",
		"log_events",
		"traffic_metrics",
		"saved_queries",
		"schema_migrations",
		"sessions",
	} {
		assert.Contains(t, tables, expected)
	}

	var columnCount int
	require.NoError(t, db.QueryRow(`
		SELECT COUNT(1) FROM pragma_table_info('websites') WHERE name IN ('type', 'upstream')
	`).Scan(&columnCount))
	assert.Equal(t, 2, columnCount)
}

func TestMigrate_Idempotent(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "gosite.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	dir := migrationsDir(t)
	require.NoError(t, sqlite.Migrate(db, dir))
	require.NoError(t, sqlite.Migrate(db, dir))

	tables, err := sqlite.ListTables(db)
	require.NoError(t, err)
	assert.Len(t, tables, 11)

	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(1) FROM schema_migrations`).Scan(&count))
	assert.Equal(t, 4, count)
}

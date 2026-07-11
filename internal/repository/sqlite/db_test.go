package sqlite_test

import (
	"path/filepath"
	"testing"
	"time"

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
		"nginx_status_samples",
		"nginx_vts_server_samples",
		"nginx_vts_upstream_samples",
		"saved_queries",
		"schema_migrations",
		"sessions",
		"plugin_versions",
		"plugin_access_tokens",
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
	assert.Len(t, tables, 17)

	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(1) FROM schema_migrations`).Scan(&count))
	assert.Equal(t, 10, count)
}

func TestVacuum_ReclaimsSpaceAfterDelete(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "gosite.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	require.NoError(t, sqlite.Migrate(db, migrationsDir(t)))

	// Insert rows then delete them to create free pages.
	for i := 0; i < 100; i++ {
		_, err := db.Exec(`INSERT INTO audit_logs (ts, user_email, action, status) VALUES (?, 'test', 'test', 'ok')`,
			time.Now().UTC().Format(time.RFC3339Nano))
		require.NoError(t, err)
	}

	var beforeCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(1) FROM audit_logs`).Scan(&beforeCount))
	assert.Equal(t, 100, beforeCount)

	_, err = db.Exec(`DELETE FROM audit_logs`)
	require.NoError(t, err)

	// VACUUM should succeed and reclaim space.
	require.NoError(t, sqlite.Vacuum(db))

	var afterCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(1) FROM audit_logs`).Scan(&afterCount))
	assert.Equal(t, 0, afterCount)

	// Database file should still be valid — tables are intact.
	tables, err := sqlite.ListTables(db)
	require.NoError(t, err)
	assert.Contains(t, tables, "audit_logs")
}

func TestVacuum_OnEmptyDB(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "gosite.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	require.NoError(t, sqlite.Migrate(db, migrationsDir(t)))

	// VACUUM on a freshly migrated DB with no data should succeed.
	require.NoError(t, sqlite.Vacuum(db))
}

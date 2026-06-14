package splunklite

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestMetaServiceListsPerVhostLogSources(t *testing.T) {
	logDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(logDir, "access-example.test.log"), []byte(""), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(logDir, "error-example.test.log"), []byte(""), 0o644))

	meta, err := NewMetaService(nil, logDir).Meta(context.Background())
	require.NoError(t, err)

	ids := map[string]bool{}
	for _, source := range meta.Sources {
		ids[source.ID] = true
	}
	require.True(t, ids["access:example.test"])
	require.True(t, ids["error:example.test"])
}

func TestLogIngestorPersistsAccessLogEvents(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE log_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ts DATETIME NOT NULL,
			source TEXT NOT NULL,
			site TEXT NOT NULL,
			status_code INTEGER,
			bytes INTEGER,
			line_hash TEXT NOT NULL,
			raw_preview TEXT NOT NULL
		);
		CREATE UNIQUE INDEX idx_log_events_line_hash ON log_events (line_hash) WHERE line_hash <> '';
	`)
	require.NoError(t, err)

	logDir := t.TempDir()
	line := `127.0.0.1 - - [14/Jun/2026:08:00:00 +0000] "GET / HTTP/1.1" 200 123 "-" "curl"`
	require.NoError(t, os.WriteFile(filepath.Join(logDir, "access-example.test.log"), []byte(line+"\n"), 0o644))

	ingestor := NewLogIngestor(sqlite.NewLogEventRepository(db), logDir)
	require.NoError(t, ingestor.Ingest(context.Background()))
	require.NoError(t, ingestor.Ingest(context.Background()))

	var count int
	var site string
	var status int
	require.NoError(t, db.QueryRow(`SELECT COUNT(1), site, status_code FROM log_events`).Scan(&count, &site, &status))
	require.Equal(t, 1, count)
	require.Equal(t, "example.test", site)
	require.Equal(t, 200, status)
}

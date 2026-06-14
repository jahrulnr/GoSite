package auth_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/auth"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "s.db")+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`CREATE TABLE sessions (
		id TEXT PRIMARY KEY, user_id INTEGER NOT NULL,
		created_at DATETIME NOT NULL, expires_at DATETIME NOT NULL
	)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

func TestSession_PersisterCreateAndGet(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	repo := sqlite.NewSessionRepository(db)
	persister := auth.NewSQLitePersister(repo)
	store := auth.NewStoreWithOptions(time.Hour, true, persister)

	s, err := store.CreateFor(7, false)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if s.UserID != 7 {
		t.Fatalf("expected user 7, got %d", s.UserID)
	}

	got, ok := store.Get(s.ID)
	if !ok || got.UserID != 7 {
		t.Fatalf("cache hit failed: ok=%v got=%+v", ok, got)
	}
}

func TestSession_FallbackToPersisterOnCacheMiss(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	repo := sqlite.NewSessionRepository(db)
	persister := auth.NewSQLitePersister(repo)

	writer := auth.NewStoreWithOptions(time.Hour, true, persister)
	s, err := writer.CreateFor(99, false)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// fresh store, empty cache
	reader := auth.NewStoreWithOptions(time.Hour, true, persister)
	got, ok := reader.Get(s.ID)
	if !ok {
		t.Fatalf("expected persister hit")
	}
	if got.UserID != 99 {
		t.Fatalf("expected user 99, got %d", got.UserID)
	}
}

func TestSession_DeleteRemovesFromPersister(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	repo := sqlite.NewSessionRepository(db)
	persister := auth.NewSQLitePersister(repo)
	store := auth.NewStoreWithOptions(time.Hour, true, persister)

	s, err := store.CreateFor(1, false)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	store.Delete(s.ID)

	if _, ok := store.Get(s.ID); ok {
		t.Fatal("expected cache miss after delete")
	}
	if _, ok := persister.Get(context.Background(), s.ID); ok {
		t.Fatal("expected persister miss after delete")
	}
}

func TestSession_PurgeExpiredRemovesStaleRows(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	repo := sqlite.NewSessionRepository(db)

	// directly insert a stale row + a fresh row (UTC, matching repository writes)
	now := time.Now().UTC()
	_, err := db.Exec(`INSERT INTO sessions (id, user_id, created_at, expires_at) VALUES (?,?,?,?)`,
		"stale", 1, now.Add(-2*time.Hour), now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("insert stale: %v", err)
	}
	_, err = db.Exec(`INSERT INTO sessions (id, user_id, created_at, expires_at) VALUES (?,?,?,?)`,
		"fresh", 1, now, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("insert fresh: %v", err)
	}

	n, err := repo.PurgeExpired(context.Background())
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected to purge 1 row, purged %d", n)
	}
}

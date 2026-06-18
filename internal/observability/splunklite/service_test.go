package splunklite_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/jahrulnr/gosite/internal/contracts"
	"github.com/jahrulnr/gosite/internal/observability/splunklite"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/pkg/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testEnv struct {
	svc   *splunklite.Service
	audit *sqlite.AuditRepository
	jobs  *sqlite.JobRepository
	logs  *sqlite.LogEventRepository
	saved *sqlite.SavedQueryRepository
}

func setupSplunk(t *testing.T) testEnv {
	t.Helper()
	db := openTestDB(t)
	audit := sqlite.NewAuditRepository(db)
	jobs := sqlite.NewJobRepository(db)
	logs := sqlite.NewLogEventRepository(db)
	saved := sqlite.NewSavedQueryRepository(db)
	svc := splunklite.NewService(audit, jobs, logs, saved, 90, 14)
	ingestor := splunklite.NewLogIngestor(logs, t.TempDir())
	svc.SetIngestor(ingestor)
	return testEnv{svc: svc, audit: audit, jobs: jobs, logs: logs, saved: saved}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "gosite.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, migrationsDir(t)))
	return db
}

func migrationsDir(t *testing.T) string {
	t.Helper()
	return filepath.Clean(filepath.Join("..", "..", "..", "migrations"))
}

func TestQuery_TimeRangeInverted(t *testing.T) {
	t.Parallel()

	env := setupSplunk(t)
	from := time.Now().UTC()
	to := from.Add(-1 * time.Hour)
	_, err := env.svc.Query(context.Background(), splunklite.QueryRequest{
		Source: "audit",
		From:   &from,
		To:     &to,
	})
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperror.CodeTimeRangeInvalid, appErr.Code)
}

func TestQuery_UnknownFieldSkipped(t *testing.T) {
	t.Parallel()

	env := setupSplunk(t)
	ctx := context.Background()
	writer := splunklite.NewAuditWriter(env.audit)
	require.NoError(t, writer.Write(ctx, contracts.AuditEntry{
		Action: "website.create", Status: "ok", Message: "ok",
	}))

	res, err := env.svc.Query(ctx, splunklite.QueryRequest{
		Source: "audit",
		Query:  "bogus=field",
	})
	require.NoError(t, err)
	assert.Equal(t, 0, res.Hits)
	assert.Empty(t, res.Events)
}

func TestAudit_WriteOnMutation(t *testing.T) {
	t.Parallel()

	env := setupSplunk(t)
	writer := splunklite.NewAuditWriter(env.audit)
	ctx := context.Background()

	err := writer.Write(ctx, contracts.AuditEntry{
		Timestamp: time.Now().UTC(),
		UserEmail: "admin@demo.com",
		Action:    "website.create",
		Domain:    "example.test",
		Status:    "ok",
		Message:   "created website",
		MetaJSON:  `{"id":1}`,
	})
	require.NoError(t, err)

	res, err := env.svc.Query(ctx, splunklite.QueryRequest{
		Source: "audit",
		Query:  "action:website.create user:admin@demo.com",
	})
	require.NoError(t, err)
	require.Equal(t, 1, res.Hits)
	require.Len(t, res.Events, 1)
	assert.Equal(t, "website.create", res.Events[0].Action)
}

func TestQuery_AuditFreeText(t *testing.T) {
	t.Parallel()

	env := setupSplunk(t)
	ctx := context.Background()
	writer := splunklite.NewAuditWriter(env.audit)
	require.NoError(t, writer.Write(ctx, contracts.AuditEntry{
		UserEmail: "admin@demo.com",
		Action:    "login",
		Status:    "ok",
		Message:   "user signed in",
	}))

	res, err := env.svc.Query(ctx, splunklite.QueryRequest{
		Source: "audit",
		Query:  "login",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Hits)
}

func TestQuery_AuditWildcardMatch(t *testing.T) {
	t.Parallel()

	env := setupSplunk(t)
	ctx := context.Background()
	writer := splunklite.NewAuditWriter(env.audit)
	require.NoError(t, writer.Write(ctx, contracts.AuditEntry{
		UserEmail: "admin@demo.com",
		Action:    "website.update",
		Status:    "ok",
		Message:   "updated",
	}))
	require.NoError(t, writer.Write(ctx, contracts.AuditEntry{
		UserEmail: "ops@demo.com",
		Action:    "nginx.reload",
		Status:    "ok",
		Message:   "reloaded",
	}))

	res, err := env.svc.Query(ctx, splunklite.QueryRequest{
		Source: "audit",
		Query:  "action:website.*",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Hits)
}

func TestQuery_JobSource(t *testing.T) {
	t.Parallel()

	env := setupSplunk(t)
	ctx := context.Background()
	_, err := env.jobs.Create(ctx, sqlite.JobRun{
		JobType: "ssl.certbot",
		Name:    "example.test",
		Status:  sqlite.JobStatusOK,
		Output:  "success",
	})
	require.NoError(t, err)

	res, err := env.svc.Query(ctx, splunklite.QueryRequest{
		Source: "job",
		Query:  "job_type=ssl.* status=ok",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Hits)
}

func TestQuery_LogSource(t *testing.T) {
	t.Parallel()

	env := setupSplunk(t)
	ctx := context.Background()
	code := 404
	bytes := 89
	_, err := env.logs.Insert(ctx, sqlite.LogEvent{
		Timestamp:  time.Now().UTC(),
		Source:     sqlite.LogSourceAccess,
		Site:       "example.test",
		StatusCode: &code,
		Bytes:      &bytes,
		RawPreview: `GET /missing 404`,
	})
	require.NoError(t, err)

	res, err := env.svc.Query(ctx, splunklite.QueryRequest{
		Source: "access",
		Query:  "site=example.test status=404",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Hits)
}

func TestQuery_AllSourcesMerge(t *testing.T) {
	t.Parallel()

	env := setupSplunk(t)
	ctx := context.Background()
	writer := splunklite.NewAuditWriter(env.audit)
	require.NoError(t, writer.Write(ctx, contracts.AuditEntry{
		Action: "website.create", Status: "ok", Message: "ok",
	}))

	res, err := env.svc.Query(ctx, splunklite.QueryRequest{Source: "all"})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, res.Hits, 1)
	assert.NotEmpty(t, res.Events)
}

func TestQuery_AllSources_StatusCodeSkipsAudit(t *testing.T) {
	t.Parallel()

	env := setupSplunk(t)
	ctx := context.Background()
	writer := splunklite.NewAuditWriter(env.audit)
	require.NoError(t, writer.Write(ctx, contracts.AuditEntry{
		Action: "website.create", Status: "ok", Message: "audit row",
	}))

	code := 302
	_, err := env.logs.Insert(ctx, sqlite.LogEvent{
		Timestamp:  time.Now().UTC(),
		Source:     sqlite.LogSourceAccess,
		Site:       "bangunsoft.com",
		StatusCode: &code,
		RawPreview: `GET /old 302`,
	})
	require.NoError(t, err)

	res, err := env.svc.Query(ctx, splunklite.QueryRequest{
		Source: "all",
		Query:  `status=/^3\d{2}$/`,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Hits)
	require.Len(t, res.Events, 1)
	assert.Equal(t, "access", res.Events[0].Source)
}

func TestQuery_Access_StatusCodeRegexp(t *testing.T) {
	t.Parallel()

	env := setupSplunk(t)
	ctx := context.Background()
	for _, code := range []int{200, 302, 404} {
		status := code
		_, err := env.logs.Insert(ctx, sqlite.LogEvent{
			Timestamp:  time.Now().UTC(),
			Source:     sqlite.LogSourceAccess,
			Site:       "bangunsoft.com",
			StatusCode: &status,
			RawPreview: "sample",
		})
		require.NoError(t, err)
	}

	res, err := env.svc.Query(ctx, splunklite.QueryRequest{
		Source: "access",
		Site:   "bangunsoft.com",
		Query:  `status=/^3\d{2}$/`,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Hits)
}

func TestQuery_Access_StatusCodeWildcard(t *testing.T) {
	t.Parallel()

	env := setupSplunk(t)
	ctx := context.Background()
	code := 301
	_, err := env.logs.Insert(ctx, sqlite.LogEvent{
		Timestamp:  time.Now().UTC(),
		Source:     sqlite.LogSourceAccess,
		Site:       "bangunsoft.com",
		StatusCode: &code,
		RawPreview: `GET /moved 301`,
	})
	require.NoError(t, err)

	res, err := env.svc.Query(ctx, splunklite.QueryRequest{
		Source: "access",
		Site:   "bangunsoft.com",
		Query:  `status=3*`,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Hits)
}

func TestSaveQuery_Persists(t *testing.T) {
	t.Parallel()

	env := setupSplunk(t)
	ctx := context.Background()
	q, err := env.svc.SaveQuery(ctx, "failed ssl", "job", "status=failed")
	require.NoError(t, err)
	assert.Equal(t, "failed ssl", q.Name)

	list, err := env.svc.ListSavedQueries(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "status=failed", list[0].Query)
}

func TestRecentAudit_ReturnsLatest(t *testing.T) {
	t.Parallel()

	env := setupSplunk(t)
	ctx := context.Background()
	writer := splunklite.NewAuditWriter(env.audit)
	for i := 0; i < 3; i++ {
		require.NoError(t, writer.Write(ctx, contracts.AuditEntry{
			Action:  "website.create",
			Status:  "ok",
			Message: "created",
		}))
	}
	events, err := env.svc.RecentAudit(ctx, 2)
	require.NoError(t, err)
	assert.Len(t, events, 2)
}

func TestPurgeRetention_RemovesOldAudit(t *testing.T) {
	t.Parallel()

	env := setupSplunk(t)
	ctx := context.Background()
	oldTS := time.Now().UTC().Add(-100 * 24 * time.Hour)
	require.NoError(t, splunklite.NewAuditWriter(env.audit).Write(ctx, contracts.AuditEntry{
		Timestamp: oldTS,
		Action:    "website.delete",
		Status:    "ok",
		Message:   "old",
	}))
	require.NoError(t, env.svc.PurgeRetention(ctx, time.Now().UTC()))

	res, err := env.svc.Query(ctx, splunklite.QueryRequest{Source: "audit"})
	require.NoError(t, err)
	assert.Equal(t, 0, res.Hits)
}

func TestQuery_DefaultLimit(t *testing.T) {
	t.Parallel()

	env := setupSplunk(t)
	ctx := context.Background()
	writer := splunklite.NewAuditWriter(env.audit)
	require.NoError(t, writer.Write(ctx, contracts.AuditEntry{Action: "ping", Status: "ok"}))

	res, err := env.svc.Query(ctx, splunklite.QueryRequest{Source: "audit"})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Hits)
}

func TestQuery_UnknownSource(t *testing.T) {
	t.Parallel()

	env := setupSplunk(t)
	_, err := env.svc.Query(context.Background(), splunklite.QueryRequest{Source: "nope"})
	require.Error(t, err)
}

func TestQuery_RegexAudit(t *testing.T) {
	t.Parallel()

	env := setupSplunk(t)
	ctx := context.Background()
	writer := splunklite.NewAuditWriter(env.audit)
	require.NoError(t, writer.Write(ctx, contracts.AuditEntry{
		Action:  "website.create",
		Status:  "ok",
		Message: "created",
	}))
	require.NoError(t, writer.Write(ctx, contracts.AuditEntry{
		Action:  "login",
		Status:  "ok",
		Message: "signed in",
	}))

	res, err := env.svc.Query(ctx, splunklite.QueryRequest{
		Source: "audit",
		Query:  `action=/^website\..*/`,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Hits)
}

func TestQuery_RegexLogEvent(t *testing.T) {
	t.Parallel()

	env := setupSplunk(t)
	ctx := context.Background()
	code := 404
	_, err := env.logs.Insert(ctx, sqlite.LogEvent{
		Source:     sqlite.LogSourceAccess,
		Site:       "example.test",
		StatusCode: &code,
		RawPreview: `GET /missing 404`,
	})
	require.NoError(t, err)

	res, err := env.svc.Query(ctx, splunklite.QueryRequest{
		Source: "access",
		Query:  `message=/missing/`,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Hits)
}

func TestUpdateSavedQuery(t *testing.T) {
	t.Parallel()

	env := setupSplunk(t)
	ctx := context.Background()
	q, err := env.svc.SaveQuery(ctx, "renamed", "job", "status=failed")
	require.NoError(t, err)

	updated, err := env.svc.UpdateSavedQuery(ctx, q.ID, "renamed v2", "", "")
	require.NoError(t, err)
	assert.Equal(t, "renamed v2", updated.Name)
	assert.Equal(t, "status=failed", updated.Query)

	list, err := env.svc.ListSavedQueries(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "renamed v2", list[0].Name)
}

func TestUpdateSavedQuery_NotFound(t *testing.T) {
	t.Parallel()

	env := setupSplunk(t)
	_, err := env.svc.UpdateSavedQuery(context.Background(), 9999, "x", "", "")
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperror.CodeNotFound, appErr.Code)
}

func TestUpdateSavedQuery_InvalidQuery(t *testing.T) {
	t.Parallel()

	env := setupSplunk(t)
	ctx := context.Background()
	q, err := env.svc.SaveQuery(ctx, "ok", "job", "status=ok")
	require.NoError(t, err)

	_, err = env.svc.UpdateSavedQuery(ctx, q.ID, "", "", `/[invalid/`)
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperror.CodeQueryInvalid, appErr.Code)
}

func TestDeleteSavedQuery(t *testing.T) {
	t.Parallel()

	env := setupSplunk(t)
	ctx := context.Background()
	q, err := env.svc.SaveQuery(ctx, "doomed", "job", "status=failed")
	require.NoError(t, err)

	require.NoError(t, env.svc.DeleteSavedQuery(ctx, q.ID))

	list, err := env.svc.ListSavedQueries(ctx)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestDeleteSavedQuery_NotFound(t *testing.T) {
	t.Parallel()

	env := setupSplunk(t)
	err := env.svc.DeleteSavedQuery(context.Background(), 9999)
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperror.CodeNotFound, appErr.Code)
}

func TestTail_StreamsEvents(t *testing.T) {
	t.Parallel()

	env := setupSplunk(t)
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	ch := make(chan splunklite.QueryEvent, 16)
	tailErr := make(chan error, 1)
	go func() { tailErr <- env.svc.Tail(ctx, "audit", "", ch, "") }()

	// Give Tail a moment to seed its cursor at time.Now() before we start
	// writing, so the new events are picked up on the next poll.
	time.Sleep(50 * time.Millisecond)

	writer := splunklite.NewAuditWriter(env.audit)
	for i := 0; i < 3; i++ {
		require.NoError(t, writer.Write(ctx, contracts.AuditEntry{
			Action:  "website.create",
			Status:  "ok",
			Message: "created",
		}))
	}

	got := 0
	for {
		select {
		case ev := <-ch:
			got++
			assert.Equal(t, splunklite.SourceAudit, ev.Source)
		case <-ctx.Done():
			// wait briefly for the tail goroutine to exit cleanly
			select {
			case <-tailErr:
			case <-time.After(200 * time.Millisecond):
			}
			require.GreaterOrEqual(t, got, 3)
			return
		}
	}
}

func TestTailQuery_FiltersEvents(t *testing.T) {
	t.Parallel()

	env := setupSplunk(t)
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	ch := make(chan splunklite.QueryEvent, 16)
	tailErr := make(chan error, 1)
	go func() { tailErr <- env.svc.TailQuery(ctx, "audit", "", "curl", ch, "") }()

	time.Sleep(50 * time.Millisecond)

	writer := splunklite.NewAuditWriter(env.audit)
	require.NoError(t, writer.Write(ctx, contracts.AuditEntry{
		Action:  "website.create",
		Status:  "ok",
		Message: "created by browser",
	}))
	require.NoError(t, writer.Write(ctx, contracts.AuditEntry{
		Action:  "website.create",
		Status:  "ok",
		Message: "created by curl",
	}))

	var got []splunklite.QueryEvent
	for {
		select {
		case ev := <-ch:
			got = append(got, ev)
		case <-ctx.Done():
			select {
			case <-tailErr:
			case <-time.After(200 * time.Millisecond):
			}
			require.Len(t, got, 1)
			assert.Contains(t, got[0].Message, "curl")
			return
		}
	}
}

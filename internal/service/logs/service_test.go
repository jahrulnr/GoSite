package logs_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/logs"
	"github.com/jahrulnr/gosite/pkg/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubWebsites struct {
	sites []sqlite.Website
	err   error
}

func (s stubWebsites) List(_ context.Context) ([]sqlite.Website, error) {
	return s.sites, s.err
}

func TestLogs_ListSites_IncludesDefault(t *testing.T) {
	t.Parallel()
	svc := logs.NewService(t.TempDir(), stubWebsites{sites: []sqlite.Website{
		{Domain: "a.test", Name: "A"},
	}})
	out, err := svc.ListSites(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 2)
	assert.Equal(t, "default", out[0].Domain)
	assert.Equal(t, "a.test", out[1].Domain)
}

func TestLogs_Tail_GlobalAccessLog(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "access.log"), []byte("line1\nline2\n"), 0o644))
	svc := logs.NewService(dir, stubWebsites{})
	result, err := svc.Tail(context.Background(), logs.TailInput{Type: "access", Tail: 10})
	require.NoError(t, err)
	assert.Equal(t, "default", result.Domain)
	assert.Contains(t, result.Lines[len(result.Lines)-1], "line2")
}

func TestLogs_Tail_DomainAccessLog(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "access-site.test.log"), []byte("domain log\n"), 0o644))
	svc := logs.NewService(dir, stubWebsites{})
	result, err := svc.Tail(context.Background(), logs.TailInput{
		Domain: "site.test",
		Type:   "accesslog",
		Tail:   5,
	})
	require.NoError(t, err)
	assert.Equal(t, "site.test", result.Domain)
	assert.Contains(t, result.Content, "domain log")
}

func TestLogs_Tail_ErrorLog(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "error.log"), []byte("error one\n"), 0o644))
	svc := logs.NewService(dir, stubWebsites{})
	result, err := svc.Tail(context.Background(), logs.TailInput{Type: "errorlog"})
	require.NoError(t, err)
	assert.Equal(t, "error", result.Type)
}

func TestLogs_Tail_MissingFile(t *testing.T) {
	t.Parallel()
	svc := logs.NewService(t.TempDir(), stubWebsites{})
	_, err := svc.Tail(context.Background(), logs.TailInput{Type: "access"})
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperror.CodeNotFound, appErr.Code)
}

func TestLogs_Tail_InvalidType(t *testing.T) {
	t.Parallel()
	svc := logs.NewService(t.TempDir(), stubWebsites{})
	_, err := svc.Tail(context.Background(), logs.TailInput{Type: "debug"})
	require.Error(t, err)
}

func TestLogs_Tail_DefaultTailLimit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var content string
	for i := 0; i < 20; i++ {
		content += "line\n"
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "access.log"), []byte(content), 0o644))
	svc := logs.NewService(dir, stubWebsites{})
	result, err := svc.Tail(context.Background(), logs.TailInput{Type: "access", Tail: 0})
	require.NoError(t, err)
	assert.Greater(t, result.LineCount, 0)
}

func TestLogs_Tail_CapsAtMaxLines(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "access.log"), []byte("x\n"), 0o644))
	svc := logs.NewService(dir, stubWebsites{})
	result, err := svc.Tail(context.Background(), logs.TailInput{Type: "access", Tail: 50000})
	require.NoError(t, err)
	assert.Equal(t, 1, result.LineCount)
	assert.Len(t, result.Lines, 1)
}

func TestLogs_ListSites_RepoError(t *testing.T) {
	t.Parallel()
	svc := logs.NewService(t.TempDir(), stubWebsites{err: assert.AnError})
	_, err := svc.ListSites(context.Background())
	require.Error(t, err)
}

func TestLogs_Tail_SanitizesDomainPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "access-evil.log"), []byte("safe\n"), 0o644))
	svc := logs.NewService(dir, stubWebsites{})
	result, err := svc.Tail(context.Background(), logs.TailInput{
		Domain: "../evil",
		Type:   "access",
	})
	require.NoError(t, err)
	assert.NotContains(t, result.Path, "..")
}

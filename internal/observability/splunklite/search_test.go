package splunklite

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSearch_Empty(t *testing.T) {
	t.Parallel()
	expr, pipes, err := ParseSearch("   ")
	require.NoError(t, err)
	assert.Nil(t, expr)
	assert.Nil(t, pipes)
}

func TestParseSearch_FieldEq(t *testing.T) {
	t.Parallel()
	expr, _, err := ParseSearch("status=404 action=login")
	require.NoError(t, err)
	and, ok := expr.(*AndExpr)
	require.True(t, ok)
	require.Len(t, and.Children, 2)
}

func TestParseSearch_ComparisonAndOr(t *testing.T) {
	t.Parallel()
	expr, _, err := ParseSearch(`status>=399 AND (curl OR status=200)`)
	require.NoError(t, err)
	and, ok := expr.(*AndExpr)
	require.True(t, ok)
	require.Len(t, and.Children, 2)
	or, ok := and.Children[1].(*OrExpr)
	require.True(t, ok)
	require.Len(t, or.Children, 2)
}

func TestParseSearch_Pipes(t *testing.T) {
	t.Parallel()
	_, pipes, err := ParseSearch("status=5* | head 50 | sort -ts")
	require.NoError(t, err)
	require.Len(t, pipes, 2)
	assert.Equal(t, "head", pipes[0].Name)
	assert.Equal(t, 50, pipes[0].Limit)
	assert.Equal(t, "sort", pipes[1].Name)
	assert.Equal(t, "ts", pipes[1].Field)
	assert.True(t, pipes[1].Desc)
}

func TestParseSearch_Regex(t *testing.T) {
	t.Parallel()
	expr, _, err := ParseSearch(`/timeout|500/`)
	require.NoError(t, err)
	pred, ok := expr.(*PredExpr)
	require.True(t, ok)
	assert.Equal(t, OpRegexp, pred.Op)
}

func TestParseSearch_InvalidRegex(t *testing.T) {
	t.Parallel()
	_, _, err := ParseSearch(`/[invalid/`)
	require.Error(t, err)
}

func TestParseSearch_FieldRegexSplit(t *testing.T) {
	t.Parallel()
	expr, _, err := ParseSearch(`action=/^website\..*/`)
	require.NoError(t, err)
	pred, ok := expr.(*PredExpr)
	require.True(t, ok)
	assert.Equal(t, "action", pred.Field)
	assert.Equal(t, OpRegexp, pred.Op)
	assert.Equal(t, `^website\..*`, pred.Value)

	expr2, _, err := ParseSearch(`status=/^3\d{2}$/`)
	require.NoError(t, err)
	pred2, ok := expr2.(*PredExpr)
	require.True(t, ok)
	assert.Equal(t, "status", pred2.Field)
	assert.Equal(t, `^3\d{2}$`, pred2.Value)
}

func TestCompileFilter_Access3xxRange(t *testing.T) {
	t.Parallel()
	expr, _, err := ParseSearch("status>=300 status<400")
	require.NoError(t, err)
	sql, err := CompileFilter(expr, SourceKindLogAccess)
	require.NoError(t, err)
	require.True(t, sql.Applicable)
	require.NotEmpty(t, sql.Wheres)
}

func TestCompileFilter_ErrorSkipsStatusCompare(t *testing.T) {
	t.Parallel()
	expr, _, err := ParseSearch("status>=399")
	require.NoError(t, err)
	sql, err := CompileFilter(expr, SourceKindLogError)
	require.NoError(t, err)
	assert.False(t, sql.Applicable)
}

func TestApplyPipes_HeadSort(t *testing.T) {
	t.Parallel()
	events := []QueryEvent{
		{TS: mustTime("2026-01-01T10:00:00Z"), Message: "a"},
		{TS: mustTime("2026-01-01T12:00:00Z"), Message: "b"},
		{TS: mustTime("2026-01-01T11:00:00Z"), Message: "c"},
	}
	out := ApplyPipes(events, []PipeCmd{{Name: "head", Limit: 2}, {Name: "sort", Field: "ts", Desc: true}})
	require.Len(t, out, 2)
	assert.Equal(t, "b", out[0].Message)
}

func TestEvalFilter_CanonicalAccess(t *testing.T) {
	t.Parallel()
	expr, _, err := ParseSearch(`status>=399 AND (curl OR status=200)`)
	require.NoError(t, err)

	matchHighCurl := QueryEvent{Source: "access", Message: "curl/7.68", Meta: map[string]any{"status_code": 500}}
	matchHighNoCurl := QueryEvent{Source: "access", Message: "GET /", Meta: map[string]any{"status_code": 404}}
	matchLow200 := QueryEvent{Source: "access", Message: "curl/7.68", Meta: map[string]any{"status_code": 200}}
	noMatch := QueryEvent{Source: "access", Message: "GET /", Meta: map[string]any{"status_code": 200}}

	assert.True(t, EvalFilter(expr, matchHighCurl, SourceKindLogAccess))
	assert.False(t, EvalFilter(expr, matchHighNoCurl, SourceKindLogAccess))
	assert.False(t, EvalFilter(expr, matchLow200, SourceKindLogAccess))
	assert.False(t, EvalFilter(expr, noMatch, SourceKindLogAccess))
}

func mustTime(s string) (t time.Time) {
	t, _ = time.Parse(time.RFC3339, s)
	return t
}

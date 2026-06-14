package contract_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func examplesDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "api", "examples"))
}

func TestContractExamples_Exist(t *testing.T) {
	dir := examplesDir(t)
	for _, name := range []string{
		"auth-login-metadata.json",
		"auth-login-response.json",
		"dashboard.json",
		"logs-tail.json",
		"websites-list.json",
		"saved-queries.json",
		"traffic-series.json",
	} {
		path := filepath.Join(dir, name)
		_, err := os.Stat(path)
		assert.NoError(t, err, "missing example %s", name)
	}
}

func TestContractExamples_ValidJSON(t *testing.T) {
	dir := examplesDir(t)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		require.NoError(t, err, entry.Name())
		var v any
		require.NoError(t, json.Unmarshal(data, &v), entry.Name())
	}
}

func TestContract_LogsTail_HasLinesArray(t *testing.T) {
	data := readExample(t, "logs-tail.json")
	var body struct {
		Lines []string `json:"lines"`
	}
	require.NoError(t, json.Unmarshal(data, &body))
	assert.NotEmpty(t, body.Lines)
}

func TestContract_SavedQueries_SnakeCase(t *testing.T) {
	data := readExample(t, "saved-queries.json")
	var body struct {
		Queries []struct {
			ID        int64  `json:"id"`
			CreatedAt string `json:"created_at"`
			Query     string `json:"query"`
		} `json:"queries"`
	}
	require.NoError(t, json.Unmarshal(data, &body))
	require.Len(t, body.Queries, 1)
	assert.Equal(t, "action=login", body.Queries[0].Query)
	assert.NotEmpty(t, body.Queries[0].CreatedAt)
}

func TestContract_Dashboard_HasRequiredSections(t *testing.T) {
	data := readExample(t, "dashboard.json")
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &body))
	for _, key := range []string{"system", "traffic_summary", "ssl_expiring", "recent_audit"} {
		_, ok := body[key]
		assert.True(t, ok, "dashboard missing %s", key)
	}
}

func readExample(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(examplesDir(t), name))
	require.NoError(t, err)
	return data
}

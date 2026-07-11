package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolsForScopes_WithCronAndPlugins(t *testing.T) {
	all := []string{"system:read", "websites:read", "nginx:read", "docker:read", "cron:read", "plugins:read"}
	tools := toolsForScopes(all)
	require.Len(t, tools, 6)

	byName := make(map[string]toolDef)
	for _, tool := range tools {
		byName[tool.Name] = tool
	}

	assert.Equal(t, "cron:read", byName["jobs"].Scope)
	assert.Equal(t, "plugins:read", byName["plugins"].Scope)

	assert.Len(t, toolsForScopes([]string{"cron:read"}), 1, "cron:read should expose the jobs tool")
	assert.Len(t, toolsForScopes([]string{"plugins:read"}), 1, "plugins:read should expose the plugins tool")
	assert.Len(t, toolsForScopes([]string{"jobs:read"}), 0, "legacy jobs:read should not expose any tool")
}

func TestServe_InitializeAndToolsList(t *testing.T) {
	client := &gositeClient{baseURL: "http://localhost", token: "tok", http: http.DefaultClient}
	s := &server{client: client, tools: toolsForScopes([]string{"system:read"})}

	in := strings.NewReader(`
{"jsonrpc":"2.0","id":1,"method":"initialize"}
{"jsonrpc":"2.0","id":2,"method":"tools/list"}
{"jsonrpc":"2.0","method":"notifications/initialized"}
`)
	out := &bytes.Buffer{}

	require.NoError(t, s.serve(in, out))

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	require.Len(t, lines, 2)

	var initResp, listResp rpcResponse
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &initResp))
	require.Equal(t, float64(1), initResp.ID)
	require.NotNil(t, initResp.Result)

	require.NoError(t, json.Unmarshal([]byte(lines[1]), &listResp))
	require.Equal(t, float64(2), listResp.ID)
	result, ok := listResp.Result.(map[string]any)
	require.True(t, ok)
	tools, ok := result["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)
}

func TestDispatch_ToolsCall(t *testing.T) {
	called := make(map[string]int)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called[r.URL.Path]++
		assert.Equal(t, "tok", r.Header.Get("X-Gosite-Access-Token"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer ts.Close()

	client := &gositeClient{baseURL: ts.URL, token: "tok", http: ts.Client()}
	allScopes := []string{"system:read", "websites:read", "nginx:read", "docker:read", "cron:read", "plugins:read"}
	s := &server{client: client, tools: toolsForScopes(allScopes)}

	cases := []struct {
		name string
		path string
	}{
		{"system", "/api/v1/system/info"},
		{"websites", "/api/v1/websites"},
		{"nginx", "/api/v1/nginx/test"},
		{"docker", "/api/v1/docker/containers"},
		{"jobs", "/api/v1/cronjobs"},
		{"plugins", "/api/v1/plugins"},
	}

	for _, tc := range cases {
		req := rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`42`), Method: "tools/call", Params: json.RawMessage(fmt.Sprintf(`{"name":"%s"}`, tc.name))}
		resp := s.dispatch(req)
		require.Nil(t, resp.Error, "tool %s returned error: %v", tc.name, resp.Error)
		require.NotNil(t, resp.Result)
		require.Equal(t, 1, called[tc.path], "tool %s did not call %s", tc.name, tc.path)
	}

	unknown := rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`99`), Method: "tools/call", Params: json.RawMessage(`{"name":"unknown"}`)}
	resp := s.dispatch(unknown)
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32000, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "unknown tool")
}

func TestCallTool_HandlesHostErrors(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, "bad token")
	}))
	defer ts.Close()

	client := &gositeClient{baseURL: ts.URL, token: "tok", http: ts.Client()}
	s := &server{client: client, tools: toolsForScopes([]string{"cron:read"})}

	resp := s.dispatch(rpcRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: "tools/call", Params: json.RawMessage(`{"name":"jobs"}`)})
	require.NotNil(t, resp.Error)
	assert.Contains(t, resp.Error.Message, "401")
	assert.Contains(t, resp.Error.Message, "bad token")
}

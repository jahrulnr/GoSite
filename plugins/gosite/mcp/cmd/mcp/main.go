// MCP stdio server for GoSite — experimental pre-GA transport.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
	skip    bool        `json:"-"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type server struct {
	client *gositeClient
	tools  []toolDef
}

type toolDef struct {
	Name        string
	Description string
	Scope       string
}

func main() {
	client, scopes, err := newGositeClientFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gosite-mcp: %v\n", err)
		os.Exit(1)
	}
	s := &server{client: client, tools: toolsForScopes(scopes)}
	if err := s.serve(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "gosite-mcp: %v\n", err)
		os.Exit(1)
	}
}

func (s *server) serve(in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	enc := json.NewEncoder(out)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			continue
		}
		resp := s.dispatch(req)
		if resp.skip {
			continue
		}
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (s *server) dispatch(req rpcRequest) rpcResponse {
	id := parseID(req.ID)
	switch req.Method {
	case "initialize":
		return rpcResponse{JSONRPC: "2.0", ID: id, Result: map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]string{"name": "gosite-mcp", "version": "0.2.0"},
		}}
	case "notifications/initialized", "initialized":
		if id == nil {
			return rpcResponse{skip: true}
		}
		return rpcResponse{JSONRPC: "2.0", ID: id}
	case "tools/list":
		items := make([]map[string]any, 0, len(s.tools))
		for _, tool := range s.tools {
			items = append(items, map[string]any{
				"name":        tool.Name,
				"description": tool.Description,
				"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
			})
		}
		return rpcResponse{JSONRPC: "2.0", ID: id, Result: map[string]any{"tools": items}}
	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		_ = json.Unmarshal(req.Params, &params)
		body, err := s.callTool(params.Name)
		if err != nil {
			return rpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: -32000, Message: err.Error()}}
		}
		return rpcResponse{JSONRPC: "2.0", ID: id, Result: map[string]any{
			"content": []map[string]string{{"type": "text", "text": body}},
		}}
	default:
		return rpcResponse{JSONRPC: "2.0", ID: id, Result: map[string]any{}}
	}
}

func (s *server) callTool(name string) (string, error) {
	switch name {
	case "system":
		return s.client.get("/api/v1/system/info")
	case "websites":
		return s.client.get("/api/v1/websites")
	case "nginx":
		return s.client.post("/api/v1/nginx/test", "{}")
	case "docker":
		return s.client.get("/api/v1/docker/containers")
	case "jobs":
		return s.client.get("/api/v1/cronjobs")
	case "plugins":
		return s.client.get("/api/v1/plugins")
	default:
		return "", fmt.Errorf("unknown tool %q", name)
	}
}

func toolsForScopes(scopes []string) []toolDef {
	has := func(scope string) bool {
		for _, s := range scopes {
			if s == scope {
				return true
			}
		}
		return false
	}
	var out []toolDef
	if has("system:read") {
		out = append(out, toolDef{Name: "system", Description: "GoSite system info", Scope: "system:read"})
	}
	if has("websites:read") {
		out = append(out, toolDef{Name: "websites", Description: "List websites", Scope: "websites:read"})
	}
	if has("nginx:read") {
		out = append(out, toolDef{Name: "nginx", Description: "Test nginx config", Scope: "nginx:read"})
	}
	if has("docker:read") || has("docker:manage") {
		out = append(out, toolDef{Name: "docker", Description: "List docker containers", Scope: "docker:read"})
	}
	if has("cron:read") {
		out = append(out, toolDef{Name: "jobs", Description: "List cron jobs", Scope: "cron:read"})
	}
	if has("plugins:read") {
		out = append(out, toolDef{Name: "plugins", Description: "List installed plugins", Scope: "plugins:read"})
	}
	return out
}

type gositeClient struct {
	baseURL string
	token   string
	basic   string
	http    *http.Client
}

func newGositeClientFromEnv() (*gositeClient, []string, error) {
	if err := validateMCPClientEnv(); err != nil {
		return nil, nil, err
	}
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("GOSITE_URL")), "/")
	token := strings.TrimSpace(os.Getenv("GOSITE_ACCESS_TOKEN"))
	if base == "" || token == "" {
		return nil, nil, fmt.Errorf("GOSITE_URL and GOSITE_ACCESS_TOKEN are required")
	}
	user := strings.TrimSpace(os.Getenv("GOSITE_BASIC_USER"))
	pass := os.Getenv("GOSITE_BASIC_PASS")
	var basic string
	if user != "" {
		req, _ := http.NewRequest(http.MethodGet, base+"/api/v1/integration-tokens/self", nil)
		req.SetBasicAuth(user, pass)
		basic = req.Header.Get("Authorization")
	}
	client := &gositeClient{
		baseURL: base,
		token:   token,
		basic:   basic,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
	scopes, err := client.introspect()
	return client, scopes, err
}

func (c *gositeClient) introspect() ([]string, error) {
	body, err := c.get("/api/v1/integration-tokens/self")
	if err != nil {
		return nil, err
	}
	var payload struct {
		Scopes []string `json:"scopes"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return nil, err
	}
	return payload.Scopes, nil
}

func validateMCPClientEnv() error {
	if email := strings.TrimSpace(os.Getenv("GOSITE_EMAIL")); email != "" {
		return fmt.Errorf("GOSITE_EMAIL is not supported; use GOSITE_ACCESS_TOKEN")
	}
	if pass := os.Getenv("GOSITE_PASSWORD"); strings.TrimSpace(pass) != "" {
		return fmt.Errorf("GOSITE_PASSWORD is not supported; use GOSITE_ACCESS_TOKEN")
	}
	if !envTruthy("GOSITE_INSECURE_SESSION") {
		return nil
	}
	env := strings.ToLower(strings.TrimSpace(firstNonEmpty(os.Getenv("GOSITE_ENV"), os.Getenv("APP_ENV"))))
	if env == "production" {
		return fmt.Errorf("GOSITE_INSECURE_SESSION is not allowed when GOSITE_ENV=production")
	}
	return nil
}

func envTruthy(key string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return v == "1" || v == "true" || v == "yes"
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func (c *gositeClient) get(path string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return "", err
	}
	c.applyHeaders(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		if resp.StatusCode == 403 || resp.StatusCode == 401 {
			return "", fmt.Errorf("host returned %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
		}
		return "", fmt.Errorf("host returned %d", resp.StatusCode)
	}
	return string(raw), nil
}

func (c *gositeClient) post(path, payload string) (string, error) {
	req, err := http.NewRequest(http.MethodPost, c.baseURL+path, bytes.NewBufferString(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	c.applyHeaders(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("host returned %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return string(raw), nil
}

func (c *gositeClient) applyHeaders(req *http.Request) {
	if c.basic != "" {
		req.Header.Set("Authorization", c.basic)
	}
	req.Header.Set("X-Gosite-Access-Token", c.token)
}

func parseID(raw json.RawMessage) interface{} {
	if len(raw) == 0 {
		return nil
	}
	var id interface{}
	_ = json.Unmarshal(raw, &id)
	return id
}

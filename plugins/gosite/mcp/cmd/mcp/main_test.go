package main

import "testing"

func TestToolsForScopes(t *testing.T) {
	scopes := []string{"system:read", "websites:read", "jobs:read"}
	tools := toolsForScopes(scopes)
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}
}

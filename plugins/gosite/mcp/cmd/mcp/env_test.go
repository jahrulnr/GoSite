package main

import "testing"

func TestValidateMCPClientEnv_RejectsDeprecatedCredentials(t *testing.T) {
	t.Setenv("GOSITE_EMAIL", "admin@example.com")
	t.Setenv("GOSITE_PASSWORD", "")
	t.Setenv("GOSITE_INSECURE_SESSION", "")
	t.Setenv("GOSITE_ENV", "")
	t.Setenv("APP_ENV", "local")

	if err := validateMCPClientEnv(); err == nil {
		t.Fatal("expected error for GOSITE_EMAIL")
	}
}

func TestValidateMCPClientEnv_RejectsInsecureSessionInProduction(t *testing.T) {
	t.Setenv("GOSITE_EMAIL", "")
	t.Setenv("GOSITE_PASSWORD", "")
	t.Setenv("GOSITE_INSECURE_SESSION", "1")
	t.Setenv("GOSITE_ENV", "production")
	t.Setenv("APP_ENV", "")

	if err := validateMCPClientEnv(); err == nil {
		t.Fatal("expected error for insecure session in production")
	}
}

func TestValidateMCPClientEnv_AllowsInsecureSessionInLocal(t *testing.T) {
	t.Setenv("GOSITE_EMAIL", "")
	t.Setenv("GOSITE_PASSWORD", "")
	t.Setenv("GOSITE_INSECURE_SESSION", "1")
	t.Setenv("GOSITE_ENV", "local")
	t.Setenv("APP_ENV", "")

	if err := validateMCPClientEnv(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

package testutil

import (
	"context"
	"fmt"
	"sync"
)

// MockNginx records nginx operations for assertions in tests.
type MockNginx struct {
	mu sync.Mutex

	TestCalls   int
	ReloadCalls int

	SiteConfigs map[string]string
	Backups     []string

	TestErr   error
	ReloadErr error
	WriteErr  error
	ReadErr   error
	BackupErr error
}

// NewMockNginx returns an empty nginx mock.
func NewMockNginx() *MockNginx {
	return &MockNginx{SiteConfigs: make(map[string]string)}
}

func (m *MockNginx) Test(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TestCalls++
	return m.TestErr
}

func (m *MockNginx) Reload(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ReloadCalls++
	return m.ReloadErr
}

func (m *MockNginx) WriteSiteConfig(ctx context.Context, domain, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.WriteErr != nil {
		return m.WriteErr
	}
	m.SiteConfigs[domain] = content
	return nil
}

func (m *MockNginx) ReadSiteConfig(ctx context.Context, domain string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ReadErr != nil {
		return "", m.ReadErr
	}
	content, ok := m.SiteConfigs[domain]
	if !ok {
		return "", fmt.Errorf("config not found for %s", domain)
	}
	return content, nil
}

func (m *MockNginx) BackupSiteConfig(ctx context.Context, domain string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.BackupErr != nil {
		return "", m.BackupErr
	}
	path := fmt.Sprintf("/tmp/%s.conf.bak", domain)
	m.Backups = append(m.Backups, path)
	return path, nil
}

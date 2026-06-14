package testutil

import (
	"context"
	"fmt"
	"sync"

	"github.com/jahrulnr/gosite/internal/contracts"
)

// MockDocker records docker client operations for assertions in tests.
type MockDocker struct {
	mu sync.Mutex

	Containers []contracts.ContainerSummary

	ListErr    error
	RestartErr error
	StopErr    error
	LogsErr    error
}

// NewMockDocker returns an empty docker client mock.
func NewMockDocker() *MockDocker {
	return &MockDocker{}
}

func (m *MockDocker) ListContainers(ctx context.Context) ([]contracts.ContainerSummary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ListErr != nil {
		return nil, m.ListErr
	}
	return append([]contracts.ContainerSummary(nil), m.Containers...), nil
}

func (m *MockDocker) RestartContainer(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.RestartErr != nil {
		return m.RestartErr
	}
	for i, c := range m.Containers {
		if c.ID == id {
			m.Containers[i].Status = "restarting"
			return nil
		}
	}
	return fmt.Errorf("container %s not found", id)
}

func (m *MockDocker) StopContainer(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.StopErr != nil {
		return m.StopErr
	}
	for i, c := range m.Containers {
		if c.ID == id {
			m.Containers[i].Status = "exited"
			return nil
		}
	}
	return fmt.Errorf("container %s not found", id)
}

func (m *MockDocker) ContainerLogs(ctx context.Context, id string, tail int) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.LogsErr != nil {
		return "", m.LogsErr
	}
	return fmt.Sprintf("logs for %s tail=%d", id, tail), nil
}

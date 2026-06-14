package docker

import (
	"context"

	"github.com/jahrulnr/gosite/internal/contracts"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// NoopClient is used when the Docker socket is unavailable.
type NoopClient struct{}

// ListContainers reports docker unavailable.
func (NoopClient) ListContainers(context.Context) ([]contracts.ContainerSummary, error) {
	return nil, apperror.New(apperror.CodeInternal, "docker socket unavailable")
}

// RestartContainer reports docker unavailable.
func (NoopClient) RestartContainer(context.Context, string) error {
	return apperror.New(apperror.CodeInternal, "docker socket unavailable")
}

// StopContainer reports docker unavailable.
func (NoopClient) StopContainer(context.Context, string) error {
	return apperror.New(apperror.CodeInternal, "docker socket unavailable")
}

// ContainerLogs reports docker unavailable.
func (NoopClient) ContainerLogs(context.Context, string, int) (string, error) {
	return "", apperror.New(apperror.CodeInternal, "docker socket unavailable")
}

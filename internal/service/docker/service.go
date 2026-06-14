package docker

import (
	"context"

	"github.com/jahrulnr/gosite/internal/contracts"
	dockerinfra "github.com/jahrulnr/gosite/internal/infra/docker"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// Service manages Docker container operations.
type Service struct {
	client contracts.DockerClient
}

// NewService returns a docker service.
func NewService(client contracts.DockerClient) *Service {
	return &Service{client: client}
}

// List returns all containers.
func (s *Service) List(ctx context.Context) ([]contracts.ContainerSummary, error) {
	rows, err := s.client.ListContainers(ctx)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeInternal, "list containers", err)
	}
	return rows, nil
}

// Restart restarts a container.
func (s *Service) Restart(ctx context.Context, id string) error {
	if _, err := dockerinfra.SanitizeContainerID(id); err != nil {
		return err
	}
	if err := s.client.RestartContainer(ctx, id); err != nil {
		return apperror.From(err)
	}
	return nil
}

// Stop stops a container.
func (s *Service) Stop(ctx context.Context, id string) error {
	if _, err := dockerinfra.SanitizeContainerID(id); err != nil {
		return err
	}
	if err := s.client.StopContainer(ctx, id); err != nil {
		return apperror.From(err)
	}
	return nil
}

// Logs returns recent container logs.
func (s *Service) Logs(ctx context.Context, id string, tail int) (string, error) {
	if _, err := dockerinfra.SanitizeContainerID(id); err != nil {
		return "", err
	}
	if tail <= 0 {
		tail = 200
	}
	logs, err := s.client.ContainerLogs(ctx, id, tail)
	if err != nil {
		return "", apperror.From(err)
	}
	return logs, nil
}

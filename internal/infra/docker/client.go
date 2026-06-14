package docker

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/jahrulnr/gosite/internal/contracts"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

var containerIDPattern = regexp.MustCompile(`^[a-zA-Z0-9-]+$`)

// Client wraps the Docker Engine API.
type Client struct {
	api *client.Client
}

// NewClient dials the Docker socket at host (empty uses default from env).
func NewClient(host string) (*Client, error) {
	opts := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}
	if host != "" {
		opts = append(opts, client.WithHost(host))
	}
	api, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}
	return &Client{api: api}, nil
}

// Close releases the underlying client.
func (c *Client) Close() error {
	if c == nil || c.api == nil {
		return nil
	}
	return c.api.Close()
}

// ListContainers returns all containers on the host.
func (c *Client) ListContainers(ctx context.Context) ([]contracts.ContainerSummary, error) {
	rows, err := c.api.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}
	out := make([]contracts.ContainerSummary, 0, len(rows))
	for _, row := range rows {
		name := row.ID
		if len(row.Names) > 0 {
			name = strings.TrimPrefix(row.Names[0], "/")
		}
		out = append(out, contracts.ContainerSummary{
			ID:      row.ID,
			Name:    name,
			Image:   row.Image,
			Status:  row.Status,
			State:   containerState(row.Status),
			Created: fmt.Sprintf("%d", row.Created),
		})
	}
	return out, nil
}

// RestartContainer restarts a container by id or name.
func (c *Client) RestartContainer(ctx context.Context, id string) error {
	id, err := SanitizeContainerID(id)
	if err != nil {
		return err
	}
	timeout := 10
	if err := c.api.ContainerRestart(ctx, id, container.StopOptions{Timeout: &timeout}); err != nil {
		return apperror.Wrap(apperror.CodeDockerNotFound, "restart container", err)
	}
	return nil
}

// StopContainer stops a container by id or name.
func (c *Client) StopContainer(ctx context.Context, id string) error {
	id, err := SanitizeContainerID(id)
	if err != nil {
		return err
	}
	timeout := 10
	if err := c.api.ContainerStop(ctx, id, container.StopOptions{Timeout: &timeout}); err != nil {
		return apperror.Wrap(apperror.CodeDockerNotFound, "stop container", err)
	}
	return nil
}

// ContainerLogs returns recent log lines for a container.
func (c *Client) ContainerLogs(ctx context.Context, id string, tail int) (string, error) {
	id, err := SanitizeContainerID(id)
	if err != nil {
		return "", err
	}
	if tail <= 0 {
		tail = 200
	}
	reader, err := c.api.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       fmt.Sprintf("%d", tail),
		Timestamps: false,
	})
	if err != nil {
		return "", apperror.Wrap(apperror.CodeDockerNotFound, "container logs", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read container logs: %w", err)
	}
	return StripDockerLogStream(data), nil
}

// SanitizeContainerID validates container identifiers.
func SanitizeContainerID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", apperror.New(apperror.CodeInvalidInput, "container id required")
	}
	if !containerIDPattern.MatchString(id) {
		return "", apperror.New(apperror.CodeInvalidInput, "invalid container id")
	}
	return id, nil
}

// StripDockerLogStream removes Docker multiplexed log framing when present.
func StripDockerLogStream(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	var b strings.Builder
	for i := 0; i+8 <= len(data); {
		size := int(data[i+4])<<24 | int(data[i+5])<<16 | int(data[i+6])<<8 | int(data[i+7])
		if size <= 0 || i+8+size > len(data) {
			break
		}
		b.Write(data[i+8 : i+8+size])
		i += 8 + size
	}
	if b.Len() == 0 {
		return string(data)
	}
	return b.String()
}

func containerState(status string) string {
	lower := strings.ToLower(status)
	switch {
	case strings.HasPrefix(lower, "up"), strings.Contains(lower, "running"):
		return "running"
	case strings.HasPrefix(lower, "exited"), strings.Contains(lower, "dead"):
		return "exited"
	case strings.Contains(lower, "paused"):
		return "paused"
	default:
		return "unknown"
	}
}

// Ping verifies connectivity to the Docker daemon.
func (c *Client) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := c.api.Ping(ctx)
	return err
}

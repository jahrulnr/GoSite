package contracts

import "context"

// ContainerSummary is a minimal container view for the panel API.
type ContainerSummary struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Image   string `json:"image"`
	Status  string `json:"status"`
	State   string `json:"state,omitempty"`
	Created string `json:"created,omitempty"`
}

// DockerClient abstracts Docker Engine operations.
type DockerClient interface {
	ListContainers(ctx context.Context) ([]ContainerSummary, error)
	RestartContainer(ctx context.Context, id string) error
	StopContainer(ctx context.Context, id string) error
	ContainerLogs(ctx context.Context, id string, tail int) (string, error)
}

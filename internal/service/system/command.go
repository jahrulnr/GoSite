package system

import (
	"context"

	infraContracts "github.com/jahrulnr/gosite/internal/contracts"
)

// CommandAdapter adapts infraContracts.CommandRunner to system.CommandRunner.
type CommandAdapter struct {
	Runner infraContracts.CommandRunner
}

// Run executes a command and returns stdout.
func (c CommandAdapter) Run(ctx context.Context, name string, args ...string) (string, error) {
	res, err := c.Runner.Run(ctx, name, args...)
	if err != nil {
		return "", err
	}
	return res.Stdout, nil
}

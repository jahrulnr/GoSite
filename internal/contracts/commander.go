package contracts

import (
	"context"
	"io"
)

// CommandResult captures stdout, stderr, and exit status from a command.
type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// CommandRunner executes OS commands behind an interface for testing.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (CommandResult, error)
	RunWithInput(ctx context.Context, stdin io.Reader, name string, args ...string) (CommandResult, error)
}

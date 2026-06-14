package commander

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/jahrulnr/gosite/internal/contracts"
)

// ExecRunner runs OS commands using os/exec.
type ExecRunner struct{}

// NewExecRunner returns a command runner backed by os/exec.
func NewExecRunner() *ExecRunner {
	return &ExecRunner{}
}

func (e *ExecRunner) Run(ctx context.Context, name string, args ...string) (contracts.CommandResult, error) {
	return run(ctx, nil, name, args...)
}

func (e *ExecRunner) RunWithInput(ctx context.Context, stdin io.Reader, name string, args ...string) (contracts.CommandResult, error) {
	return run(ctx, stdin, name, args...)
}

func run(ctx context.Context, stdin io.Reader, name string, args ...string) (contracts.CommandResult, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if stdin != nil {
		cmd.Stdin = stdin
	}
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return contracts.CommandResult{}, fmt.Errorf("run command: %w", err)
		}
	}
	return contracts.CommandResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}, nil
}

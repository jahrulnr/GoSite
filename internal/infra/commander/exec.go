package commander

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

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

// RunStreaming executes a command and invokes onChunk as stdout/stderr lines arrive.
func (e *ExecRunner) RunStreaming(ctx context.Context, name string, args []string, onChunk func(stream, chunk string)) (contracts.CommandResult, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return contracts.CommandResult{}, fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return contracts.CommandResult{}, fmt.Errorf("stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return contracts.CommandResult{}, fmt.Errorf("start command: %w", err)
	}

	var stdout, stderr strings.Builder
	var mu sync.Mutex
	var wg sync.WaitGroup
	consume := func(stream string, r io.Reader, out *strings.Builder) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)
		for scanner.Scan() {
			chunk := scanner.Text() + "\n"
			mu.Lock()
			out.WriteString(chunk)
			mu.Unlock()
			if onChunk != nil {
				onChunk(stream, chunk)
			}
		}
	}
	wg.Add(2)
	go consume("stdout", stdoutPipe, &stdout)
	go consume("stderr", stderrPipe, &stderr)

	wg.Wait()
	err = cmd.Wait()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return contracts.CommandResult{}, fmt.Errorf("run command: %w", err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	return contracts.CommandResult{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: exitCode}, nil
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

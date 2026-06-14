package testutil

import (
	"context"
	"io"
	"strings"
	"sync"

	"github.com/jahrulnr/gosite/internal/contracts"
)

// MockCommander records executed commands for assertions in tests.
type MockCommander struct {
	mu sync.Mutex

	Calls []CommandCall
	Err   error
	ReloadErr     error
	NginxTestFail bool

	// Stdout is returned for generic commands.
	Stdout string
}

// CommandCall captures one command invocation.
type CommandCall struct {
	Name string
	Args []string
}

// NewMockCommander returns an empty command runner mock.
func NewMockCommander() *MockCommander {
	return &MockCommander{Stdout: "ok"}
}

// SnapshotCalls returns a copy of recorded command calls.
func (m *MockCommander) SnapshotCalls() []CommandCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]CommandCall, len(m.Calls))
	copy(out, m.Calls)
	return out
}

func (m *MockCommander) Run(ctx context.Context, name string, args ...string) (contracts.CommandResult, error) {
	m.mu.Lock()
	m.Calls = append(m.Calls, CommandCall{Name: name, Args: append([]string(nil), args...)})
	err := m.Err
	reloadErr := m.ReloadErr
	nginxTestFail := m.NginxTestFail
	stdout := m.Stdout
	m.mu.Unlock()

	if err != nil {
		return contracts.CommandResult{}, err
	}
	if name == "nginx" && contains(args, "reload") && reloadErr != nil {
		return contracts.CommandResult{Stderr: reloadErr.Error(), ExitCode: 1}, reloadErr
	}
	out := stdout
	if name == "nginx" && contains(args, "-t") {
		if nginxTestFail {
			return contracts.CommandResult{Stderr: "syntax error", ExitCode: 1}, nil
		}
		out = "nginx: the configuration file syntax is ok\nnginx: configuration file test is successful"
	}
	return contracts.CommandResult{Stdout: out, ExitCode: 0}, nil
}

func (m *MockCommander) RunWithInput(ctx context.Context, stdin io.Reader, name string, args ...string) (contracts.CommandResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var input string
	if stdin != nil {
		b, _ := io.ReadAll(stdin)
		input = string(b)
	}
	m.Calls = append(m.Calls, CommandCall{Name: name, Args: append(append([]string{"<stdin>"}, args...), input)})
	if m.Err != nil {
		return contracts.CommandResult{}, m.Err
	}
	return contracts.CommandResult{Stdout: strings.TrimSpace(input), ExitCode: 0}, nil
}

func contains(args []string, needle string) bool {
	for _, a := range args {
		if a == needle {
			return true
		}
	}
	return false
}

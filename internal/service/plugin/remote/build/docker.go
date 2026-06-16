package build

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jahrulnr/gosite/internal/service/plugin/remote/failures"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// Spec is one Docker-backed plugin build (G2b).
type Spec struct {
	VCS       string
	Repo      string
	Tag       string
	GoVersion string
	Package   string
	Token     string
}

// Config tunes the builder sandbox.
type Config struct {
	Enabled   bool
	Image     string
	Timeout   time.Duration
	MemoryMB  int
	CPU       float64
	GitLabURL string
}

// Runner builds plugin zip artifacts in an isolated container.
type Runner interface {
	Build(ctx context.Context, spec Spec) (artifact []byte, sourceCommit string, err error)
}

// DockerRunner clones a git tag and runs go build inside Docker.
type DockerRunner struct {
	cfg Config
}

// NewDockerRunner returns a docker-backed builder.
func NewDockerRunner(cfg Config) *DockerRunner {
	return &DockerRunner{cfg: cfg}
}

// Build clones the repo tag and produces a zip with manifest.json + binary.
func (d *DockerRunner) Build(ctx context.Context, spec Spec) ([]byte, string, error) {
	if !d.cfg.Enabled {
		return nil, "", apperror.New(apperror.CodeInvalidInput, failures.BuildDisabled)
	}
	workDir, err := os.MkdirTemp("", "gosite-plugin-build-")
	if err != nil {
		return nil, "", apperror.Wrap(apperror.CodePluginOperation, failures.BuildFailed, err)
	}
	defer os.RemoveAll(workDir)

	cloneURL, err := gitCloneURL(spec)
	if err != nil {
		return nil, "", err
	}
	cloneCtx, cancel := context.WithTimeout(ctx, d.cfg.Timeout)
	defer cancel()
	clone := exec.CommandContext(cloneCtx, "git", "clone", "--depth", "1", "--branch", spec.Tag, cloneURL, filepath.Join(workDir, "src"))
	if spec.Token != "" && strings.EqualFold(spec.VCS, "gitlab") {
		clone.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	}
	if out, err := clone.CombinedOutput(); err != nil {
		return nil, "", apperror.Wrap(apperror.CodePluginOperation, failures.BuildFailed, fmt.Errorf("git clone: %s: %w", strings.TrimSpace(string(out)), err))
	}

	commit, _ := gitHeadCommit(cloneCtx, filepath.Join(workDir, "src"))

	image := strings.TrimSpace(d.cfg.Image)
	if image == "" {
		image = "golang:1.22-bookworm"
	}
	goVer := strings.TrimSpace(spec.GoVersion)
	if goVer != "" && !strings.HasPrefix(goVer, "golang:") {
		image = "golang:" + strings.TrimPrefix(goVer, "go")
	}
	pkg := strings.TrimSpace(spec.Package)
	if pkg == "" {
		pkg = "."
	}
	script := fmt.Sprintf(`set -e
cd /work/src
go build -o /work/plugin.bin %s
test -f manifest.json
zip -q /work/artifact.zip manifest.json plugin.bin
`, pkg)

	runCtx, runCancel := context.WithTimeout(ctx, d.cfg.Timeout)
	defer runCancel()
	args := []string{
		"run", "--rm",
		"-v", workDir + ":/work",
		"-w", "/work",
	}
	if d.cfg.MemoryMB > 0 {
		args = append(args, "--memory", fmt.Sprintf("%dm", d.cfg.MemoryMB))
	}
	if d.cfg.CPU > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%.2f", d.cfg.CPU))
	}
	args = append(args, image, "bash", "-lc", script)
	cmd := exec.CommandContext(runCtx, "docker", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, commit, apperror.Wrap(apperror.CodePluginOperation, failures.BuildFailed, fmt.Errorf("docker build: %s: %w", strings.TrimSpace(string(out)), err))
	}

	data, err := os.ReadFile(filepath.Join(workDir, "artifact.zip"))
	if err != nil {
		return nil, commit, apperror.Wrap(apperror.CodePluginOperation, failures.BuildFailed, err)
	}
	return data, commit, nil
}

func gitCloneURL(spec Spec) (string, error) {
	repo := strings.TrimSpace(spec.Repo)
	if repo == "" {
		return "", apperror.New(apperror.CodeInvalidInput, failures.ResolveFailed)
	}
	switch strings.ToLower(strings.TrimSpace(spec.VCS)) {
	case "gitlab":
		base := "https://gitlab.com"
		if strings.Contains(repo, "://") {
			return repo, nil
		}
		return fmt.Sprintf("%s/%s.git", base, repo), nil
	default:
		if strings.Contains(repo, "://") {
			return repo, nil
		}
		return fmt.Sprintf("https://github.com/%s.git", repo), nil
	}
}

func gitHeadCommit(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "HEAD")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

// NopRunner rejects build requests — used when G2b is disabled.
type NopRunner struct{}

func (NopRunner) Build(context.Context, Spec) ([]byte, string, error) {
	return nil, "", apperror.New(apperror.CodeInvalidInput, failures.BuildDisabled)
}

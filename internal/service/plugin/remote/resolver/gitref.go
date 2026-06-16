package resolver

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jahrulnr/gosite/internal/service/plugin/remote/failures"
	"github.com/jahrulnr/gosite/internal/service/plugin/remote/types"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// GitRefResolver resolves tier-0 plugins from raw manifest.json at a git tag (G5).
type GitRefResolver struct {
	GitHub GitHubResolver
	GitLab GitLabResolver
}

func (g GitRefResolver) Supports(source types.Source) bool {
	return strings.EqualFold(strings.TrimSpace(source.Type), "git-ref")
}

func (g GitRefResolver) Resolve(ctx context.Context, source types.Source) (types.FetchPlan, types.ResolvePreview, error) {
	repo := strings.TrimSpace(source.Repo)
	tag := strings.TrimSpace(source.Tag)
	if repo == "" || tag == "" {
		return types.FetchPlan{}, types.ResolvePreview{}, apperror.New(apperror.CodeInvalidInput, failures.ResolveFailed)
	}
	vcs := strings.ToLower(strings.TrimSpace(source.URL))
	if vcs == "" {
		vcs = "github"
	}

	var manifestJSON []byte
	var err error
	switch vcs {
	case "gitlab":
		parts := strings.Split(repo, "/")
		if len(parts) != 2 {
			return types.FetchPlan{}, types.ResolvePreview{}, apperror.New(apperror.CodeInvalidInput, "repo must be group/name")
		}
		manifestJSON, err = g.GitLab.fetchRaw(ctx, parts[0], parts[1], tag, "manifest.json")
	default:
		parts := strings.Split(repo, "/")
		if len(parts) != 2 {
			return types.FetchPlan{}, types.ResolvePreview{}, apperror.New(apperror.CodeInvalidInput, "repo must be owner/name")
		}
		manifestJSON, err = g.GitHub.fetchRaw(ctx, parts[0], parts[1], tag, "manifest.json")
	}
	if err != nil {
		return types.FetchPlan{}, types.ResolvePreview{}, err
	}

	var manifest struct {
		ID          string   `json:"id"`
		Name        string   `json:"name"`
		Version     string   `json:"version"`
		Tier        int      `json:"tier"`
		Permissions []string `json:"permissions"`
		Capabilities struct {
			Hooks []string `json:"hooks"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(manifestJSON, &manifest); err != nil {
		return types.FetchPlan{}, types.ResolvePreview{}, apperror.Wrap(apperror.CodeInvalidInput, failures.ResolveFailed, err)
	}
	if manifest.Tier != 0 {
		return types.FetchPlan{}, types.ResolvePreview{}, apperror.New(apperror.CodeInvalidInput, "git-ref source requires tier 0 manifest")
	}
	if strings.TrimSpace(manifest.ID) == "" || strings.TrimSpace(manifest.Version) == "" {
		return types.FetchPlan{}, types.ResolvePreview{}, apperror.New(apperror.CodeInvalidInput, failures.ResolveFailed)
	}

	artifact, digest, err := zipManifest(manifestJSON)
	if err != nil {
		return types.FetchPlan{}, types.ResolvePreview{}, apperror.Wrap(apperror.CodePluginOperation, failures.ResolveFailed, err)
	}

	sourceRef := fmt.Sprintf("%s@%s", repo, tag)
	plan := types.FetchPlan{
		SHA256:           digest,
		SourceType:       "git-ref",
		SourceRef:        sourceRef,
		InstallPath:      "git-ref",
		ResolvedDigest:   digest,
		InlineArtifact:   artifact,
		SourceRepository: repo,
	}
	preview := types.ResolvePreview{
		PluginID:    manifest.ID,
		Version:     manifest.Version,
		Tier:        manifest.Tier,
		SHA256:      digest,
		SourceType:  "git-ref",
		SourceRef:   sourceRef,
		InstallPath: "git-ref",
		Permissions: manifest.Permissions,
		Hooks:       manifest.Capabilities.Hooks,
	}
	return plan, preview, nil
}

func zipManifest(manifestJSON []byte) ([]byte, string, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("manifest.json")
	if err != nil {
		return nil, "", err
	}
	if _, err := w.Write(manifestJSON); err != nil {
		return nil, "", err
	}
	if err := zw.Close(); err != nil {
		return nil, "", err
	}
	data := buf.Bytes()
	sum := sha256.Sum256(data)
	return data, hex.EncodeToString(sum[:]), nil
}

package resolver

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/jahrulnr/gosite/internal/service/plugin/remote/failures"
	"github.com/jahrulnr/gosite/internal/service/plugin/remote/index"
	"github.com/jahrulnr/gosite/internal/service/plugin/remote/types"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

type manifestHints struct {
	Permissions []string
	Hooks       []string
	Tier        int
}

type indexResolveInput struct {
	Idx          index.File
	Tag          string
	SourceType   string
	SourceRef    string
	BuildEnabled bool
	ForceBuild   bool
	BuildToken   string
	BuildVCS     string
	Hints        manifestHints
}

func resolveIndexRelease(in indexResolveInput) (types.FetchPlan, types.ResolvePreview, error) {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	asset, rel, err := in.Idx.SelectAsset(in.Tag, goos, goarch)
	if err == nil && !in.ForceBuild {
		plan := types.FetchPlan{
			URL:              asset.URL,
			SHA256:           strings.ToLower(asset.SHA256),
			SourceType:       in.SourceType,
			SourceRef:        in.SourceRef,
			InstallPath:      "release",
			ResolvedDigest:   strings.ToLower(asset.SHA256),
			SourceCommit:     rel.SourceCommit,
			SourceRepository: firstNonEmpty(rel.SourceRepository, in.Idx.Repository),
		}
		preview := previewFromRelease(in, rel, plan.SHA256, asset.URL, keyIDFromAsset(asset))
		return plan, preview, nil
	}

	buildSpec := in.Idx.BuildSpec()
	if !in.ForceBuild && (buildSpec == nil || !in.BuildEnabled) {
		if err != nil && strings.HasPrefix(err.Error(), "platform_unsupported:") {
			platforms := in.Idx.PlatformsForVersion(in.Tag)
			supported := make([]types.Platform, 0, len(platforms))
			for _, p := range platforms {
				supported = append(supported, types.Platform{OS: p.OS, Arch: p.Arch})
			}
			preview := types.ResolvePreview{
				PluginID:           in.Idx.ID,
				SourceType:         in.SourceType,
				SourceRef:          in.SourceRef,
				SupportedPlatforms: supported,
				SourceRepository:   in.Idx.Repository,
			}
			return types.FetchPlan{}, preview, apperror.New(apperror.CodeInvalidInput, failures.PlatformUnsupported)
		}
		if err != nil {
			return types.FetchPlan{}, types.ResolvePreview{}, apperror.Wrap(apperror.CodeInvalidInput, failures.ResolveFailed, err)
		}
	}

	if buildSpec == nil {
		return types.FetchPlan{}, types.ResolvePreview{}, apperror.New(apperror.CodeInvalidInput, failures.BuildDisabled)
	}
	if !in.BuildEnabled && !in.ForceBuild {
		return types.FetchPlan{}, types.ResolvePreview{}, apperror.New(apperror.CodeInvalidInput, failures.BuildDisabled)
	}

	pkg := strings.TrimSpace(buildSpec.Package)
	if pkg == "" {
		pkg = strings.TrimSpace(buildSpec.Entrypoint)
	}
	plan := types.FetchPlan{
		SourceType:       strings.TrimSuffix(in.SourceType, "-release") + "-build",
		SourceRef:        in.SourceRef,
		InstallPath:      "build",
		SourceRepository: in.Idx.Repository,
		Build: &types.BuildPlan{
			VCS:       in.BuildVCS,
			Repo:      repoFromSourceRef(in.SourceRef),
			Tag:       in.Tag,
			GoVersion: buildSpec.GoVersion,
			Package:   pkg,
			Token:     in.BuildToken,
		},
	}
	preview := types.ResolvePreview{
		PluginID:         in.Idx.ID,
		Version:          strings.TrimPrefix(in.Tag, "v"),
		Tier:             in.Hints.Tier,
		SourceType:       plan.SourceType,
		SourceRef:        in.SourceRef,
		InstallPath:      "build",
		SourceRepository: in.Idx.Repository,
		Permissions:      in.Hints.Permissions,
		Hooks:            in.Hints.Hooks,
	}
	if rel.Version != "" {
		preview.Version = strings.TrimPrefix(rel.Version, "v")
		preview.MinGoSiteVersion = rel.MinGoSiteVersion
		preview.SourceCommit = rel.SourceCommit
	}
	return plan, preview, nil
}

func previewFromRelease(in indexResolveInput, rel index.Release, sha256, url, keyID string) types.ResolvePreview {
	preview := types.ResolvePreview{
		PluginID:         in.Idx.ID,
		Version:          strings.TrimPrefix(rel.Version, "v"),
		Tier:             in.Hints.Tier,
		Signed:           keyID != "",
		SHA256:           sha256,
		URL:              url,
		MinGoSiteVersion: rel.MinGoSiteVersion,
		SourceType:       in.SourceType,
		SourceRef:        in.SourceRef,
		InstallPath:      "release",
		SourceCommit:     rel.SourceCommit,
		SourceRepository: firstNonEmpty(rel.SourceRepository, in.Idx.Repository),
		Permissions:      in.Hints.Permissions,
		Hooks:            in.Hints.Hooks,
		KeyID:            keyID,
	}
	return preview
}

func keyIDFromAsset(asset index.Asset) string {
	if len(asset.Signatures) > 0 {
		return asset.Signatures[0].KeyID
	}
	return ""
}

func repoFromSourceRef(sourceRef string) string {
	if i := strings.Index(sourceRef, "@"); i > 0 {
		return sourceRef[:i]
	}
	return sourceRef
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// GitHubBuildResolver forces Path B docker build for a GitHub tag.
type GitHubBuildResolver struct {
	GitHub       GitHubResolver
	BuildEnabled bool
}

func (g GitHubBuildResolver) Supports(source types.Source) bool {
	return strings.EqualFold(strings.TrimSpace(source.Type), "github-build")
}

func (g GitHubBuildResolver) Resolve(ctx context.Context, source types.Source) (types.FetchPlan, types.ResolvePreview, error) {
	return g.resolve(ctx, source, "github")
}

func (g GitHubBuildResolver) resolve(ctx context.Context, source types.Source, vcs string) (types.FetchPlan, types.ResolvePreview, error) {
	repo := strings.TrimSpace(source.Repo)
	tag := strings.TrimSpace(source.Tag)
	if repo == "" || tag == "" {
		return types.FetchPlan{}, types.ResolvePreview{}, apperror.New(apperror.CodeInvalidInput, failures.ResolveFailed)
	}
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return types.FetchPlan{}, types.ResolvePreview{}, apperror.New(apperror.CodeInvalidInput, "repo must be owner/name")
	}
	indexBytes, err := g.GitHub.fetchRaw(ctx, parts[0], parts[1], tag, "gosite.plugin.json")
	if err != nil {
		return types.FetchPlan{}, types.ResolvePreview{}, err
	}
	idx, err := index.Parse(indexBytes)
	if err != nil {
		return types.FetchPlan{}, types.ResolvePreview{}, apperror.Wrap(apperror.CodeInvalidInput, failures.ResolveFailed, err)
	}
	perms, hooks, tier := g.GitHub.loadManifestHints(ctx, parts[0], parts[1], tag)
	sourceRef := fmt.Sprintf("%s/%s@%s", parts[0], parts[1], tag)
	return resolveIndexRelease(indexResolveInput{
		Idx:          idx,
		Tag:          tag,
		SourceType:   vcs + "-build",
		SourceRef:    sourceRef,
		BuildEnabled: g.BuildEnabled,
		ForceBuild:   true,
		BuildToken:   g.GitHub.Token,
		BuildVCS:     vcs,
		Hints:        manifestHints{Permissions: perms, Hooks: hooks, Tier: tier},
	})
}

// GitLabBuildResolver forces Path B docker build for a GitLab tag.
type GitLabBuildResolver struct {
	GitLab       GitLabResolver
	BuildEnabled bool
}

func (g GitLabBuildResolver) Supports(source types.Source) bool {
	return strings.EqualFold(strings.TrimSpace(source.Type), "gitlab-build")
}

func (g GitLabBuildResolver) Resolve(ctx context.Context, source types.Source) (types.FetchPlan, types.ResolvePreview, error) {
	repo := strings.TrimSpace(source.Repo)
	tag := strings.TrimSpace(source.Tag)
	if repo == "" || tag == "" {
		return types.FetchPlan{}, types.ResolvePreview{}, apperror.New(apperror.CodeInvalidInput, failures.ResolveFailed)
	}
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return types.FetchPlan{}, types.ResolvePreview{}, apperror.New(apperror.CodeInvalidInput, "repo must be group/name")
	}
	indexBytes, err := g.GitLab.fetchRaw(ctx, parts[0], parts[1], tag, "gosite.plugin.json")
	if err != nil {
		return types.FetchPlan{}, types.ResolvePreview{}, err
	}
	idx, err := index.Parse(indexBytes)
	if err != nil {
		return types.FetchPlan{}, types.ResolvePreview{}, apperror.Wrap(apperror.CodeInvalidInput, failures.ResolveFailed, err)
	}
	perms, hooks, tier := g.GitLab.loadManifestHints(ctx, parts[0], parts[1], tag)
	sourceRef := fmt.Sprintf("%s/%s@%s", parts[0], parts[1], tag)
	return resolveIndexRelease(indexResolveInput{
		Idx:          idx,
		Tag:          tag,
		SourceType:   "gitlab-build",
		SourceRef:    sourceRef,
		BuildEnabled: g.BuildEnabled,
		ForceBuild:   true,
		BuildToken:   g.GitLab.Token,
		BuildVCS:     "gitlab",
		Hints:        manifestHints{Permissions: perms, Hooks: hooks, Tier: tier},
	})
}

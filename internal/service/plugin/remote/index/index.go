package index

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
)

// File is the gosite.plugin.json distribution index at a repo tag.
type File struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Repository   string       `json:"repository"`
	Distribution Distribution `json:"distribution"`
}

// Distribution holds release entries.
type Distribution struct {
	APIVersion string    `json:"apiVersion"`
	Releases   []Release `json:"releases"`
	Build      *Build    `json:"build,omitempty"`
}

// Build describes a host-side Docker build for Path B (G2b).
type Build struct {
	GoVersion  string `json:"goVersion"`
	Package    string `json:"package"`
	Entrypoint string `json:"entrypoint"`
}

// Release is one published version in the index.
type Release struct {
	Version          string  `json:"version"`
	MinGoSiteVersion string  `json:"minGoSiteVersion"`
	SourceCommit     string  `json:"sourceCommit"`
	SourceRepository string  `json:"sourceRepository"`
	BuildTime        string  `json:"buildTime"`
	Assets           []Asset `json:"assets"`
}

// Asset is one platform-specific zip in a release.
type Asset struct {
	Name       string `json:"name"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
	URL        string `json:"url"`
	SHA256     string `json:"sha256"`
	Signatures []struct {
		KeyID string `json:"keyId"`
		Sig   string `json:"sig"`
	} `json:"signatures"`
}

// Parse decodes gosite.plugin.json bytes.
func Parse(data []byte) (File, error) {
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return File{}, fmt.Errorf("parse gosite.plugin.json: %w", err)
	}
	if strings.TrimSpace(f.ID) == "" {
		return File{}, fmt.Errorf("gosite.plugin.json: id required")
	}
	return f, nil
}

// SelectAsset picks the asset for version and host platform.
func (f File) SelectAsset(version string, goos, goarch string) (Asset, Release, error) {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	var platforms []Platform
	for _, rel := range f.Distribution.Releases {
		relVer := strings.TrimPrefix(strings.TrimSpace(rel.Version), "v")
		if relVer != version {
			continue
		}
		for _, asset := range rel.Assets {
			platforms = append(platforms, Platform{OS: asset.OS, Arch: asset.Arch})
			if strings.EqualFold(asset.OS, goos) && strings.EqualFold(asset.Arch, goarch) {
				if strings.TrimSpace(asset.URL) == "" || strings.TrimSpace(asset.SHA256) == "" {
					return Asset{}, Release{}, fmt.Errorf("asset missing url or sha256")
				}
				return asset, rel, nil
			}
		}
		return Asset{}, Release{}, fmt.Errorf("platform_unsupported:%s", encodePlatforms(platforms))
	}
	return Asset{}, Release{}, fmt.Errorf("version %q not found in index", version)
}

// Platform is GOOS/GOARCH advertised in the index.
type Platform struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

func encodePlatforms(platforms []Platform) string {
	parts := make([]string, 0, len(platforms))
	for _, p := range platforms {
		parts = append(parts, p.OS+"/"+p.Arch)
	}
	return strings.Join(parts, ",")
}

// PlatformsForVersion lists GOOS/GOARCH pairs advertised for a release tag.
func (f File) PlatformsForVersion(version string) []Platform {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	for _, rel := range f.Distribution.Releases {
		relVer := strings.TrimPrefix(strings.TrimSpace(rel.Version), "v")
		if relVer != version {
			continue
		}
		out := make([]Platform, 0, len(rel.Assets))
		for _, asset := range rel.Assets {
			out = append(out, Platform{OS: asset.OS, Arch: asset.Arch})
		}
		return out
	}
	return nil
}

// BuildSpec returns the distribution build block when present.
func (f File) BuildSpec() *Build {
	if f.Distribution.Build == nil {
		return nil
	}
	cp := *f.Distribution.Build
	return &cp
}

// HostPlatform returns the current process GOOS/GOARCH.
func HostPlatform() (string, string) {
	return runtime.GOOS, runtime.GOARCH
}

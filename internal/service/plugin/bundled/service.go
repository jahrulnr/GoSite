package bundled

import (
	_ "embed"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

//go:embed index.json
var embeddedIndex []byte

const indexFileName = "bundled-plugins.json"

// Entry is one official plugin shipped with the GoSite release.
type Entry struct {
	PluginID          string `json:"plugin_id"`
	Artifact          string `json:"artifact"`
	PermissionsPreAck bool   `json:"permissions_pre_ack"`
	Restorable        bool   `json:"restorable"`
}

// Index is the bundled plugin catalog document.
type Index struct {
	APIVersion string  `json:"apiVersion"`
	Plugins    []Entry `json:"plugins"`
}

// Service loads bundled plugin metadata and artifact bytes from disk.
type Service struct {
	dir string
}

// NewService returns a bundled plugin loader. Empty dir uses only the embedded index;
// artifacts are read from dir when set.
func NewService(dir string) *Service {
	return &Service{dir: strings.TrimSpace(dir)}
}

// Enabled reports whether a bundled directory is configured or discoverable.
func (s *Service) Enabled() bool {
	if s.dir != "" {
		return true
	}
	if _, err := os.Stat("/app/bundled-plugins"); err == nil {
		return true
	}
	return false
}

// ResolveDir returns the directory used for artifact files.
func (s *Service) ResolveDir() string {
	if s.dir != "" {
		return s.dir
	}
	if info, err := os.Stat("/app/bundled-plugins"); err == nil && info.IsDir() {
		return "/app/bundled-plugins"
	}
	return ""
}

// LoadIndex returns the bundled plugin index.
func (s *Service) LoadIndex() (Index, error) {
	data := embeddedIndex
	if dir := s.ResolveDir(); dir != "" {
		path := filepath.Join(dir, indexFileName)
		if raw, err := os.ReadFile(path); err == nil {
			data = raw
		} else if !errors.Is(err, os.ErrNotExist) {
			return Index{}, err
		}
	}
	var index Index
	if err := json.Unmarshal(data, &index); err != nil {
		return Index{}, err
	}
	return index, nil
}

// Entry returns one bundled plugin entry by id.
func (s *Service) Entry(pluginID string) (Entry, error) {
	index, err := s.LoadIndex()
	if err != nil {
		return Entry{}, err
	}
	for _, entry := range index.Plugins {
		if entry.PluginID == pluginID {
			return entry, nil
		}
	}
	return Entry{}, ErrNotFound
}

// LoadArtifact reads the zip bytes for a bundled entry.
func (s *Service) LoadArtifact(entry Entry) ([]byte, error) {
	dir := s.ResolveDir()
	if dir == "" {
		return nil, ErrArtifactsUnavailable
	}
	path := filepath.Join(dir, entry.Artifact)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrArtifactsUnavailable
		}
		return nil, err
	}
	return data, nil
}

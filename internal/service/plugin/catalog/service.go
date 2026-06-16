package catalog

import (
	"context"
	_ "embed"
	"encoding/json"
	"os"
	"strings"

	"github.com/jahrulnr/gosite/internal/service/plugin/remote/types"
)

//go:embed catalog.json
var bundled []byte

// Entry is one curated catalog row (G4).
type Entry struct {
	PluginID    string       `json:"plugin_id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Vendor      string       `json:"vendor"`
	Source      types.Source `json:"source"`
}

// File is the catalog document.
type File struct {
	Entries []Entry `json:"entries"`
}

// Service reads the plugin catalog index.
type Service struct {
	path string
}

// NewService returns a catalog service. Empty path uses the bundled catalog.
func NewService(path string) *Service {
	return &Service{path: strings.TrimSpace(path)}
}

// List returns entries optionally filtered by query string.
func (s *Service) List(_ context.Context, query string) ([]Entry, error) {
	file, err := s.load()
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return file.Entries, nil
	}
	out := make([]Entry, 0, len(file.Entries))
	for _, entry := range file.Entries {
		if strings.Contains(strings.ToLower(entry.PluginID), q) ||
			strings.Contains(strings.ToLower(entry.Name), q) ||
			strings.Contains(strings.ToLower(entry.Description), q) ||
			strings.Contains(strings.ToLower(entry.Vendor), q) {
			out = append(out, entry)
		}
	}
	return out, nil
}

// Get returns one catalog entry by plugin id.
func (s *Service) Get(_ context.Context, pluginID string) (Entry, error) {
	file, err := s.load()
	if err != nil {
		return Entry{}, err
	}
	for _, entry := range file.Entries {
		if entry.PluginID == pluginID {
			return entry, nil
		}
	}
	return Entry{}, ErrNotFound
}

func (s *Service) load() (File, error) {
	data := bundled
	if s.path != "" {
		raw, err := os.ReadFile(s.path)
		if err != nil {
			return File{}, err
		}
		data = raw
	}
	var file File
	if err := json.Unmarshal(data, &file); err != nil {
		return File{}, err
	}
	return file, nil
}

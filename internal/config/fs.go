package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// FSOptions selects the files the adapter reads. It performs no Git discovery
// and never walks to a parent directory.
type FSOptions struct {
	RepoDir    string // required; cleaned to an absolute path, used verbatim (no Git discovery)
	GlobalPath string // test seam; "" → ${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml
}

// LoadFilesystemSources reads DIR/.docket.yml (repository), DIR/.docket.local.yml
// (repository-local), and the global config, returning present layers in
// low→high order. A missing file is an absent layer, not an error.
func LoadFilesystemSources(opts FSOptions) ([]Source, error) {
	if opts.RepoDir == "" {
		return nil, errors.New("config: FSOptions.RepoDir is required")
	}
	repoDir, err := filepath.Abs(opts.RepoDir)
	if err != nil {
		return nil, fmt.Errorf("config: resolving repository directory %q: %w", opts.RepoDir, err)
	}
	repoDir = filepath.Clean(repoDir)

	globalPath := opts.GlobalPath
	if globalPath == "" {
		globalPath = defaultGlobalPath()
	}

	type candidate struct {
		layer LayerKind
		name  string
		path  string
	}
	candidates := []candidate{
		{LayerRepository, ".docket.yml", filepath.Join(repoDir, ".docket.yml")},
		{LayerRepositoryLocal, ".docket.local.yml", filepath.Join(repoDir, ".docket.local.yml")},
	}
	if globalPath != "" {
		// Global is the lowest-precedence file layer, so it comes first.
		candidates = append([]candidate{{LayerGlobal, globalPath, globalPath}}, candidates...)
	}

	var sources []Source
	for _, c := range candidates {
		data, err := os.ReadFile(c.path)
		if err != nil {
			if os.IsNotExist(err) {
				continue // layer absent
			}
			return nil, fmt.Errorf("config: reading %s: %w", c.path, err)
		}
		sources = append(sources, Source{Layer: c.layer, Name: c.name, Data: data})
	}
	return sources, nil
}

// defaultGlobalPath is ${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml.
// It returns "" when neither variable is set, which makes the global layer
// absent rather than pointing at a relative path.
func defaultGlobalPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "docket", "config.yml")
	}
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".config", "docket", "config.yml")
	}
	return ""
}

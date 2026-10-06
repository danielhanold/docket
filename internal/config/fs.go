package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/danielhanold/docket/internal/layout"
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
//
// A private repository (DIR/.git resolves to a common dir that layout.Detect
// calls private) reads the global config plus LoadPrivateRepositorySource
// instead: it never reads .docket.yml. The mode probe needs no git process;
// any probe error is returned, never guessed.
func LoadFilesystemSources(opts FSOptions) ([]Source, error) {
	if opts.RepoDir == "" {
		return nil, errors.New("config: FSOptions.RepoDir is required")
	}
	repoDir, err := filepath.Abs(opts.RepoDir)
	if err != nil {
		return nil, fmt.Errorf("config: resolving repository directory %q: %w", opts.RepoDir, err)
	}
	repoDir = filepath.Clean(repoDir)

	// A repository directory that is not there must not resolve as "every file
	// layer absent": that reports a valid, mutation-allowed configuration for a
	// repository that does not exist. Stat it once, up front, so a typo and a
	// path that names a file both fail the same way — os.ReadFile alone would
	// treat the first as absent layers and the second as an ENOTDIR read error.
	info, err := os.Stat(repoDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config: repository directory %q does not exist", repoDir)
		}
		return nil, fmt.Errorf("config: inspecting repository directory %q: %w", repoDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("config: repository directory %q is not a directory", repoDir)
	}

	globalPath := opts.GlobalPath
	if globalPath == "" {
		globalPath = defaultGlobalPath()
	}

	common, ok, err := layout.CommonDirOf(repoDir)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	if ok {
		mode, err := layout.Detect(common)
		if err != nil {
			return nil, fmt.Errorf("config: %w", err)
		}
		if mode == layout.Private {
			return loadPrivateSources(globalPath, common, repoDir)
		}
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

// LoadGlobalSource reads the global configuration layer on its own, returning
// zero or one Source. A missing file is an absent layer, not an error.
//
// It exists for the operations that have no repository at all — installing
// docket into a user's home is not something a checkout can have an opinion
// about — so pointing LoadFilesystemSources at a stand-in directory would be
// both a lie and a way for a .docket.yml in the current directory to steer a
// machine-wide install. globalPath is a test seam; "" resolves the default.
func LoadGlobalSource(globalPath string) ([]Source, error) {
	if globalPath == "" {
		globalPath = defaultGlobalPath()
	}
	if globalPath == "" {
		return nil, nil // neither XDG_CONFIG_HOME nor HOME: the layer is absent
	}
	data, err := os.ReadFile(globalPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("config: reading %s: %w", globalPath, err)
	}
	return []Source{{Layer: LayerGlobal, Name: globalPath, Data: data}}, nil
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

// ConflictingLocalConfigError is a private repository that also carries a
// .docket.local.yml: a private repository reads only its private config.
type ConflictingLocalConfigError struct{ LocalPath, PrivatePath string }

func (e *ConflictingLocalConfigError) Error() string {
	return fmt.Sprintf("config: both %s and %s exist; a private repository reads only %s — move any settings you need into it and delete %s",
		e.LocalPath, e.PrivatePath, e.PrivatePath, e.LocalPath)
}

// LoadPrivateRepositorySource reads a private repository's repository layer,
// layout.PrivateConfigPath(commonDir), returning zero or one Source named
// layout.PrivateConfigDisplay. A missing file is an absent layer. A
// <primaryWorktree>/.docket.local.yml beside it is a *ConflictingLocalConfigError:
// a private repository reads only its private config, and silently ignoring
// the local file would drop settings the user believes are in force.
func LoadPrivateRepositorySource(commonDir, primaryWorktree string) ([]Source, error) {
	privatePath := layout.PrivateConfigPath(commonDir)
	localPath := filepath.Join(primaryWorktree, ".docket.local.yml")
	if _, err := os.Stat(localPath); err == nil {
		return nil, &ConflictingLocalConfigError{LocalPath: localPath, PrivatePath: privatePath}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("config: inspecting %s: %w", localPath, err)
	}

	data, err := os.ReadFile(privatePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("config: reading %s: %w", privatePath, err)
	}
	return []Source{{Layer: LayerRepository, Name: layout.PrivateConfigDisplay, Data: data}}, nil
}

// loadPrivateSources is a private repository's full layer stack: the global
// config, then its private repository layer.
func loadPrivateSources(globalPath, commonDir, primaryWorktree string) ([]Source, error) {
	var sources []Source
	if globalPath != "" {
		global, err := LoadGlobalSource(globalPath)
		if err != nil {
			return nil, err
		}
		sources = append(sources, global...)
	}
	repo, err := LoadPrivateRepositorySource(commonDir, primaryWorktree)
	if err != nil {
		return nil, err
	}
	return append(sources, repo...), nil
}

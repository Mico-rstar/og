package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Config holds all path derivation for the og tool.
type Config struct {
	OGHome    string
	BaseFile  string // ~/.og/base
	StateFile string // ~/.og/state
	Store     string // ~/.og/store
	LockFile  string // ~/.og/.lock
}

// NewConfig builds a Config from OG_HOME env (default ~/.og).
// When running as root (via sudo), derives the real user's home from SUDO_USER
// instead of HOME (which sudo-rs resets to root's home).
func NewConfig() *Config {
	home := os.Getenv("OG_HOME")
	if home == "" {
		h, err := resolveHomeDir()
		if err != nil {
			panic(fmt.Sprintf("cannot determine home dir: %v", err))
		}
		home = filepath.Join(h, ".og")
	}
	return &Config{
		OGHome:    home,
		BaseFile:  filepath.Join(home, "base"),
		StateFile: filepath.Join(home, "state"),
		Store:     filepath.Join(home, "store"),
		LockFile:  filepath.Join(home, ".lock"),
	}
}

// resolveHomeDir determines the real user's home directory.
// When running as root via sudo, HOME is typically reset to /root.
// In that case, use SUDO_USER to look up the real user's home from /etc/passwd.
func resolveHomeDir() (string, error) {
	if os.Geteuid() != 0 {
		return os.UserHomeDir()
	}
	// Running as root — try SUDO_USER.
	sudoUser := os.Getenv("SUDO_USER")
	if sudoUser == "" {
		return os.UserHomeDir()
	}
	// Look up home dir from /etc/passwd.
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return os.UserHomeDir()
	}
	for _, line := range splitLines(string(data)) {
		fields := splitFields(line, ':')
		if len(fields) >= 6 && fields[0] == sudoUser {
			return fields[5], nil
		}
	}
	return os.UserHomeDir()
}

// Workspace is a single recorded workspace entry in the state file.
type Workspace struct {
	Name       string `json:"name"`
	Branch     string `json:"branch"`
	BaseCommit string `json:"base_commit"`
	Upper      string `json:"upper"`
	Work       string `json:"work"`
	Merged     string `json:"merged"`
}

// BaseState is the persisted base fingerprint and metadata.
type BaseState struct {
	Path         string                    `json:"path"`
	Commit       string                    `json:"commit"`
	GitDirty     bool                      `json:"git_dirty"`
	Locked       bool                      `json:"locked"`
	Dependencies map[string]DepFingerprint `json:"dependencies"`
	LockfileHash string                    `json:"lockfile_hash"`
}

// DepFingerprint is a coarse fingerprint of a dependency directory.
type DepFingerprint struct {
	LockfileHash string `json:"lockfile_hash"`
	FileCount    int    `json:"file_count"`
	TotalSize    int64  `json:"total_size"`
}

var (
	ErrNoBase        = errors.New("no base repo registered; run 'og init' in your repo first")
	ErrNotFound      = errors.New("workspace not found")
	ErrAlreadyExists = errors.New("workspace already exists")
)

// Base returns the recorded base repo path.
func (c *Config) Base() (string, error) {
	data, err := os.ReadFile(c.BaseFile)
	if err != nil {
		return "", ErrNoBase
	}
	return string(data), nil
}

// StoreDir returns ~/.og/store/<name>.
func (c *Config) StoreDir(name string) string { return filepath.Join(c.Store, name) }

// Upper returns ~/.og/store/<name>/upper.
func (c *Config) Upper(name string) string { return filepath.Join(c.StoreDir(name), "upper") }

// Work returns ~/.og/store/<name>/work.
func (c *Config) Work(name string) string { return filepath.Join(c.StoreDir(name), "work") }

// Merged returns ~/.og/merged/<name>.
// Kept outside base so it works when base is read-only locked.
// The basename is <name> so git worktree git-dir names are unique.
func (c *Config) Merged(base, name string) string {
	_ = base
	return filepath.Join(c.OGHome, "merged", name)
}

// LoadState reads the state file (JSON array of Workspace).
func (c *Config) LoadState() ([]Workspace, error) {
	data, err := os.ReadFile(c.StateFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var ws []Workspace
	if err := json.Unmarshal(data, &ws); err != nil {
		return nil, fmt.Errorf("corrupt state file: %w", err)
	}
	return ws, nil
}

// SaveState atomically writes the state file (JSON array).
func (c *Config) SaveState(ws []Workspace) error {
	if err := os.MkdirAll(c.OGHome, 0o755); err != nil {
		return err
	}
	unlock, err := c.lock()
	if err != nil {
		return err
	}
	defer unlock()
	data, err := json.MarshalIndent(ws, "", "  ")
	if err != nil {
		return err
	}
	tmp := c.StateFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, c.StateFile)
}

// FindWorkspace returns the workspace with the given name.
func (c *Config) FindWorkspace(name string) (*Workspace, error) {
	ws, err := c.LoadState()
	if err != nil {
		return nil, err
	}
	for i := range ws {
		if ws[i].Name == name {
			return &ws[i], nil
		}
	}
	return nil, ErrNotFound
}

// AddWorkspace appends or replaces a workspace in the state.
func (c *Config) AddWorkspace(w Workspace) error {
	ws, err := c.LoadState()
	if err != nil {
		return err
	}
	for i := range ws {
		if ws[i].Name == w.Name {
			return ErrAlreadyExists
		}
	}
	ws = append(ws, w)
	return c.SaveState(ws)
}

// RemoveWorkspace deletes a workspace from the state.
func (c *Config) RemoveWorkspace(name string) error {
	ws, err := c.LoadState()
	if err != nil {
		return err
	}
	out := ws[:0]
	for _, w := range ws {
		if w.Name != name {
			out = append(out, w)
		}
	}
	return c.SaveState(out)
}

// BaseStatePath returns the path to the base fingerprint file.
func (c *Config) BaseStatePath() string { return filepath.Join(c.OGHome, "base-state") }

// LoadBaseState reads the persisted base state.
func (c *Config) LoadBaseState() (*BaseState, error) {
	data, err := os.ReadFile(c.BaseStatePath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var bs BaseState
	if err := json.Unmarshal(data, &bs); err != nil {
		return nil, fmt.Errorf("corrupt base state: %w", err)
	}
	return &bs, nil
}

// SaveBaseState atomically writes the base state.
func (c *Config) SaveBaseState(bs *BaseState) error {
	if err := os.MkdirAll(c.OGHome, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(bs, "", "  ")
	if err != nil {
		return err
	}
	tmp := c.BaseStatePath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, c.BaseStatePath())
}

// lock acquires a simple file lock for state operations.
func (c *Config) lock() (func(), error) {
	if err := os.MkdirAll(c.OGHome, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(c.LockFile, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := flock(f); err != nil {
		f.Close()
		return nil, err
	}
	return func() {
		flockUnlock(f)
		f.Close()
	}, nil
}

// flock/flockUnlock are implemented in lock_os.go (platform-specific).
var _ = sync.Mutex{}

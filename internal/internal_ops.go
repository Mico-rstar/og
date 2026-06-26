package internal

import (
	"fmt"
	"os"
	"path/filepath"
)

// InternalMount is called as root via sudo. Reads all paths from config, not args.
func InternalMount(cfg *Config, name string) error {
	base, err := cfg.Base()
	if err != nil {
		return err
	}
	upper := cfg.Upper(name)
	work := cfg.Work(name)
	merged := cfg.Merged(base, name)

	if err := os.MkdirAll(merged, 0o755); err != nil {
		return fmt.Errorf("mkdir merged: %w", err)
	}
	if err := os.MkdirAll(upper, 0o755); err != nil {
		return fmt.Errorf("mkdir upper: %w", err)
	}
	if err := os.MkdirAll(work, 0o755); err != nil {
		return fmt.Errorf("mkdir work: %w", err)
	}

	if err := MountOverlay(base, upper, work, merged); err != nil {
		return err
	}

	// Fix ownership of the overlay roots (non-recursive — recursive chown
	// causes mass copy-up). Use SUDO_USER if available.
	user := os.Getenv("SUDO_USER")
	if user == "" {
		user = os.Getenv("USER")
	}
	if user != "" && os.Geteuid() == 0 {
		_ = chownToUser(user, merged)
		_ = chownToUser(user, upper)
		_ = chownToUser(user, work)
	}
	return nil
}

// InternalUmount is called as root via sudo.
func InternalUmount(cfg *Config, name string) error {
	base, err := cfg.Base()
	if err != nil {
		return err
	}
	merged := cfg.Merged(base, name)
	return UmountOverlay(merged)
}

// InternalPurge is called as root via sudo. Deletes the store directory
// (which contains root-owned overlay workdir) and the merged directory.
func InternalPurge(cfg *Config, name string) error {
	storeDir := cfg.StoreDir(name)
	_ = os.RemoveAll(storeDir)

	base, err := cfg.Base()
	if err != nil {
		return err
	}
	merged := cfg.Merged(base, name)
	_ = os.RemoveAll(merged)

	// Also remove the worktree git-dir (since .git file was moved to upper,
	// git worktree remove can't work — we clean it up manually).
	// git names the worktree git-dir after the basename of the merged path,
	// which is <name> by construction.
	gtdir := filepath.Join(base, ".git", "worktrees", name)
	_ = os.RemoveAll(gtdir)

	return nil
}

// InternalLock is called as root via sudo.
func InternalLock(cfg *Config) error {
	base, err := cfg.Base()
	if err != nil {
		return err
	}
	return LockBase(base)
}

// InternalUnlock is called as root via sudo.
func InternalUnlock(cfg *Config) error {
	base, err := cfg.Base()
	if err != nil {
		return err
	}
	return UnlockBase(base)
}

// chownToUser chowns a path to the given username's uid:gid.
func chownToUser(username, path string) error {
	// Look up uid/gid from /etc/passwd.
	info, err := userLookup(username)
	if err != nil {
		return err
	}
	return os.Chown(path, info.uid, info.gid)
}

type userInfo struct {
	uid int
	gid int
}

func userLookup(username string) (*userInfo, error) {
	// Parse /etc/passwd to find uid:gid.
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return nil, err
	}
	lines := splitLines(string(data))
	for _, line := range lines {
		fields := splitFields(line, ':')
		if len(fields) >= 4 && fields[0] == username {
			uid := atoi(fields[2])
			gid := atoi(fields[3])
			return &userInfo{uid: uid, gid: gid}, nil
		}
	}
	return nil, fmt.Errorf("user %s not found", username)
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func splitFields(s string, sep byte) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// filepath import retained for potential future use.
var _ = filepath.Join

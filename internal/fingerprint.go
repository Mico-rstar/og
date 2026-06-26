package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Default lockfiles to check for dependency fingerprinting.
var defaultLockfiles = []string{
	"package-lock.json", "yarn.lock", "pnpm-lock.yaml",
	"Cargo.lock", "go.sum", "poetry.lock", "Gemfile.lock",
	"composer.lock", "pom.xml", "build.gradle", "build.gradle.kts",
}

// Default dependency directories to fingerprint.
var defaultDepDirs = []string{
	"node_modules", "vendor", ".venv", "venv",
	"target", "build", "dist", "__pycache__",
}

// ComputeBaseState computes the current base state (fingerprint).
func ComputeBaseState(base string) (*BaseState, error) {
	commit, err := RevParse(base, "HEAD")
	if err != nil {
		return nil, err
	}
	dirty, _ := IsDirty(base)

	bs := &BaseState{
		Path:         base,
		Commit:       commit,
		GitDirty:     dirty,
		Dependencies: make(map[string]DepFingerprint),
	}

	// Lockfile hash (combined hash of all present lockfiles).
	lfHash, _ := hashLockfiles(base)
	bs.LockfileHash = lfHash

	// Per dependency directory fingerprint.
	for _, dir := range defaultDepDirs {
		full := filepath.Join(base, dir)
		if _, err := os.Stat(full); err != nil {
			continue
		}
		fc, size, err := countFilesAndSize(full)
		if err != nil {
			continue
		}
		bs.Dependencies[dir] = DepFingerprint{
			FileCount: fc,
			TotalSize: size,
		}
	}

	return bs, nil
}

// hashLockfiles computes a combined SHA256 of all present lockfiles in base.
func hashLockfiles(base string) (string, error) {
	h := sha256.New()
	found := false
	for _, lf := range defaultLockfiles {
		path := filepath.Join(base, lf)
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		found = true
		fmt.Fprintf(h, "%s:", lf)
		if _, err := io.Copy(h, f); err != nil {
			f.Close()
			return "", err
		}
		f.Close()
		h.Write([]byte{0})
	}
	if !found {
		return "", nil
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// countFilesAndSize recursively counts files and total size in dir.
func countFilesAndSize(dir string) (int, int64, error) {
	count := 0
	var size int64
	err := filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if !info.IsDir() {
			count++
			size += info.Size()
		}
		return nil
	})
	return count, size, err
}

// DiffBaseState compares current base state against recorded, returns warnings.
type BaseHealth struct {
	SourceChanged   bool // HEAD changed or working tree dirty
	CommitRecorded  string
	CommitCurrent   string
	GitDirty        bool
	DepChanged      bool // dependency fingerprint changed
	ChangedDepDirs  []string
	LockfileChanged bool
}

// CheckBaseHealth compares the recorded base state with the current state.
func CheckBaseHealth(recorded *BaseState, current *BaseState) BaseHealth {
	h := BaseHealth{
		CommitRecorded: recorded.Commit,
		CommitCurrent:  current.Commit,
		GitDirty:       current.GitDirty,
	}

	if recorded.Commit != current.Commit {
		h.SourceChanged = true
	}
	if current.GitDirty {
		h.SourceChanged = true
	}

	if recorded.LockfileHash != current.LockfileHash {
		h.LockfileChanged = true
		h.DepChanged = true
	}

	// Compare per-dir fingerprints.
	for dir, old := range recorded.Dependencies {
		cur, ok := current.Dependencies[dir]
		if !ok {
			h.ChangedDepDirs = append(h.ChangedDepDirs, dir+" (removed)")
			h.DepChanged = true
			continue
		}
		if old.FileCount != cur.FileCount || old.TotalSize != cur.TotalSize {
			h.ChangedDepDirs = append(h.ChangedDepDirs, dir)
			h.DepChanged = true
		}
	}
	// Check for newly added dep dirs.
	for dir := range current.Dependencies {
		if _, ok := recorded.Dependencies[dir]; !ok {
			h.ChangedDepDirs = append(h.ChangedDepDirs, dir+" (added)")
			h.DepChanged = true
		}
	}

	return h
}

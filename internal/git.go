package internal

import (
	"fmt"
	"os/exec"
	"strings"
)

// Git runs a git command in the given repo dir.
// Returns combined stdout+stderr.
func Git(dir string, args ...string) (string, error) {
	full := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", full...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// GitOK runs git and returns (output, true) on success, (output, false) on failure.
func GitOK(dir string, args ...string) (string, bool) {
	out, err := Git(dir, args...)
	return out, err == nil
}

// RevParse runs git rev-parse <ref>.
func RevParse(dir, ref string) (string, error) {
	out, err := Git(dir, "rev-parse", ref)
	if err != nil {
		return "", fmt.Errorf("git rev-parse %s: %s", ref, strings.TrimSpace(out))
	}
	return strings.TrimSpace(out), nil
}

// CurrentBranch returns the current branch name in dir, or "" if detached.
func CurrentBranch(dir string) string {
	out, ok := GitOK(dir, "branch", "--show-current")
	if !ok {
		return ""
	}
	return strings.TrimSpace(out)
}

// IsDirty returns true if the working tree has uncommitted changes.
func IsDirty(dir string) (bool, error) {
	out, err := Git(dir, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("git status: %s", strings.TrimSpace(out))
	}
	return strings.TrimSpace(out) != "", nil
}

// IsRepo returns true if dir is inside a git repository.
func IsRepo(dir string) bool {
	_, ok := GitOK(dir, "rev-parse", "--git-dir")
	return ok
}

// BranchExists returns true if the branch exists (local).
func BranchExists(dir, branch string) bool {
	_, ok := GitOK(dir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return ok
}

// CommitExists returns true if the given commit-ish resolves.
func CommitExists(dir, commit string) bool {
	_, ok := GitOK(dir, "cat-file", "-e", commit)
	return ok
}

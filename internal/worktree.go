package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AddWorktree creates a git worktree with its own git-dir (shared object store).
// It uses --no-checkout because overlay will provide the files.
// If newBranch is non-empty, creates a new branch; otherwise checks out existingBranch.
func AddWorktree(base, merged, newBranch, existingBranch string) error {
	args := []string{"worktree", "add", "--no-checkout", "--force"}
	if newBranch != "" {
		args = append(args, "-b", newBranch, merged)
	} else {
		// git worktree add <path> <commit-ish>
		args = append(args, merged, existingBranch)
	}
	out, err := Git(base, args...)
	if err != nil {
		return fmt.Errorf("git worktree add: %s", strings.TrimSpace(out))
	}
	return nil
}

// RemoveWorktree removes a worktree entry (cleanup). Uses --force since the
// working dir is an overlay mount point and may not exist after umount.
func RemoveWorktree(base, merged string) error {
	out, err := Git(base, "worktree", "remove", "--force", merged)
	if err != nil {
		// If the worktree is already gone, try prune as fallback.
		_, _ = Git(base, "worktree", "prune")
		return fmt.Errorf("git worktree remove: %s", strings.TrimSpace(out))
	}
	return nil
}

// MoveGitFile moves the .git file from merged to upper so that overlay
// places it in the upper layer, shadowing the lower's .git directory.
func MoveGitFile(merged, upper string) error {
	src := filepath.Join(merged, ".git")
	dst := filepath.Join(upper, ".git")

	// Verify the .git file exists in merged (worktree creates it).
	info, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf(".git not found in worktree: %w", err)
	}

	// It should be a regular file (gitdir pointer), not a directory.
	if info.IsDir() {
		// If it's a directory (shouldn't happen with --no-checkout), error out.
		return fmt.Errorf("expected .git file in worktree, found directory")
	}

	if err := os.MkdirAll(upper, 0o755); err != nil {
		return err
	}
	return os.Rename(src, dst)
}

// FixGitdirPointer rewrites the `gitdir` file inside the worktree's git-dir
// to point to merged/.git instead of the original worktree path's .git.
// This is needed because git validates that the gitdir pointer matches the
// actual working directory.
func FixGitdirPointer(base, name, merged string) error {
	// The worktree git-dir is at <base>/.git/worktrees/<name>/
	gtdir := filepath.Join(base, ".git", "worktrees", name)
	gitdirFile := filepath.Join(gtdir, "gitdir")

	content, err := os.ReadFile(gitdirFile)
	if err != nil {
		// If the gitdir file doesn't exist, git might not validate. Skip.
		return nil
	}

	// The content is like: /path/to/merged/.git
	// We need it to point to merged/.git (same thing if merged is the path
	// we used in worktree add). If it already matches, skip.
	expected := filepath.Join(merged, ".git")
	if strings.TrimSpace(string(content)) == expected {
		return nil
	}

	return os.WriteFile(gitdirFile, []byte(expected+"\n"), 0o644)
}

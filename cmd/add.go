package cmd

import (
	"os"
	"path/filepath"

	"github.com/linrunxin/og/internal"
	"github.com/spf13/cobra"
)

var addNewBranch bool

var addCmd = &cobra.Command{
	Use:   "add <name> [<branch>] [-b <new-branch>]",
	Short: "Create an overlay workspace",
	Long: `Create a new overlay workspace.

  og add <name> <branch>           Checkout an existing branch
  og add <name> -b <new-branch>    Create a new branch and checkout

The workspace shares the base's files (source + dependencies) via overlayfs.
Only diffs are copied to the upper layer.`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		c, base := requireBase()

		name := args[0]
		var newBranch, existingBranch string

		if addNewBranch {
			if len(args) < 2 {
				internal.Die("usage: og add <name> -b <new-branch>")
			}
			newBranch = args[1]
		} else {
			if len(args) < 2 {
				internal.Die("usage: og add <name> <branch>\n      og add <name> -b <new-branch>")
			}
			existingBranch = args[1]
			if !internal.BranchExists(base, existingBranch) {
				internal.Die("branch '%s' does not exist", existingBranch)
			}
		}

		// Determine the effective branch name.
		branch := newBranch
		if branch == "" {
			branch = existingBranch
		}

		// Prevent checking out a branch that's already in use by another workspace.
		// git worktree normally enforces this, but overlay's git-dir isolation
		// bypasses that protection.
		existing, _ := c.LoadState()
		for _, w := range existing {
			if w.Branch == branch {
				internal.Die("branch '%s' is already checked out in workspace '%s'\n  use a different branch or 'og add %s -b <new-branch>'", branch, w.Name, name)
			}
		}

		// Check name collision.
		if _, err := c.FindWorkspace(name); err == nil {
			internal.Die("workspace '%s' already exists", name)
		}

		// Verify upper doesn't have stale data.
		upper := c.Upper(name)
		if entries, _ := os.ReadDir(upper); len(entries) > 0 {
			internal.Die("workspace '%s' has stale upper data. Run 'og remove %s' first.", name, name)
		}

		merged := c.Merged(base, name)

		// Warn if base has changed since workspaces were created.
		checkBaseHealth(c, base)

		// 1. Create git worktree (shared object store, independent git-dir).
		if err := internal.AddWorktree(base, merged, newBranch, existingBranch); err != nil {
			internal.Die("%v", err)
		}

		// 2. Move .git file from merged to upper so overlay places it in upper,
		//    shadowing the lower's .git directory.
		if err := internal.MoveGitFile(merged, upper); err != nil {
			_ = internal.RemoveWorktree(base, merged)
			internal.Die("%v", err)
		}

		// 3. Fix gitdir pointer to point to merged/.git.
		if err := internal.FixGitdirPointer(base, name, merged); err != nil {
			_ = internal.RemoveWorktree(base, merged)
			_ = os.Remove(filepath.Join(upper, ".git"))
			internal.Die("%v", err)
		}

		// 4. Mount overlay.
		if err := internal.MountWorkspacePrivileged(name); err != nil {
			_ = internal.RemoveWorktree(base, merged)
			_ = os.RemoveAll(c.StoreDir(name))
			internal.Die("%v", err)
		}

		// 5. Align source to branch (diffs copy-up to upper).
		out, err := internal.Git(merged, "reset", "--hard", "HEAD")
		if err != nil {
			internal.Warn("git reset --hard: %s", out)
		}

		// 6. Record base commit.
		commit, _ := internal.RevParse(base, "HEAD")

		// 7. Save state.
		ws := internal.Workspace{
			Name:       name,
			Branch:     branch,
			BaseCommit: commit,
			Upper:      upper,
			Work:       c.Work(name),
			Merged:     merged,
		}
		if err := c.AddWorkspace(ws); err != nil {
			internal.Warn("save state: %v", err)
		}

		internal.Say("workspace ready: %s", merged)
		internal.Dim("  lower: %s", base)
		internal.Dim("  upper: %s", upper)
		internal.Dim("  branch: %s", branch)
		internal.Dim("")
		internal.Dim("cd into it:  og enter %s", name)
		internal.Dim("or:          cd %s", merged)
	},
}

func init() {
	addCmd.Flags().BoolVarP(&addNewBranch, "new", "b", false, "create a new branch")
	rootCmd.AddCommand(addCmd)
}

// checkBaseHealth warns if the base has changed since workspaces were created.
func checkBaseHealth(c *internal.Config, base string) {
	recorded, err := c.LoadBaseState()
	if err != nil || recorded == nil {
		return
	}
	current, err := internal.ComputeBaseState(base)
	if err != nil {
		return
	}
	h := internal.CheckBaseHealth(recorded, current)
	if h.SourceChanged {
		internal.Warn("base HEAD changed (%s → %s)", recorded.Commit[:8], current.Commit[:8])
		internal.Warn("run 'og sync' to fix workspace source tearing")
	}
}

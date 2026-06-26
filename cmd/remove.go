package cmd

import (
	"github.com/linrunxin/og/internal"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:     "remove <name>",
	Aliases: []string{"rm"},
	Short:   "Remove a workspace",
	Long: `Remove a workspace completely.

Unmounts the overlay, removes the git worktree, and deletes the upper data.
This is a full cleanup — there is no "unmount only" mode.

To preserve your work, commit or push before removing.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		c, _ := requireBase()
		name := args[0]

		if _, err := c.FindWorkspace(name); err != nil {
			internal.Die("workspace '%s' not found", name)
		}

		// 1. Unmount overlay.
		if err := internal.UmountWorkspacePrivileged(name); err != nil {
			internal.Warn("umount: %v", err)
		} else {
			internal.Say("unmounted: %s", name)
		}

		// 2. Purge store dir (root-owned overlay workdir) + worktree git-dir.
		//    Since .git file was moved to upper, git worktree remove can't work.
		//    We clean up the worktree git-dir manually via privileged purge.
		if err := internal.PurgeWorkspacePrivileged(name); err != nil {
			internal.Warn("purge: %v", err)
		}

		// 3. Remove from state.
		if err := c.RemoveWorkspace(name); err != nil {
			internal.Warn("remove state: %v", err)
		}

		internal.Say("removed: %s", name)
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
}

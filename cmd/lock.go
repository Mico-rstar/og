package cmd

import (
	"github.com/linrunxin/og/internal"
	"github.com/spf13/cobra"
)

var baseCmd = &cobra.Command{
	Use:   "base",
	Short: "Manage the base repo",
	Long: `Manage the base repo.

The base is normally locked (read-only). Use 'og base unlock' when you need
to update dependencies or pull changes. It auto-locks on the next 'og ls' or
'og add'.`,
}

var baseUnlockCmd = &cobra.Command{
	Use:   "unlock",
	Short: "Unlock the base repo (writable)",
	Run: func(cmd *cobra.Command, args []string) {
		c, base := requireBase()

		if !internal.IsBaseLocked(base) {
			internal.Say("base already unlocked")
			return
		}

		if err := internal.UnlockBasePrivileged(); err != nil {
			internal.Die("%v", err)
		}

		if bs, _ := c.LoadBaseState(); bs != nil {
			bs.Locked = false
			_ = c.SaveBaseState(bs)
		}

		internal.Say("base unlocked (writable)")
		internal.Dim("run 'og ls' to re-lock, or 'og base lock'")
	},
}

var baseLockCmd = &cobra.Command{
	Use:   "lock",
	Short: "Lock the base repo (read-only)",
	Run: func(cmd *cobra.Command, args []string) {
		c, base := requireBase()

		if internal.IsBaseLocked(base) {
			internal.Say("base already locked")
			return
		}

		if err := internal.LockBasePrivileged(); err != nil {
			internal.Die("%v", err)
		}

		// Recompute fingerprint after changes.
		if bs, err := internal.ComputeBaseState(base); err == nil {
			bs.Locked = true
			_ = c.SaveBaseState(bs)
		}

		internal.Say("base locked (read-only, .git writable)")
	},
}

func init() {
	baseCmd.AddCommand(baseUnlockCmd)
	baseCmd.AddCommand(baseLockCmd)
	rootCmd.AddCommand(baseCmd)
}

package cmd

import (
	"os"

	"github.com/linrunxin/og/internal"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Mark current repo as base",
	Long: `Mark the current git repository as the base (read-only baseline).

This:
  - Records the repo path in ~/.og/base
  - Installs sudoers rules for passwordless mount/umount/lock/unlock
  - Installs a systemd user service for auto-resume on reboot
  - Adds /.og/ to .git/info/exclude
  - Records a base fingerprint (commit + dependency hash)
  - Locks the base (read-only bind mount, .git stays writable)`,
	Run: func(cmd *cobra.Command, args []string) {
		repo, err := os.Getwd()
		if err != nil {
			internal.Die("cannot get cwd: %v", err)
		}
		if !internal.IsRepo(repo) {
			internal.Die("not a git repo: %s", repo)
		}

		c := cfg()

		// Create OG_HOME.
		if err := os.MkdirAll(c.OGHome, 0o755); err != nil {
			internal.Die("mkdir %s: %v", c.OGHome, err)
		}
		if err := os.MkdirAll(c.Store, 0o755); err != nil {
			internal.Die("mkdir %s: %v", c.Store, err)
		}

		// Record base.
		if err := os.WriteFile(c.BaseFile, []byte(repo), 0o644); err != nil {
			internal.Die("write base: %v", err)
		}

		// Add /.og/ to .git/info/exclude.
		if err := internal.AddExclude(repo); err != nil {
			internal.Warn("could not add exclude: %v", err)
		}

		// Install sudoers + systemd.
		if err := internal.InstallSudoers(); err != nil {
			internal.Warn("sudoers install failed: %v", err)
			internal.Warn("mount/umount/lock will require interactive sudo")
		}
		if err := internal.InstallSystemd(); err != nil {
			internal.Warn("systemd install failed: %v", err)
		}

		// Compute and save base fingerprint.
		bs, err := internal.ComputeBaseState(repo)
		if err != nil {
			internal.Warn("fingerprint failed: %v", err)
		} else {
			bs.Locked = true
			if err := c.SaveBaseState(bs); err != nil {
				internal.Warn("save base state: %v", err)
			}
		}

		// Lock the base.
		if err := internal.LockBasePrivileged(); err != nil {
			internal.Warn("lock failed: %v", err)
			internal.Warn("base remains writable — run 'og base lock' to lock manually")
		}

		internal.Say("base repo: %s", repo)
		internal.Say("sudoers installed (passwordless mount/umount/lock for %s)", os.Getenv("USER"))
		internal.Say("systemd service enabled (auto-resume on boot)")
		internal.Say("base locked (read-only, .git writable)")
		internal.Dim("")
		internal.Dim("Next: og add <name> <branch>            checkout existing branch")
		internal.Dim("      og add <name> -b <new-branch>     create new branch")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

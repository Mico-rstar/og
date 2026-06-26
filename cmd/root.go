package cmd

import (
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/linrunxin/og/internal"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "og",
	Short: "overlayfs-based parallel git workspaces",
	Long: `og — overlayfs + git worktree for parallel branch development.

Creates isolated workspaces that share a read-only base (source + dependencies)
via overlayfs, while git worktree provides independent branches with a shared
object store. This gives you:
  - Full git branch isolation (like git worktree)
  - Shared dependencies (node_modules, target, etc.) via overlay COW
  - Minimal disk usage — only diffs are copied to the upper layer

USAGE:
  og init                       Mark current repo as base (run once)
  og add <name> [-b <branch>]   Create a workspace
  og <name> [cmd...]            Enter workspace (or run a command in it)
  og ls                         List workspaces
  og rm <name>                  Remove a workspace
  og base unlock                Unlock base for changes
  og help                       Show this help`,
	// Don't let cobra reject unknown subcommands — we use them as workspace names.
	SilenceErrors: true,
	SilenceUsage:  true,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Help()
			return
		}
		// Any unknown first arg = workspace name.
		enterWorkspace(args[0], args[1:])
	},
}

// Execute is the main entry point for the CLI.
// Before cobra routing, check if the first arg is an unknown subcommand
// (i.e., a workspace name) and handle it directly.
func Execute() {
	args := os.Args[1:]
	if len(args) > 0 && !isKnownSubcommand(args[0]) && args[0] != "-h" && args[0] != "--help" {
		enterWorkspace(args[0], args[1:])
		return
	}
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// cfg returns a Config derived from the environment.
func cfg() *internal.Config {
	return internal.NewConfig()
}

// requireBase loads config and base, dies if not initialized.
func requireBase() (*internal.Config, string) {
	c := cfg()
	base, err := c.Base()
	if err != nil {
		internal.Die("%v", err)
	}
	return c, base
}

// isKnownSubcommand checks if name matches a registered subcommand or alias.
func isKnownSubcommand(name string) bool {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == name {
			return true
		}
		for _, alias := range cmd.Aliases {
			if alias == name {
				return true
			}
		}
	}
	return false
}

// enterWorkspace is the shared logic for `og <name>` and `og enter <name>`.
func enterWorkspace(name string, cmdArgs []string) {
	c, _ := requireBase()

	ws, err := c.FindWorkspace(name)
	if err != nil {
		internal.Die("workspace '%s' not found", name)
	}

	if !isMountpoint(ws.Merged) {
		// Auto-resume if upper data exists.
		if _, statErr := os.Stat(ws.Upper); statErr == nil {
			if mountErr := internal.MountWorkspacePrivileged(name); mountErr != nil {
				internal.Die("workspace '%s' not mounted and resume failed: %v", name, mountErr)
			}
		} else {
			internal.Die("workspace '%s' not mounted. Run 'og add %s <branch>'.", name, name)
		}
	}

	// Auto-sync: if base changed, fix source tearing before entering.
	autoSync(c, ws)

	// Run command or interactive shell.
	runInWorkspace(ws.Merged, cmdArgs)
}

// runInWorkspace execs a command or interactive shell in the merged dir.
func runInWorkspace(merged string, cmdArgs []string) {
	if len(cmdArgs) > 0 {
		runCommand(merged, cmdArgs)
		return
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "bash"
	}
	internal.Say("entering %s", merged)
	internal.Dim("exit to leave.")
	runCommand(merged, []string{shell})
}

// runCommand runs a command in the workspace directory.
func runCommand(merged string, cmdArgs []string) {
	c2 := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	c2.Dir = merged
	c2.Stdin = os.Stdin
	c2.Stdout = os.Stdout
	c2.Stderr = os.Stderr
	if err := c2.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				os.Exit(status.ExitStatus())
			}
		}
		internal.Die("%v", err)
	}
}

// autoSync checks if base changed and fixes source tearing.
func autoSync(c *internal.Config, ws *internal.Workspace) {
	recorded, err := c.LoadBaseState()
	if err != nil || recorded == nil {
		return
	}
	base, err := c.Base()
	if err != nil {
		return
	}
	current, err := internal.ComputeBaseState(base)
	if err != nil {
		return
	}
	h := internal.CheckBaseHealth(recorded, current)
	if !h.SourceChanged {
		return
	}
	dirty, _ := internal.IsDirty(ws.Merged)
	if dirty {
		internal.Warn("base changed but '%s' has uncommitted changes — skipping sync", ws.Name)
		return
	}
	out, err := internal.Git(ws.Merged, "reset", "--hard", "HEAD")
	if err != nil {
		internal.Warn("auto-sync failed: %s", strings.TrimSpace(out))
		return
	}
	internal.Say("synced '%s' (base changed)", ws.Name)
}

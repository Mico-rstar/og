package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/linrunxin/og/internal"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List workspaces",
	Run: func(cmd *cobra.Command, args []string) {
		c, base := requireBase()

		// Auto-resume: re-mount workspaces that have upper data but aren't mounted.
		autoResume(c, base)

		// Auto-prune: remove stale entries.
		autoPrune(c, base)

		// Show base status.
		fmt.Printf("%sbase%s: %s\n", internal.ColorBold(), internal.ColorReset(), base)

		// Base health.
		if recorded, _ := c.LoadBaseState(); recorded != nil {
			if current, err := internal.ComputeBaseState(base); err == nil {
				h := internal.CheckBaseHealth(recorded, current)
				if h.SourceChanged {
					fmt.Printf("  %s⚠ base HEAD changed%s (%s → %s)\n",
						internal.ColorYellow(), internal.ColorReset(),
						shortHash(recorded.Commit), shortHash(current.Commit))
					fmt.Printf("  %s⚠ workspaces will auto-sync on enter%s\n",
						internal.ColorYellow(), internal.ColorReset())
				}
				if h.DepChanged {
					fmt.Printf("  %s⚠ base dependencies changed%s: %s\n",
						internal.ColorYellow(), internal.ColorReset(), strings.Join(h.ChangedDepDirs, ", "))
					fmt.Printf("  %s⚠ run 'og rm <name> && og add <name> <branch>' to rebuild%s\n",
						internal.ColorYellow(), internal.ColorReset())
				}
			}
		}

		// Locked?
		if internal.IsBaseLocked(base) {
			fmt.Printf("  %slocked%s (read-only)\n", internal.ColorGreen(), internal.ColorReset())
		} else {
			fmt.Printf("  %sunlocked%s (run 'og base unlock' to lock)\n", internal.ColorYellow(), internal.ColorReset())
		}

		fmt.Println()

		// List workspaces.
		ws, err := c.LoadState()
		if err != nil {
			internal.Die("load state: %v", err)
		}
		if len(ws) == 0 {
			internal.Dim("  (no workspaces)")
			return
		}

		for _, w := range ws {
			status := internal.ColorRed() + "unmounted" + internal.ColorReset()
			branch := "-"
			upperSize := "-"

			if isMountpoint(w.Merged) {
				status = internal.ColorGreen() + "mounted" + internal.ColorReset()
				if b := internal.CurrentBranch(w.Merged); b != "" {
					branch = b
				}
				upperSize = du(w.Upper)
			}

			fmt.Printf("  %s%s%s  %s  branch=%s  upper=%s\n",
				internal.ColorCyan(), w.Name, internal.ColorReset(),
				status, branch, upperSize)
		}
	},
}

// autoResume re-mounts workspaces that have upper data but aren't mounted.
func autoResume(c *internal.Config, base string) {
	ws, err := c.LoadState()
	if err != nil {
		return
	}
	for _, w := range ws {
		if isMountpoint(w.Merged) {
			continue
		}
		if _, err := os.Stat(w.Upper); err != nil {
			continue // upper gone
		}
		_ = internal.MountWorkspacePrivileged(w.Name)
	}
}

// autoPrune removes stale workspace entries (upper gone or worktree git-dir gone).
func autoPrune(c *internal.Config, base string) {
	ws, err := c.LoadState()
	if err != nil {
		return
	}
	var keep []internal.Workspace
	removed := false
	for _, w := range ws {
		stale := false
		if _, err := os.Stat(w.Upper); err != nil {
			stale = true
		}
		gtdir := filepath.Join(base, ".git", "worktrees", w.Name)
		if _, err := os.Stat(gtdir); err != nil {
			stale = true
		}
		if stale {
			if isMountpoint(w.Merged) {
				_ = internal.UmountWorkspacePrivileged(w.Name)
			}
			_ = internal.PurgeWorkspacePrivileged(w.Name)
			removed = true
		} else {
			keep = append(keep, w)
		}
	}
	if removed {
		_ = c.SaveState(keep)
	}
}

func isMountpoint(path string) bool {
	return exec.Command("mountpoint", "-q", path).Run() == nil
}

func du(path string) string {
	out, err := exec.Command("du", "-sh", path).CombinedOutput()
	if err != nil {
		return "-"
	}
	fields := strings.Fields(string(out))
	if len(fields) > 0 {
		return fields[0]
	}
	return "-"
}

func shortHash(h string) string {
	if len(h) > 8 {
		return h[:8]
	}
	return h
}

func init() {
	rootCmd.AddCommand(listCmd)
}

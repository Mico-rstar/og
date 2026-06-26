package main

import (
	"os"

	"github.com/linrunxin/og/cmd"
	"github.com/linrunxin/og/internal"
)

func main() {
	// Handle internal privileged entrypoints (called via sudo).
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--internal-mount":
			if len(os.Args) < 3 {
				os.Exit(1)
			}
			cfg := internal.NewConfig()
			if err := internal.InternalMount(cfg, os.Args[2]); err != nil {
				os.Stderr.WriteString(err.Error() + "\n")
				os.Exit(1)
			}
			return
		case "--internal-umount":
			if len(os.Args) < 3 {
				os.Exit(1)
			}
			cfg := internal.NewConfig()
			if err := internal.InternalUmount(cfg, os.Args[2]); err != nil {
				os.Stderr.WriteString(err.Error() + "\n")
				os.Exit(1)
			}
			return
		case "--internal-lock":
			cfg := internal.NewConfig()
			if err := internal.InternalLock(cfg); err != nil {
				os.Stderr.WriteString(err.Error() + "\n")
				os.Exit(1)
			}
			return
		case "--internal-unlock":
			cfg := internal.NewConfig()
			if err := internal.InternalUnlock(cfg); err != nil {
				os.Stderr.WriteString(err.Error() + "\n")
				os.Exit(1)
			}
			return
		case "--internal-purge":
			if len(os.Args) < 3 {
				os.Exit(1)
			}
			cfg := internal.NewConfig()
			if err := internal.InternalPurge(cfg, os.Args[2]); err != nil {
				os.Stderr.WriteString(err.Error() + "\n")
				os.Exit(1)
			}
			return
		}
	}

	cmd.Execute()
}

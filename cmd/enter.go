package cmd

import (
	"github.com/spf13/cobra"
)

var enterCmd = &cobra.Command{
	Use:                "enter <name> [cmd...]",
	Short:              "Enter a workspace (alias: og <name> [cmd...])",
	DisableFlagParsing: true,
	Args:               cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		enterWorkspace(args[0], args[1:])
	},
}

func init() {
	rootCmd.AddCommand(enterCmd)
}

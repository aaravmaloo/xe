package cmd

import "github.com/spf13/cobra"

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage project settings",
}

var configAutoVenvCmd = &cobra.Command{
	Use:   "autovenv <on|off>",
	Short: "Toggle project autovenv setting",
	Args:  cobra.ExactArgs(1),
	Run:   venvAutoCmd.Run,
}

func init() {
	configCmd.AddCommand(configAutoVenvCmd)
	rootCmd.AddCommand(configCmd)
}

package cmd

import (
	"fmt"
	"xe/src/internal/core"

	"github.com/spf13/cobra"
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot <name>",
	Short: "Create a snapshot of the current xe state",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		err := core.CreateSnapshot(name)
		if err != nil {
			fmt.Printf("Error creating snapshot: %v\n", err)
			return
		}
		fmt.Printf("Snapshot '%s' created successfully\n", name)
	},
}

var restoreCmd = &cobra.Command{
	Use:   "restore <name>",
	Short: "Restore xe state from a snapshot",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		err := core.RestoreSnapshot(name)
		if err != nil {
			fmt.Printf("Error restoring snapshot: %v\n", err)
			return
		}
		fmt.Printf("Successfully restored snapshot '%s'\n", name)
	},
}

func init() {
	rootCmd.AddCommand(snapshotCmd)
	rootCmd.AddCommand(restoreCmd)
}

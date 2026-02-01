package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var workspaceCmd = &cobra.Command{
	Use:     "workspace",
	Aliases: []string{"workspaces"},
	Short:   "Manage monorepos and workspaces",
}

var wsInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new workspace",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Initialized xe workspace")
	},
}

var wsAddCmd = &cobra.Command{
	Use:   "add <path>",
	Short: "Add a project to the workspace",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]
		fmt.Printf("Added %s to workspace\n", path)
	},
}

func init() {
	workspaceCmd.AddCommand(wsInitCmd)
	workspaceCmd.AddCommand(wsAddCmd)
	rootCmd.AddCommand(workspaceCmd)
}

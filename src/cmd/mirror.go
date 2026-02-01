package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var mirrorCmd = &cobra.Command{
	Use:   "mirror",
	Short: "Manage PyPI registry mirrors",
}

var mirrorAddCmd = &cobra.Command{
	Use:   "add <url>",
	Short: "Add a new PyPI mirror",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		url := args[0]
		fmt.Printf("Added mirror: %s\n", url)
	},
}

var mirrorListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured mirrors",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Configured mirrors:")
		fmt.Println("- https://pypi.org/simple (Default)")
	},
}

func init() {
	mirrorCmd.AddCommand(mirrorAddCmd)
	mirrorCmd.AddCommand(mirrorListCmd)
	rootCmd.AddCommand(mirrorCmd)
}

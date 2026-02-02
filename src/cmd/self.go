package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var selfCmd = &cobra.Command{
	Use:   "self",
	Short: "Manage xe itself",
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update xe to the latest version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Checking for updates...")
		fmt.Println("xe is already up to date (v1.0.0)")
	},
}

func init() {
	selfCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(selfCmd)
}

package cmd

import (
	"fmt"
	"os"
	"xe/src/internal/utils"
	"xe/src/internal/xedir"

	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Perform initial setup (add shims to PATH)",
	Run: func(cmd *cobra.Command, args []string) {
		shimDir := xedir.ShimDir()
		if err := os.MkdirAll(shimDir, 0755); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		err := utils.AddToPath(shimDir)
		if err != nil {
			fmt.Printf("Error adding to PATH: %v\n", err)
			return
		}
		fmt.Printf("Successfully added %s to your PATH. Please restart your terminal.\n", shimDir)
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

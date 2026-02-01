package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"xe/src/internal/utils"

	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Perform initial setup (add shims to PATH)",
	Run: func(cmd *cobra.Command, args []string) {
		home, _ := os.UserHomeDir()
		shimDir := filepath.Join(home, ".xe", "bin")
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

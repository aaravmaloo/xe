package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var forceFlag bool

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove all global and local state managed by xe",
	Long: `Remove the global xe data directory (~/.xe), all self-installed 
Python runtimes (~/AppData/Local/Programs/Python), and local project 
configuration files (.xe, xe.toml). WARNING: This operation is destructive.`,
	Run: func(cmd *cobra.Command, args []string) {
		if !forceFlag {
			pterm.Warning.Println("This will delete all global and local xe data, including:")
			fmt.Println("- ~/.xe (config, cache, venvs)")
			fmt.Println("- ~/AppData/Local/Programs/Python (self-installed runtimes)")
			fmt.Println("- .xe and xe.toml in the current directory")
			fmt.Print("\nAre you sure you want to proceed? (y/N): ")

			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(strings.ToLower(input))

			if input != "y" && input != "yes" {
				pterm.Info.Println("Cleanup cancelled.")
				return
			}
		}

		pterm.Info.Println("Starting system-wide cleanup...")

		// 1. Global ~/.xe
		home, _ := os.UserHomeDir()
		xeGlobalDir := filepath.Join(home, ".xe")
		removePath(xeGlobalDir, "Global configuration and data")

		// 2. Self-installed Pythons
		pythonDir := filepath.Join(home, "AppData", "Local", "Programs", "Python")
		removePath(pythonDir, "Self-installed Python runtimes")

		// 3. Local project files
		removePath(".xe", "Local project state")
		removePath("xe.toml", "Local project configuration")

		pterm.Success.Println("Cleanup complete. All xe-related data has been removed.")
	},
}

func removePath(path string, description string) {
	if _, err := os.Stat(path); err == nil {
		pterm.Info.Printf("Removing %s at %s...\n", description, path)
		if err := os.RemoveAll(path); err != nil {
			pterm.Error.Printf("Failed to remove %s: %v\n", path, err)
		}
	}
}

func init() {
	cleanCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Force cleanup without confirmation")
	rootCmd.AddCommand(cleanCmd)
}

package cmd

import (
	"fmt"
	"os"
	"strings"
	"xe/src/internal/python"
	"xe/src/internal/venv"

	"github.com/spf13/cobra"
)

var initPythonVersion string

var initCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Initialize a new project environment",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := "current"
		if len(args) > 0 {
			name = args[0]
		}

		fmt.Printf("Initializing project %s...\n", name)

		// Create .xe directory in project root if not global
		xeDir := ".xe"
		if err := os.MkdirAll(xeDir, 0755); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		vm, _ := venv.NewVenvManager()
		pm, _ := python.NewPythonManager()

		// 1. Determine version: Flag > Config > Default
		version := initPythonVersion
		if version == "" {
			version = GetPreferredPythonVersion()
		}

		pythonExe, err := pm.GetPythonExe(version)
		if err != nil {
			// Try to install it if missing
			if err := pm.Install(version); err == nil {
				pythonExe, _ = pm.GetPythonExe(version)
			}
		}

		// Create basic xe.toml if it doesn't exist
		tomlPath := "xe.toml"
		if _, err := os.Stat(tomlPath); os.IsNotExist(err) {
			content := fmt.Sprintf(`[python]
version = "%s"
abi = "cp%s"

[venv]
name = "%s"

[platform]
os = "windows"
arch = "x86_64"

[deps]
`, version, strings.ReplaceAll(version, ".", ""), name)
			os.WriteFile(tomlPath, []byte(content), 0644)
			fmt.Println("Created xe.toml")
		}

		if pythonExe != "" {
			if err := vm.Create(name, pythonExe); err != nil {
				fmt.Printf("Note: Venv %s already exists or couldn't be created: %v\n", name, err)
			} else {
				fmt.Printf("Created centralized venv: %s (Python %s)\n", name, version)
			}
		}

		fmt.Println("Project initialized successfully.")
		fmt.Printf("To activate, run: xe venv activate %s\n", name)
	},
}

func init() {
	initCmd.Flags().StringVarP(&initPythonVersion, "python", "p", "", "Python version to use for this project")
	rootCmd.AddCommand(initCmd)
}

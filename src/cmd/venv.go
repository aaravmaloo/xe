package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"xe/src/internal/python"
	"xe/src/internal/venv"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var venvCmd = &cobra.Command{
	Use:   "venv",
	Short: "Manage virtual environments",
}

var venvCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new virtual environment",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		vm, _ := venv.NewVenvManager()
		pm, _ := python.NewPythonManager()

		// Use centralized version detector
		version := GetPreferredPythonVersion()

		pythonExe, err := pm.GetPythonExe(version)
		if err != nil {
			pterm.Info.Printf("Python %s not found. Attempting to install...\n", version)
			if err := pm.Install(version); err != nil {
				pterm.Error.Printf("Failed to install Python %s: %v\n", version, err)
				return
			}
			pythonExe, _ = pm.GetPythonExe(version)
		}

		pterm.Info.Printf("Creating venv %s with Python %s...\n", name, version)
		err = vm.Create(name, pythonExe)
		if err != nil {
			pterm.Error.Printf("Failed to create venv: %v\n", err)
			return
		}
		pterm.Success.Printf("Venv %s created.\n", name)
	},
}

var venvListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all managed venvs",
	Run: func(cmd *cobra.Command, args []string) {
		vm, _ := venv.NewVenvManager()
		files, _ := os.ReadDir(vm.BaseDir)

		fmt.Println("Managed Virtual Environments:")
		for _, f := range files {
			if f.IsDir() {
				fmt.Printf("- %s (%s)\n", f.Name(), filepath.Join(vm.BaseDir, f.Name()))
			}
		}
	},
}

var venvActivateCmd = &cobra.Command{
	Use:   "activate <name>",
	Short: "Activate a virtual environment in a new window",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		vm, _ := venv.NewVenvManager()
		psScript := vm.GetPSActivateScript(name)

		if _, err := os.Stat(psScript); os.IsNotExist(err) {
			pterm.Error.Printf("Venv %s does not exist or PowerShell script missing.\n", name)
			return
		}

		pterm.Info.Printf("Launching new PowerShell window with venv '%s' activated...\n", name)

		// Start a new powershell process in a new window and source the activation script
		// cmd /c start powershell -NoExit -Command ". 'path\to\Activate.ps1'"
		shellCmd := exec.Command("cmd", "/c", "start", "powershell", "-NoExit", "-Command", fmt.Sprintf(". '%s'", psScript))
		if err := shellCmd.Start(); err != nil {
			pterm.Error.Printf("Failed to launch new window: %v\n", err)
			return
		}

		pterm.Success.Println("New window launched.")
	},
}

var activateCmd = &cobra.Command{
	Use:   "activate <name>",
	Short: "Alias for venv activate",
	Args:  cobra.ExactArgs(1),
	Run:   venvActivateCmd.Run,
}

var createCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Alias for venv create",
	Args:  cobra.ExactArgs(1),
	Run:   venvCreateCmd.Run,
}

func init() {
	venvCmd.AddCommand(venvCreateCmd)
	venvCmd.AddCommand(venvListCmd)
	venvCmd.AddCommand(venvActivateCmd)
	rootCmd.AddCommand(venvCmd)

	// Top level aliases
	rootCmd.AddCommand(activateCmd)
	rootCmd.AddCommand(createCmd)
}

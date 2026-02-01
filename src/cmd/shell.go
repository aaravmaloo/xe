package cmd

import (
	"os"
	"os/exec"
	"xe/src/internal/venv"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var shellCmd = &cobra.Command{
	Use:   "shell [venv_name]",
	Short: "Enter a new shell with the virtual environment activated",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		vm, _ := venv.NewVenvManager()
		venvName := ""

		if len(args) > 0 {
			venvName = args[0]
		} else {
			// Try to find from local xe.toml
			// (Simplified: we'll just check if a venv named 'current' exists or similar)
			venvName = "current"
		}

		activateScript := vm.GetActivateScript(venvName)
		if _, err := os.Stat(activateScript); os.IsNotExist(err) {
			pterm.Error.Printf("Virtual environment '%s' not found. Create it first with 'xe venv create %s'\n", venvName, venvName)
			return
		}

		pterm.Info.Printf("Entering shell with venv '%s' activated...\n", venvName)
		pterm.Info.Println("Type 'exit' to return to normal shell.")

		// On Windows, we spawn a new cmd.exe that runs the activate.bat first
		shell := exec.Command("cmd.exe", "/K", activateScript)
		shell.Stdin = os.Stdin
		shell.Stdout = os.Stdout
		shell.Stderr = os.Stderr

		if err := shell.Run(); err != nil {
			pterm.Error.Printf("Failed to spawn shell: %v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(shellCmd)
}

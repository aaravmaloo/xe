package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"xe/src/internal/python"
	"xe/src/internal/venv"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:                "run -- [command]",
	Short:              "Run a command within the project's virtual environment",
	DisableFlagParsing: true,
	Run: func(cmd *cobra.Command, args []string) {
		vm, _ := venv.NewVenvManager()
		pm, _ := python.NewPythonManager()

		// 1. Resolve environment
		pythonVersion := GetPreferredPythonVersion()
		globalPath := pm.GetPythonPath(pythonVersion)
		effectivePath := vm.GetEffectivePythonPath(globalPath)

		// 2. Prepare environment variables
		env := os.Environ()

		// Set VIRTUAL_ENV
		venvFound := false
		for i, e := range env {
			if len(e) > 12 && e[:12] == "VIRTUAL_ENV=" {
				env[i] = "VIRTUAL_ENV=" + effectivePath
				venvFound = true
				break
			}
		}
		if !venvFound {
			env = append(env, "VIRTUAL_ENV="+effectivePath)
		}

		// Update PATH to include venv Scripts
		scriptsDir := filepath.Join(effectivePath, "Scripts")
		if _, err := os.Stat(scriptsDir); os.IsNotExist(err) {
			scriptsDir = filepath.Join(effectivePath, "bin") // Unix fallback
		}

		pathValue := os.Getenv("PATH")
		newPath := scriptsDir + string(os.PathListSeparator) + effectivePath + string(os.PathListSeparator) + pathValue

		pathFound := false
		for i, e := range env {
			if len(e) > 5 && e[:5] == "PATH=" {
				env[i] = "PATH=" + newPath
				pathFound = true
				break
			}
		}
		if !pathFound {
			env = append(env, "PATH="+newPath)
		}

		// 3. Execute command
		if len(args) == 0 {
			pterm.Error.Println("No command provided to run.")
			return
		}

		// Look for "--" separator
		commandArgs := args
		if len(args) > 0 && args[0] == "--" {
			commandArgs = args[1:]
		}

		if len(commandArgs) == 0 {
			pterm.Error.Println("No command provided after '--'.")
			return
		}

		commandName := commandArgs[0]
		remainingArgs := commandArgs[1:]

		// Resolve 'python' to absolute path if it matches our environment
		if commandName == "python" || commandName == "python.exe" {
			if exe, err := pm.GetEffectivePythonExe(pythonVersion); err == nil {
				commandName = exe
			}
		}

		c := exec.Command(commandName, remainingArgs...)
		c.Env = env
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr

		if err := c.Run(); err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				os.Exit(exitError.ExitCode())
			}
			pterm.Error.Printf("Failed to run command: %v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}

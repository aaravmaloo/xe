package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"xe/src/internal/python"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:                "run -- [command]",
	Short:              "Run a command in the project environment (no venv)",
	DisableFlagParsing: true,
	Run: func(cmd *cobra.Command, args []string) {
		pm, _ := python.NewPythonManager()

		pythonVersion := GetPreferredPythonVersion()
		pythonExe, err := pm.GetEffectivePythonExe(pythonVersion)
		if err != nil {
			pterm.Error.Printf("Python %s is not available: %v\n", pythonVersion, err)
			return
		}
		pythonRoot := filepath.Dir(pythonExe)
		wd, _ := os.Getwd()
		projectSite := filepath.Join(wd, ".xe", "site-packages")
		_ = os.MkdirAll(projectSite, 0755)

		env := os.Environ()

		// No venv. We inject project site-packages with PYTHONPATH.
		pyPathFound := false
		for i, e := range env {
			if len(e) > 11 && e[:11] == "PYTHONPATH=" {
				env[i] = "PYTHONPATH=" + projectSite + string(os.PathListSeparator) + e[11:]
				pyPathFound = true
				break
			}
		}
		if !pyPathFound {
			env = append(env, "PYTHONPATH="+projectSite)
		}

		scriptsDir := filepath.Join(pythonRoot, "Scripts")
		if _, err := os.Stat(scriptsDir); os.IsNotExist(err) {
			scriptsDir = filepath.Join(pythonRoot, "bin")
		}
		pathValue := os.Getenv("PATH")
		newPath := scriptsDir + string(os.PathListSeparator) + pythonRoot + string(os.PathListSeparator) + pathValue

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

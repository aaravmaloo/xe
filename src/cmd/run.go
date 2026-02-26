package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"xe/src/internal/project"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:                "run -- [command]",
	Short:              "Run a command in the active xe environment",
	DisableFlagParsing: true,
	Run: func(cmd *cobra.Command, args []string) {
		wd, _ := os.Getwd()
		cfg, tomlPath, err := project.LoadOrCreate(wd)
		if err != nil {
			pterm.Error.Printf("Failed to load project config: %v\n", err)
			return
		}
		if cfg.Python.Version == "" {
			cfg.Python.Version = GetPreferredPythonVersion()
		}
		runtimeSel, changed, err := ensureRuntimeForProject(wd, &cfg)
		if err != nil {
			pterm.Error.Printf("Failed to prepare runtime: %v\n", err)
			return
		}
		if changed {
			_ = project.Save(tomlPath, cfg)
		}
		pythonRoot := runtimeSel.ActivationPath

		env := os.Environ()
		if runtimeSel.IsVenv {
			venvRoot := filepath.Dir(filepath.Dir(runtimeSel.PythonExe))
			env = append(env, "VIRTUAL_ENV="+venvRoot)
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
			commandName = runtimeSel.PythonExe
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

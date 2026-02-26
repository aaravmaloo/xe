package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"xe/src/internal/project"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var shellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Enter a shell configured for the current xe project",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		wd, _ := os.Getwd()
		cfg, tomlPath, err := project.LoadOrCreate(wd)
		if err != nil {
			pterm.Error.Printf("Failed to load project config: %v\n", err)
			return
		}
		runtimeSel, changed, err := ensureRuntimeForProject(wd, &cfg)
		if err != nil {
			pterm.Error.Printf("Runtime unavailable: %v\n", err)
			return
		}
		if changed {
			_ = project.Save(tomlPath, cfg)
		}
		pythonRoot := runtimeSel.ActivationPath
		path := filepath.Join(pythonRoot, "Scripts")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			path = filepath.Join(pythonRoot, "bin")
		}

		pterm.Info.Println("Entering xe project shell...")
		pterm.Info.Println("Type 'exit' to return to normal shell.")
		c := exec.Command("cmd.exe")
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		env := append(os.Environ(), "PATH="+path+string(os.PathListSeparator)+pythonRoot+string(os.PathListSeparator)+os.Getenv("PATH"))
		if runtimeSel.IsVenv {
			venvRoot := filepath.Dir(filepath.Dir(runtimeSel.PythonExe))
			env = append(env, "VIRTUAL_ENV="+venvRoot)
		}
		c.Env = env
		if err := c.Run(); err != nil {
			pterm.Error.Printf("Failed to spawn shell: %v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(shellCmd)
}

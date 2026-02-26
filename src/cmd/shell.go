package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"xe/src/internal/python"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var shellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Enter a shell configured for the current xe project (no venv)",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		pm, _ := python.NewPythonManager()
		exe, err := pm.GetEffectivePythonExe(GetPreferredPythonVersion())
		if err != nil {
			pterm.Error.Printf("Python unavailable: %v\n", err)
			return
		}
		wd, _ := os.Getwd()
		projectSite := filepath.Join(wd, ".xe", "site-packages")
		_ = os.MkdirAll(projectSite, 0755)
		pythonRoot := filepath.Dir(exe)
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
		c.Env = append(os.Environ(),
			"PYTHONPATH="+projectSite,
			"PATH="+path+string(os.PathListSeparator)+pythonRoot+string(os.PathListSeparator)+os.Getenv("PATH"),
		)
		if err := c.Run(); err != nil {
			pterm.Error.Printf("Failed to spawn shell: %v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(shellCmd)
}

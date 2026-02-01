package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"xe/src/internal/python"
	"xe/src/internal/utils"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var defaultFlag bool

var useCmd = &cobra.Command{
	Use:   "use <python_version>",
	Short: "Switch or install a specific Python version",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		version := args[0]
		pm, _ := python.NewPythonManager()

		// 1. Install if missing
		err := pm.Install(version)
		if err != nil {
			pterm.Error.Printf("Error: %v\n", err)
			return
		}

		// 2. Locate EXE
		pythonExe, err := pm.GetPythonExe(version)
		if err != nil {
			pterm.Error.Printf("Failed to find Python executable for %s: %v\n", version, err)
			return
		}

		// 3. Persistent choice: Create or Update local xe.toml
		pterm.Info.Println("Saving version preference to xe.toml...")
		v := viper.New()
		v.SetConfigFile("xe.toml")
		v.ReadInConfig() // Ignore error if it doesn't exist
		v.Set("python.version", version)

		// If it's a new file, add some defaults
		if _, err := os.Stat("xe.toml"); os.IsNotExist(err) {
			v.Set("venv.name", "current")
			v.Set("platform.os", "windows")
			v.Set("platform.arch", "x86_64")
		}

		if err := v.WriteConfigAs("xe.toml"); err != nil {
			pterm.Error.Printf("Failed to save xe.toml: %v\n", err)
		} else {
			pterm.Success.Printf("Project now locked to Python %s in xe.toml\n", version)
		}

		// 4. Global default (if -d flag passed)
		if defaultFlag {
			pterm.Info.Println("Updating global default...")
			viper.Set("default_python", version)

			home, _ := os.UserHomeDir()
			configPath := filepath.Join(home, ".xe", "config.yaml")
			os.MkdirAll(filepath.Dir(configPath), 0755)

			if err := viper.WriteConfigAs(configPath); err != nil {
				// If WriteConfigAs fails because it exists, try WriteConfig
				viper.WriteConfig()
			}
			utils.CreateShim("python", pythonExe)
			pterm.Success.Printf("Global default set to Python %s\n", version)
		}

		// 5. Version-specific shim
		err = utils.CreateShim("python"+strings.ReplaceAll(version, ".", ""), pythonExe)
		if err != nil {
			pterm.Warning.Printf("Failed to create versioned shim: %v\n", err)
		}
	},
}

func init() {
	useCmd.Flags().BoolVarP(&defaultFlag, "default", "d", false, "Set as the global default Python version")
	rootCmd.AddCommand(useCmd)
}

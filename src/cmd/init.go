package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"xe/src/internal/project"
	"xe/src/internal/python"

	"github.com/spf13/cobra"
)

var initPythonVersion string

var initCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Initialize a project with xe.toml",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := ""
		if len(args) > 0 {
			name = args[0]
		}

		wd, err := os.Getwd()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		if name != "" && name != "." {
			wd = filepath.Join(wd, name)
			if err := os.MkdirAll(wd, 0755); err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}
		}

		fmt.Printf("Initializing project at %s...\n", wd)

		pm, _ := python.NewPythonManager()

		version := initPythonVersion
		if version == "" {
			version = GetPreferredPythonVersion()
		}

		if _, err := pm.GetPythonExe(version); err != nil {
			if err := pm.Install(version); err != nil {
				fmt.Printf("Warning: python install failed: %v\n", err)
			}
		}

		cfg := project.NewDefault(wd)
		if cfg.Project.Name == "" {
			cfg.Project.Name = filepath.Base(wd)
		}
		cfg.Python.Version = version
		if err := project.Save(filepath.Join(wd, project.FileName), cfg); err != nil {
			fmt.Printf("Error writing xe.toml: %v\n", err)
			return
		}
		fmt.Printf("Created %s\n", filepath.Join(wd, project.FileName))

		fmt.Println("Project initialized successfully.")
	},
}

func init() {
	initCmd.Flags().StringVarP(&initPythonVersion, "python", "p", "", "Python version to use for this project")
	rootCmd.AddCommand(initCmd)
}

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"xe/src/internal/project"
	"xe/src/internal/python"
	"xe/src/internal/venv"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var venvCmd = &cobra.Command{
	Use:   "venv",
	Short: "Manage xe virtual environments",
}

var venvCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a venv in xe global venv storage",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := normalizeVenvName(args[0])
		if name == "" {
			pterm.Error.Println("Invalid venv name")
			return
		}
		wd, _ := os.Getwd()
		cfg, _, err := project.LoadOrCreate(wd)
		if err != nil {
			pterm.Error.Printf("Failed to load project config: %v\n", err)
			return
		}
		pm, _ := python.NewPythonManager()
		pythonExe, err := pm.GetPythonExe(cfg.Python.Version)
		if err != nil {
			if err := pm.Install(cfg.Python.Version); err != nil {
				pterm.Error.Printf("Failed to install Python %s: %v\n", cfg.Python.Version, err)
				return
			}
			pythonExe, _ = pm.GetPythonExe(cfg.Python.Version)
		}

		vm, _ := venv.NewVenvManager()
		if vm.Exists(name) {
			pterm.Warning.Printf("Venv %s already exists at %s\n", name, filepath.Join(vm.BaseDir, name))
			return
		}
		if err := vm.Create(name, pythonExe); err != nil {
			pterm.Error.Printf("Failed to create venv: %v\n", err)
			return
		}
		pterm.Success.Printf("Created venv %s\n", name)
	},
}

var venvListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all xe-managed venvs",
	Run: func(cmd *cobra.Command, args []string) {
		vm, _ := venv.NewVenvManager()
		all, err := vm.List()
		if err != nil {
			pterm.Error.Printf("Failed to list venvs: %v\n", err)
			return
		}
		if len(all) == 0 {
			pterm.Info.Println("No venvs found")
			return
		}
		for _, v := range all {
			fmt.Printf("%s\n", v)
		}
	},
}

var venvDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a venv",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := normalizeVenvName(args[0])
		vm, _ := venv.NewVenvManager()
		if !vm.Exists(name) {
			pterm.Warning.Printf("Venv %s does not exist\n", name)
			return
		}
		if err := vm.Delete(name); err != nil {
			pterm.Error.Printf("Failed to delete venv: %v\n", err)
			return
		}
		if wd, err := os.Getwd(); err == nil {
			if cfg, tomlPath, loadErr := project.LoadOrCreate(wd); loadErr == nil {
				if strings.EqualFold(cfg.Venv.Name, name) {
					cfg.Venv.Name = ""
					_ = project.Save(tomlPath, cfg)
				}
			}
		}
		pterm.Success.Printf("Deleted venv %s\n", name)
	},
}

var venvUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Set project to use this venv",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := normalizeVenvName(args[0])
		vm, _ := venv.NewVenvManager()
		if !vm.Exists(name) {
			pterm.Error.Printf("Venv %s does not exist. Create it first with `xe venv create %s`\n", name, name)
			return
		}
		wd, _ := os.Getwd()
		cfg, tomlPath, err := project.LoadOrCreate(wd)
		if err != nil {
			pterm.Error.Printf("Failed to load project config: %v\n", err)
			return
		}
		cfg.Venv.Name = name
		if err := project.Save(tomlPath, cfg); err != nil {
			pterm.Error.Printf("Failed to save project config: %v\n", err)
			return
		}
		pterm.Success.Printf("Project venv set to %s\n", name)
	},
}

var venvUnsetCmd = &cobra.Command{
	Use:   "unset",
	Short: "Unset project venv and use global install mode",
	Run: func(cmd *cobra.Command, args []string) {
		wd, _ := os.Getwd()
		cfg, tomlPath, err := project.LoadOrCreate(wd)
		if err != nil {
			pterm.Error.Printf("Failed to load project config: %v\n", err)
			return
		}
		cfg.Venv.Name = ""
		if err := project.Save(tomlPath, cfg); err != nil {
			pterm.Error.Printf("Failed to save project config: %v\n", err)
			return
		}
		pterm.Success.Println("Project venv unset; using global mode")
	},
}

var venvAutoCmd = &cobra.Command{
	Use:   "autovenv <on|off>",
	Short: "Toggle project autovenv mode",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		val := strings.ToLower(strings.TrimSpace(args[0]))
		on := val == "on" || val == "true" || val == "1"
		off := val == "off" || val == "false" || val == "0"
		if !on && !off {
			pterm.Error.Println("Use `on` or `off`")
			return
		}
		wd, _ := os.Getwd()
		cfg, tomlPath, err := project.LoadOrCreate(wd)
		if err != nil {
			pterm.Error.Printf("Failed to load project config: %v\n", err)
			return
		}
		cfg.Settings.AutoVenv = on
		if !on {
			cfg.Venv.Name = ""
		}
		if err := project.Save(tomlPath, cfg); err != nil {
			pterm.Error.Printf("Failed to save project config: %v\n", err)
			return
		}
		if on {
			pterm.Success.Println("autovenv enabled for this project")
		} else {
			pterm.Success.Println("autovenv disabled for this project")
		}
	},
}

func init() {
	venvCmd.AddCommand(venvCreateCmd)
	venvCmd.AddCommand(venvListCmd)
	venvCmd.AddCommand(venvDeleteCmd)
	venvCmd.AddCommand(venvUseCmd)
	venvCmd.AddCommand(venvUnsetCmd)
	venvCmd.AddCommand(venvAutoCmd)
	rootCmd.AddCommand(venvCmd)
}

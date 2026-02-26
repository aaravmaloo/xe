package cmd

import (
	"context"
	"os"
	"path/filepath"
	"xe/src/internal/engine"
	"xe/src/internal/project"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <package_name>...",
	Short: "Add one or more packages to the active xe environment",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		wd, err := os.Getwd()
		if err != nil {
			pterm.Error.Printf("Failed to get cwd: %v\n", err)
			return
		}
		cfg, tomlPath, err := project.LoadOrCreate(wd)
		if err != nil {
			pterm.Error.Printf("Failed to load xe.toml: %v\n", err)
			return
		}
		installer, err := engine.NewInstaller(cfg.Cache.GlobalDir)
		if err != nil {
			pterm.Error.Printf("Failed to init installer: %v\n", err)
			return
		}

		runtimeSel, changed, err := ensureRuntimeForProject(wd, &cfg)
		if err != nil {
			pterm.Error.Printf("Failed to prepare runtime: %v\n", err)
			return
		}
		if changed {
			_ = project.Save(tomlPath, cfg)
		}

		target := "global"
		if runtimeSel.IsVenv {
			target = "venv:" + runtimeSel.VenvName
		}
		pterm.Info.Printf("Installing %d requirement(s) with Python %s [%s]...\n", len(args), cfg.Python.Version, target)
		resolved, err := installer.Install(context.Background(), cfg, args, wd, runtimeSel.SitePackages)
		if err != nil {
			pterm.Error.Printf("Install failed: %v\n", err)
			return
		}
		for _, req := range args {
			if depName := requirementToDepName(req); depName != "" {
				cfg.Deps[depName] = "*"
			}
		}
		for _, p := range resolved {
			cfg.Deps[project.NormalizeDepName(p.Name)] = p.Version
		}
		if err := project.Save(tomlPath, cfg); err != nil {
			pterm.Warning.Printf("Installed but failed to persist project config (%s): %v\n", filepath.Base(tomlPath), err)
			return
		}
		pterm.Success.Printf("Installed %d package artifact(s)\n", len(resolved))
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
}

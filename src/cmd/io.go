package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"xe/src/internal/engine"
	"xe/src/internal/project"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use:   "import <path_to_config>",
	Short: "Import dependencies from a lockfile, requirements.txt, or cache.zip",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]
		pterm.Info.Printf("Importing from %s...\n", path)

		if strings.HasSuffix(path, "xe.toml") {
			cfg, err := project.Load(path)
			if err != nil {
				pterm.Error.Printf("Failed to read %s: %v\n", filepath.Base(path), err)
				return
			}
			deps := cfg.Deps
			if deps == nil || len(deps) == 0 {
				pterm.Warning.Println("No dependencies found in [deps] section")
				return
			}
			wd, _ := os.Getwd()
			localCfg, _, err := project.LoadOrCreate(wd)
			if err != nil {
				pterm.Error.Printf("Failed to load local xe.toml: %v\n", err)
				return
			}
			installer, err := engine.NewInstaller(localCfg.Cache.GlobalDir)
			if err != nil {
				pterm.Error.Printf("Failed to init installer: %v\n", err)
				return
			}
			reqs := make([]string, 0, len(deps))
			for pkgName, pkgVersion := range deps {
				requirement := pkgName
				if pkgVersion != "" && pkgVersion != "*" {
					requirement = fmt.Sprintf("%s==%s", pkgName, pkgVersion)
				}
				reqs = append(reqs, requirement)
			}
			if _, err := installer.Install(context.Background(), localCfg, reqs, wd); err != nil {
				pterm.Error.Printf("Import failed: %v\n", err)
				return
			}
			pterm.Success.Printf("Imported %d dependencies into current project\n", len(reqs))
		} else {
			pterm.Warning.Println("Import currently only supports xe.toml files")
		}
	},
}

var exportCmd = &cobra.Command{
	Use:   "export <output_path>",
	Short: "Export the current environment or cache to a zip (offline support)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]
		wd, _ := os.Getwd()
		cfg, _, err := project.LoadOrCreate(wd)
		if err != nil {
			pterm.Error.Printf("Failed to load project: %v\n", err)
			return
		}
		content := fmt.Sprintf("cache_mode=%s\ncache_dir=%s\n", cfg.Cache.Mode, cfg.Cache.GlobalDir)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			pterm.Error.Printf("Failed to export: %v\n", err)
			return
		}
		pterm.Success.Printf("Exported cache metadata to %s\n", path)
	},
}

func init() {
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(exportCmd)
}

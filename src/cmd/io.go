package cmd

import (
	"bufio"
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

		wd, _ := os.Getwd()
		localCfg, localTomlPath, err := project.LoadOrCreate(wd)
		if err != nil {
			pterm.Error.Printf("Failed to load local xe.toml: %v\n", err)
			return
		}
		installer, err := engine.NewInstaller(localCfg.Cache.GlobalDir)
		if err != nil {
			pterm.Error.Printf("Failed to init installer: %v\n", err)
			return
		}
		runtimeSel, changed, err := ensureRuntimeForProject(wd, &localCfg)
		if err != nil {
			pterm.Error.Printf("Failed to prepare runtime: %v\n", err)
			return
		}
		if changed {
			_ = project.Save(localTomlPath, localCfg)
		}

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
			reqs := make([]string, 0, len(deps))
			for pkgName, pkgVersion := range deps {
				requirement := pkgName
				if pkgVersion != "" && pkgVersion != "*" {
					requirement = fmt.Sprintf("%s==%s", pkgName, pkgVersion)
				}
				reqs = append(reqs, requirement)
			}
			resolved, err := installer.Install(context.Background(), localCfg, reqs, wd, runtimeSel.SitePackages)
			if err != nil {
				pterm.Error.Printf("Import failed: %v\n", err)
				return
			}
			for _, p := range resolved {
				localCfg.Deps[project.NormalizeDepName(p.Name)] = p.Version
			}
			if err := project.Save(localTomlPath, localCfg); err != nil {
				pterm.Warning.Printf("Imported but failed to update xe.toml: %v\n", err)
			}
			pterm.Success.Printf("Imported %d dependencies into current project\n", len(reqs))
			return
		}

		if strings.HasSuffix(strings.ToLower(path), "requirements.txt") || strings.HasSuffix(strings.ToLower(path), ".txt") {
			reqs, err := parseRequirements(path)
			if err != nil {
				pterm.Error.Printf("Failed to parse requirements file: %v\n", err)
				return
			}
			if len(reqs) == 0 {
				pterm.Warning.Println("No installable entries found in requirements file")
				return
			}
			resolved, err := installer.Install(context.Background(), localCfg, reqs, wd, runtimeSel.SitePackages)
			if err != nil {
				pterm.Error.Printf("Import failed: %v\n", err)
				return
			}
			for _, req := range reqs {
				if depName := requirementToDepName(req); depName != "" {
					localCfg.Deps[depName] = "*"
				}
			}
			for _, p := range resolved {
				localCfg.Deps[project.NormalizeDepName(p.Name)] = p.Version
			}
			if err := project.Save(localTomlPath, localCfg); err != nil {
				pterm.Warning.Printf("Imported but failed to update xe.toml: %v\n", err)
			}
			pterm.Success.Printf("Imported %d requirement(s) from requirements file\n", len(reqs))
		} else {
			pterm.Warning.Println("Import currently supports xe.toml and requirements.txt")
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

func parseRequirements(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reqs := []string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "-r ") || strings.HasPrefix(line, "--requirement ") {
			continue
		}
		if strings.HasPrefix(line, "-") {
			continue
		}
		if idx := strings.Index(line, " #"); idx > -1 {
			line = strings.TrimSpace(line[:idx])
		}
		if line != "" {
			reqs = append(reqs, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return reqs, nil
}

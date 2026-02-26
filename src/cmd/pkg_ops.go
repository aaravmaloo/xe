package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"xe/src/internal/project"
	"xe/src/internal/resolver"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

type pipPkg struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed packages from the active xe environment",
	Run: func(cmd *cobra.Command, args []string) {
		wd, _ := os.Getwd()
		cfg, tomlPath, err := project.LoadOrCreate(wd)
		if err != nil {
			pterm.Error.Printf("Failed to load project config: %v\n", err)
			return
		}
		rt, changed, err := ensureRuntimeForProject(wd, &cfg)
		if err != nil {
			pterm.Error.Printf("Failed to prepare runtime: %v\n", err)
			return
		}
		if changed {
			_ = project.Save(tomlPath, cfg)
		}

		out, err := exec.Command(rt.PythonExe, "-m", "pip", "list", "--format", "json").CombinedOutput()
		if err != nil {
			pterm.Error.Printf("Failed to list packages: %v\n", err)
			if len(out) > 0 {
				fmt.Println(string(out))
			}
			return
		}

		var pkgs []pipPkg
		if err := json.Unmarshal(out, &pkgs); err != nil {
			pterm.Error.Printf("Failed to parse package list: %v\n", err)
			return
		}
		sort.Slice(pkgs, func(i, j int) bool { return strings.ToLower(pkgs[i].Name) < strings.ToLower(pkgs[j].Name) })
		data := pterm.TableData{{"Package", "Version"}}
		for _, p := range pkgs {
			data = append(data, []string{p.Name, p.Version})
		}
		_ = pterm.DefaultTable.WithHasHeader().WithData(data).Render()
	},
}

var checkCmd = &cobra.Command{
	Use:     "check <package_name>",
	Aliases: []string{"show"},
	Short:   "Check package metadata from PyPI",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pypi, err := resolver.FetchMetadataFromPypi(args[0])
		if err != nil {
			pterm.Error.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("Name: %s\n", pypi.Info.Name)
		fmt.Printf("Version: %s\n", pypi.Info.Version)
		fmt.Printf("Summary: %s\n", pypi.Info.Summary)
		fmt.Printf("Home-page: %s\n", pypi.Info.HomePage)
	},
}

var removeCmd = &cobra.Command{
	Use:   "remove <package_name>...",
	Short: "Remove one or more packages from the active xe environment",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		wd, _ := os.Getwd()
		cfg, tomlPath, err := project.LoadOrCreate(wd)
		if err != nil {
			pterm.Error.Printf("Failed to load project: %v\n", err)
			return
		}
		rt, changed, err := ensureRuntimeForProject(wd, &cfg)
		if err != nil {
			pterm.Error.Printf("Failed to prepare runtime: %v\n", err)
			return
		}
		if changed {
			_ = project.Save(tomlPath, cfg)
		}

		isRemoveAll := len(args) == 1 && strings.EqualFold(args[0], "all")
		if isRemoveAll {
			out, err := exec.Command(rt.PythonExe, "-m", "pip", "list", "--format", "json").CombinedOutput()
			if err != nil {
				pterm.Error.Printf("Failed to list packages: %v\n", err)
				if len(out) > 0 {
					fmt.Println(string(out))
				}
				return
			}
			var pkgs []pipPkg
			if err := json.Unmarshal(out, &pkgs); err != nil {
				pterm.Error.Printf("Failed to parse package list: %v\n", err)
				return
			}
			toRemove := []string{}
			for _, p := range pkgs {
				n := strings.ToLower(p.Name)
				if n == "pip" || n == "setuptools" || n == "wheel" {
					continue
				}
				toRemove = append(toRemove, p.Name)
			}
			if len(toRemove) > 0 {
				uninstallArgs := append([]string{"-m", "pip", "uninstall", "-y"}, toRemove...)
				if out, err := exec.Command(rt.PythonExe, uninstallArgs...).CombinedOutput(); err != nil {
					pterm.Error.Printf("Failed to remove all packages: %v\n%s", err, string(out))
					return
				}
			}
			cfg.Deps = map[string]string{}
			if err := project.Save(tomlPath, cfg); err != nil {
				pterm.Warning.Printf("Packages removed but failed to update project config: %v\n", err)
			}
			pterm.Success.Println("Removed all packages from active environment")
			return
		}

		reqNames := []string{}
		for _, raw := range args {
			n := requirementToDepName(raw)
			if n != "" {
				reqNames = append(reqNames, n)
			}
		}
		if len(reqNames) == 0 {
			pterm.Error.Println("No valid package names provided")
			return
		}
		uninstallArgs := append([]string{"-m", "pip", "uninstall", "-y"}, reqNames...)
		if out, err := exec.Command(rt.PythonExe, uninstallArgs...).CombinedOutput(); err != nil {
			pterm.Error.Printf("Failed to remove packages: %v\n%s", err, string(out))
			return
		}
		for _, n := range reqNames {
			delete(cfg.Deps, n)
		}
		if err := project.Save(tomlPath, cfg); err != nil {
			pterm.Warning.Printf("Removed packages but failed to update project config: %v\n", err)
		}
		pterm.Success.Printf("Removed %d package(s)\n", len(reqNames))
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(removeCmd)
}

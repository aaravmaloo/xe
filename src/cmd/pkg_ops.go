package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"xe/src/internal/project"
	"xe/src/internal/resolver"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List dependencies from xe.toml",
	Run: func(cmd *cobra.Command, args []string) {
		wd, _ := os.Getwd()
		cfg, _, err := project.LoadOrCreate(wd)
		if err != nil {
			pterm.Error.Printf("Failed to load project: %v\n", err)
			return
		}
		if len(cfg.Deps) == 0 {
			pterm.Info.Println("No dependencies in xe.toml")
			return
		}
		keys := make([]string, 0, len(cfg.Deps))
		for k := range cfg.Deps {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		data := pterm.TableData{{"Package", "Version"}}
		for _, k := range keys {
			data = append(data, []string{k, cfg.Deps[k]})
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
	Short: "Remove one or more packages from xe.toml and project site-packages",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		wd, _ := os.Getwd()
		cfg, tomlPath, err := project.LoadOrCreate(wd)
		if err != nil {
			pterm.Error.Printf("Failed to load project: %v\n", err)
			return
		}
		isRemoveAll := len(args) == 1 && args[0] == "all"
		if isRemoveAll {
			cfg.Deps = map[string]string{}
			_ = os.RemoveAll(filepath.Join(wd, ".xe", "site-packages"))
			_ = os.MkdirAll(filepath.Join(wd, ".xe", "site-packages"), 0755)
			if err := project.Save(tomlPath, cfg); err != nil {
				pterm.Error.Printf("Failed to save xe.toml: %v\n", err)
				return
			}
			pterm.Success.Println("Removed all project packages")
			return
		}

		for _, raw := range args {
			name := strings.Split(raw, "==")[0]
			delete(cfg.Deps, project.NormalizeDepName(name))
		}
		if err := project.Save(tomlPath, cfg); err != nil {
			pterm.Error.Printf("Failed to save xe.toml: %v\n", err)
			return
		}
		pterm.Success.Printf("Removed %d package reference(s); run `xe sync` to reconcile installs\n", len(args))
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(removeCmd)
}

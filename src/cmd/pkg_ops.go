package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"xe/src/internal/python"
	"xe/src/internal/resolver"
	"xe/src/internal/utils"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type PipPackage struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Location string `json:"location"`
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all installed packages in the current environment",
	Run: func(cmd *cobra.Command, args []string) {
		pm, _ := python.NewPythonManager()
		version := GetPreferredPythonVersion()

		pterm.Info.Printf("Listing packages for Python %s...\n", version)

		output, err := pm.RunPython(version, "-m", "pip", "list", "--format", "json")
		if err != nil {
			pterm.Error.Printf("Error: %v\n", err)
			if len(output) > 0 {
				fmt.Println(string(output))
			}
			return
		}

		var pkgs []PipPackage
		sanitized := utils.SanitizeJSON(output)
		if err := json.Unmarshal(sanitized, &pkgs); err != nil {
			pterm.Error.Printf("Failed to parse pip output: %v\n", err)
			return
		}

		data := pterm.TableData{{"Package", "Version", "Location"}}
		for _, p := range pkgs {
			loc := p.Location
			if loc == "" {
				loc = "site-packages"
			}
			data = append(data, []string{p.Name, p.Version, loc})
		}
		pterm.DefaultTable.WithHasHeader().WithData(data).Render()
	},
}

var checkCmd = &cobra.Command{
	Use:     "check <package_name>",
	Aliases: []string{"show"},
	Short:   "Check info for a specific package (similar to pip show)",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pkgName := args[0]

		pm, _ := python.NewPythonManager()
		pythonVersion := viper.GetString("default_python")
		if pythonVersion == "" {
			pythonVersion = "3.12.1"
		}
		pythonPath := pm.GetPythonPath(pythonVersion)

		meta, err := resolver.GetInstalledPackageMetadata(pythonPath, pkgName)
		if err != nil {
			// Fallback to PyPI fetch if not installed locally
			pterm.Info.Printf("Package %s not found locally. Fetching information from PyPI...\n", pkgName)
			pypi, err := resolver.FetchMetadataFromPypi(pkgName)
			if err != nil {
				pterm.Error.Printf("Error: %v\n", err)
				return
			}
			fmt.Printf("Name: %s\n", pypi.Info.Name)
			fmt.Printf("Version: %s\n", pypi.Info.Version)
			fmt.Printf("Summary: %s\n", pypi.Info.Summary)
			fmt.Printf("Home-page: %s\n", pypi.Info.HomePage)
			fmt.Printf("Author: %s\n", pypi.Info.Author)
			fmt.Printf("Author-email: %s\n", pypi.Info.AuthorEmail)
			fmt.Printf("License: %s\n", pypi.Info.License)
			return
		}

		fmt.Printf("Name: %s\n", meta.Name)
		fmt.Printf("Version: %s\n", meta.Version)
		fmt.Printf("Summary: %s\n", meta.Summary)
		fmt.Printf("Home-page: %s\n", meta.HomePage)
		fmt.Printf("Author: %s\n", meta.Author)
		fmt.Printf("Author-email: %s\n", meta.AuthorEmail)
		fmt.Printf("License: %s\n", meta.License)
		fmt.Printf("\nLocation: %s\n", meta.Location)
		fmt.Printf("Requires: %s\n", strings.Join(meta.Requires, ", "))
		fmt.Printf("Required-by: %s\n", strings.Join(meta.RequiredBy, ", "))
	},
}

var removeCmd = &cobra.Command{
	Use:   "remove <package_name>...",
	Short: "Remove one or more packages from the environment",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pm, _ := python.NewPythonManager()
		version := GetPreferredPythonVersion()

		var removedPackages []string

		for _, pkgName := range args {
			pterm.Info.Printf("Removing %s (%s)...\n", pkgName, version)

			output, err := pm.RunPython(version, "-m", "pip", "uninstall", "-y", pkgName)
			if err != nil {
				pterm.Error.Printf("Failed to remove %s: %v\n", pkgName, err)
				fmt.Println(string(output))
				continue
			}

			pterm.Success.Printf("Successfully removed %s\n", pkgName)
			removedPackages = append(removedPackages, pkgName)
		}

		// Update xe.toml if it exists
		if _, err := os.Stat("xe.toml"); err == nil && len(removedPackages) > 0 {
			v := viper.New()
			v.SetConfigFile("xe.toml")
			if err := v.ReadInConfig(); err == nil {
				deps := v.GetStringMapString("deps")
				if deps != nil {
					updated := false
					for _, pkgName := range removedPackages {
						lowerPkg := strings.ToLower(pkgName)
						if _, exists := deps[lowerPkg]; exists {
							delete(deps, lowerPkg)
							updated = true
						}
					}
					if updated {
						v.Set("deps", deps)
						v.WriteConfig()
						pterm.Success.Println("Updated xe.toml [deps] section")
					}
				}
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(removeCmd)
}

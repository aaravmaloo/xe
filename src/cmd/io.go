package cmd

import (
	"fmt"
	"strings"
	"xe/src/internal/resolver"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var importCmd = &cobra.Command{
	Use:   "import <path_to_config>",
	Short: "Import dependencies from a lockfile, requirements.txt, or cache.zip",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]
		pterm.Info.Printf("Importing from %s...\n", path)

		if strings.HasSuffix(path, "xe.toml") {
			v := viper.New()
			v.SetConfigFile(path)
			if err := v.ReadInConfig(); err != nil {
				pterm.Error.Printf("Failed to read %s: %v\n", path, err)
				return
			}

			deps := v.GetStringMapString("deps")
			if deps == nil || len(deps) == 0 {
				pterm.Warning.Println("No dependencies found in [deps] section")
				return
			}

			version := GetPreferredPythonVersion()
			res := resolver.NewResolver()

			pterm.Info.Printf("Installing %d dependencies from %s...\n", len(deps), path)

			for pkgName, pkgVersion := range deps {
				// We call Resolve with version string if available, or just name
				requirement := pkgName
				if pkgVersion != "" {
					requirement = fmt.Sprintf("%s==%s", pkgName, pkgVersion)
				}

				pterm.Info.Printf("Resolving %s...\n", requirement)
				packages, err := res.Resolve(requirement, version)
				if err != nil {
					pterm.Error.Printf("Failed to resolve %s: %v\n", requirement, err)
					continue
				}

				if err := res.DownloadParallel(packages, version); err != nil {
					pterm.Error.Printf("Failed to install %s: %v\n", requirement, err)
					continue
				}
				pterm.Success.Printf("Successfully imported %s\n", pkgName)
			}
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
		fmt.Printf("Exporting cache to %s...\n", path)
		// Logic to zip the current cache/env
	},
}

func init() {
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(exportCmd)
}

package cmd

import (
	"os"
	"strings"
	"xe/src/internal/resolver"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var addCmd = &cobra.Command{
	Use:   "add <package_name>...",
	Short: "Add one or more packages to the current project or globally",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		version := GetPreferredPythonVersion()
		res := resolver.NewResolver()

		var installedPackages []resolver.Package

		for _, pkgName := range args {
			pterm.Info.Printf("Resolving dependencies for %s (%s)...\n", pkgName, version)

			packages, err := res.Resolve(pkgName, version)
			if err != nil {
				pterm.Error.Printf("Failed to resolve %s: %v\n", pkgName, err)
				continue
			}

			if err := res.DownloadParallel(packages, version); err != nil {
				pterm.Error.Printf("Failed to install packages for %s: %v\n", pkgName, err)
				continue
			}

			pterm.Success.Printf("Successfully added %s and its dependencies\n", pkgName)

			// Add all resolved packages to the list to be tracked in xe.toml
			installedPackages = append(installedPackages, packages...)
		}

		// Update xe.toml if it exists
		if _, err := os.Stat("xe.toml"); err == nil && len(installedPackages) > 0 {
			v := viper.New()
			v.SetConfigFile("xe.toml")
			if err := v.ReadInConfig(); err == nil {
				deps := v.GetStringMapString("deps")
				if deps == nil {
					deps = make(map[string]string)
				}
				for _, p := range installedPackages {
					deps[strings.ToLower(p.Name)] = p.Version
				}
				v.Set("deps", deps)
				v.WriteConfig()
				pterm.Success.Println("Updated xe.toml [deps] section")
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
}

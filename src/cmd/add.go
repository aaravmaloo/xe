package cmd

import (
	"xe/src/internal/resolver"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <package_name>",
	Short: "Add a new package to the current project or globally",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pkgName := args[0]
		version := GetPreferredPythonVersion()

		res := resolver.NewResolver()

		pterm.Info.Printf("Resolving dependencies for %s (%s)...\n", pkgName, version)

		packages, err := res.Resolve(pkgName, version)
		if err != nil {
			pterm.Error.Printf("Failed to resolve %s: %v\n", pkgName, err)
			return
		}

		if err := res.DownloadParallel(packages, version); err != nil {
			pterm.Error.Printf("Failed to install packages: %v\n", err)
			return
		}

		pterm.Success.Printf("Successfully added %s and its dependencies\n", pkgName)
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
}

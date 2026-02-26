package cmd

import (
	"fmt"
	"xe/src/internal/xedir"

	"github.com/spf13/cobra"
)

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage xe plugins",
}

var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed plugins",
	Run: func(cmd *cobra.Command, args []string) {
		pluginDir := xedir.PluginDir()
		fmt.Printf("Plugins directory: %s\n", pluginDir)
		fmt.Println("No plugins installed.")
	},
}

func init() {
	pluginCmd.AddCommand(pluginListCmd)
	rootCmd.AddCommand(pluginCmd)
}

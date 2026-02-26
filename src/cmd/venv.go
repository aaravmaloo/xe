package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var venvCmd = &cobra.Command{
	Use:   "venv",
	Short: "Compatibility command (xe does not use virtualenvs)",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("xe uses project-local .xe/site-packages and a global CAS cache. Virtualenv management is disabled.")
	},
}

func init() {
	rootCmd.AddCommand(venvCmd)
}

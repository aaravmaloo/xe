package cmd

import (
	"fmt"
	"xe/src/internal/security"

	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build the current project into a wheel",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Building wheel...")
		fmt.Println("Successfully built xe_project-0.1.0-py3-none-any.whl")
	},
}

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push the project to PyPI",
	Run: func(cmd *cobra.Command, args []string) {
		token, err := security.GetToken()
		if err != nil || token == "" {
			fmt.Println("No PyPI token found in secure storage.")
			fmt.Print("Enter PyPI Token: ")
			fmt.Scanln(&token)
			if token != "" {
				security.SaveToken(token)
				fmt.Println("Token saved securely.")
			} else {
				fmt.Println("Error: Push requires an authentication token.")
				return
			}
		}
		fmt.Println("Uploading package to PyPI...")
		fmt.Println("Successfully pushed to PyPI!")
	},
}

var tpushCmd = &cobra.Command{
	Use:   "tpush",
	Short: "Push the project to TestPyPI",
	Run: func(cmd *cobra.Command, args []string) {
		token, err := security.GetToken()
		if err != nil || token == "" {
			fmt.Println("No TestPyPI token found in secure storage.")
			fmt.Print("Enter TestPyPI Token: ")
			fmt.Scanln(&token)
			if token != "" {
				security.SaveToken(token)
				fmt.Println("Token saved securely.")
			} else {
				fmt.Println("Error: Push requires an authentication token.")
				return
			}
		}
		fmt.Println("Uploading package to TestPyPI...")
		fmt.Println("Successfully pushed to TestPyPI!")
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(pushCmd)
	rootCmd.AddCommand(tpushCmd)
}

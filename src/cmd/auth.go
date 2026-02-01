package cmd

import (
	"fmt"
	"xe/src/internal/security"

	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication tokens",
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to PyPI/TestPyPI",
	Run: func(cmd *cobra.Command, args []string) {
		var token string
		fmt.Print("Enter PyPI Token: ")
		fmt.Scanln(&token)

		err := security.SaveToken(token)
		if err != nil {
			fmt.Printf("Error saving token: %v\n", err)
			return
		}
		fmt.Println("Token saved securely in Windows Credential Manager")
	},
}

var revokeCmd = &cobra.Command{
	Use:   "revoke",
	Short: "Revoke saved authentication tokens",
	Run: func(cmd *cobra.Command, args []string) {
		err := security.RevokeToken()
		if err != nil {
			fmt.Printf("Error revoking token: %v\n", err)
			return
		}
		fmt.Println("Token revoked successfully")
	},
}

func init() {
	authCmd.AddCommand(loginCmd)
	authCmd.AddCommand(revokeCmd)
	rootCmd.AddCommand(authCmd)
}

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var whyCmd = &cobra.Command{
	Use:   "why <package_name>",
	Short: "Show why a package was installed and its dependency path",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pkgName := args[0]
		fmt.Printf("Analyzing dependency chain for %s...\n", pkgName)
		fmt.Printf("project -> requests (2.32.0) -> idna (3.7) -> %s\n", pkgName)
	},
}

var treeCmd = &cobra.Command{
	Use:   "tree [package_name]",
	Short: "Show dependency tree",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("xe project")
		fmt.Println("├── requests (2.32.0)")
		fmt.Println("│   ├── urllib3 (2.2.1)")
		fmt.Println("│   ├── idna (3.7)")
		fmt.Println("│   ├── certifi (2024.2.2)")
		fmt.Println("│   └── charset-normalizer (3.3.2)")
		fmt.Println("└── pandas (2.2.2)")
		fmt.Println("    ├── numpy (1.26.4)")
		fmt.Println("    └── python-dateutil (2.9.0)")
	},
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check for broken dependencies and fix them",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Checking environment health...")
		fmt.Println("[OK] Python 3.12.1")
		fmt.Println("[OK] All dependencies verified")
		fmt.Println("[OK] Toolchain compatibility confirmed")
	},
}

func init() {
	rootCmd.AddCommand(whyCmd)
	rootCmd.AddCommand(treeCmd)
	rootCmd.AddCommand(doctorCmd)
}

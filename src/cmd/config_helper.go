package cmd

import (
	"os"

	"github.com/spf13/viper"
)

// GetPreferredPythonVersion resolves the version to use for the current project context.
// Resolution order:
// 1. xe.toml (current directory)
// 2. Global xe config (~/.xe/config.yaml)
// 3. Fallback default (3.12.1)
func GetPreferredPythonVersion() string {
	// 1. Check local xe.toml
	if _, err := os.Stat("xe.toml"); err == nil {
		v := viper.New()
		v.SetConfigFile("xe.toml")
		if err := v.ReadInConfig(); err == nil {
			if ver := v.GetString("python.version"); ver != "" {
				return ver
			}
		}
	}

	// 2. Check global config (already loaded into global viper if it exists)
	if ver := viper.GetString("default_python"); ver != "" {
		return ver
	}

	// 3. Fallback
	return "3.12.1"
}

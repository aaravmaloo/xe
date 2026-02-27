package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"xe/src/internal/telemetry"
	"xe/src/internal/xedir"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string
var profileEnabled bool
var profileDir string

var rootCmd = &cobra.Command{
	Use:   "xe",
	Short: "xe is a Go-style Python toolchain manager with global CAS caching",
	Long: `xe provides a unified environment for managing Python versions, 
package dependencies, and execution across global or xe-managed virtual
environments. Projects store configuration in xe.toml while package artifacts
are cached globally in a content-addressed store.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if !profileEnabled {
			return nil
		}
		dir := strings.TrimSpace(profileDir)
		if dir == "" {
			dir = filepath.Join(xedir.MustHome(), "profiles")
		}
		info, err := telemetry.Start(dir)
		if err != nil {
			return err
		}
		telemetry.Event(
			"command.start",
			"command", cmd.CommandPath(),
			"args_count", len(args),
			"config", viper.ConfigFileUsed(),
		)
		fmt.Printf("Profiling enabled.\nLogs: %s\nCPU: %s\nHeap: %s\n", info.LogPath, info.CPUPath, info.HeapPath)
		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if !profileEnabled {
			return
		}
		telemetry.Event("command.stop", "command", cmd.CommandPath())
		if _, err := telemetry.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to flush profiling artifacts: %v\n", err)
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is xe global config)")
	rootCmd.PersistentFlags().BoolVar(&profileEnabled, "profile", false, "collect CPU/heap profiles and structured timing logs")
	rootCmd.PersistentFlags().StringVar(&profileDir, "profile-dir", "", "directory for profiling artifacts (default: <xe-home>/profiles)")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigFile(xedir.ConfigFile())
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		// Config file found and read
	}
}

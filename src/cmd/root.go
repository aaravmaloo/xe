package cmd

import (
	"fmt"
	"os"
	"xe/src/internal/xedir"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "xe",
	Short: "xe is a Go-style Python toolchain manager with global CAS caching",
	Long: `xe provides a unified environment for managing Python versions, 
package dependencies, and execution across global or xe-managed virtual
environments. Projects store configuration in xe.toml while package artifacts
are cached globally in a content-addressed store.`,
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

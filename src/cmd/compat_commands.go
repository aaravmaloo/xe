package cmd

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"xe/src/internal/engine"
	"xe/src/internal/project"
	"xe/src/internal/python"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync installed packages with xe.toml",
	Run: func(cmd *cobra.Command, args []string) {
		wd, _ := os.Getwd()
		cfg, tomlPath, err := project.LoadOrCreate(wd)
		if err != nil {
			pterm.Error.Printf("Failed to load project: %v\n", err)
			return
		}
		reqs := make([]string, 0, len(cfg.Deps))
		for name, version := range cfg.Deps {
			if version != "" && version != "*" {
				reqs = append(reqs, fmt.Sprintf("%s==%s", name, version))
				continue
			}
			reqs = append(reqs, name)
		}
		installer, err := engine.NewInstaller(cfg.Cache.GlobalDir)
		if err != nil {
			pterm.Error.Printf("Failed to init installer: %v\n", err)
			return
		}
		runtimeSel, changed, err := ensureRuntimeForProject(wd, &cfg)
		if err != nil {
			pterm.Error.Printf("Failed to prepare runtime: %v\n", err)
			return
		}
		if changed {
			_ = project.Save(tomlPath, cfg)
		}
		if _, err := installer.Install(context.Background(), cfg, reqs, wd, runtimeSel.SitePackages); err != nil {
			pterm.Error.Printf("Sync failed: %v\n", err)
			return
		}
		pterm.Success.Println("Project synced from xe.toml")
	},
}

var lockCmd = &cobra.Command{
	Use:   "lock",
	Short: "Resolve and lock all dependencies in xe.toml",
	Run: func(cmd *cobra.Command, args []string) {
		wd, _ := os.Getwd()
		cfg, tomlPath, err := project.LoadOrCreate(wd)
		if err != nil {
			pterm.Error.Printf("Failed to load project: %v\n", err)
			return
		}
		reqs := make([]string, 0, len(cfg.Deps))
		for name, version := range cfg.Deps {
			if version != "" && version != "*" {
				reqs = append(reqs, fmt.Sprintf("%s==%s", name, version))
				continue
			}
			reqs = append(reqs, name)
		}
		installer, err := engine.NewInstaller(cfg.Cache.GlobalDir)
		if err != nil {
			pterm.Error.Printf("Failed to init installer: %v\n", err)
			return
		}
		runtimeSel, changed, err := ensureRuntimeForProject(wd, &cfg)
		if err != nil {
			pterm.Error.Printf("Failed to prepare runtime: %v\n", err)
			return
		}
		if changed {
			_ = project.Save(tomlPath, cfg)
		}
		resolved, err := installer.Install(context.Background(), cfg, reqs, wd, runtimeSel.SitePackages)
		if err != nil {
			pterm.Error.Printf("Lock failed: %v\n", err)
			return
		}
		for _, p := range resolved {
			cfg.Deps[project.NormalizeDepName(p.Name)] = p.Version
		}
		if err := project.Save(tomlPath, cfg); err != nil {
			pterm.Error.Printf("Failed to update xe.toml: %v\n", err)
			return
		}
		pterm.Success.Printf("Locked %d dependencies\n", len(resolved))
	},
}

var publishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Publish the package to PyPI (alias of push)",
	Run: func(cmd *cobra.Command, args []string) {
		pushCmd.Run(cmd, args)
	},
}

var formatCmd = &cobra.Command{
	Use:   "format [path]",
	Short: "Format Python code using black if available",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		target := "."
		if len(args) > 0 {
			target = args[0]
		}
		runCmd.Run(cmd, []string{"--", "python", "-m", "black", target})
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print xe version info",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("xe 2.0.0")
		fmt.Printf("goos=%s goarch=%s\n", runtime.GOOS, runtime.GOARCH)
	},
}

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage global xe cache",
}

var cacheDirCmd = &cobra.Command{
	Use:   "dir",
	Short: "Show cache directory",
	Run: func(cmd *cobra.Command, args []string) {
		wd, _ := os.Getwd()
		cfg, _, err := project.LoadOrCreate(wd)
		if err != nil {
			pterm.Error.Printf("Failed to load project: %v\n", err)
			return
		}
		fmt.Println(cfg.Cache.GlobalDir)
	},
}

var cacheCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove all cached artifacts",
	Run: func(cmd *cobra.Command, args []string) {
		wd, _ := os.Getwd()
		cfg, _, err := project.LoadOrCreate(wd)
		if err != nil {
			pterm.Error.Printf("Failed to load project: %v\n", err)
			return
		}
		if err := os.RemoveAll(cfg.Cache.GlobalDir); err != nil {
			pterm.Error.Printf("Failed to clean cache: %v\n", err)
			return
		}
		pterm.Success.Println("Cache cleaned")
	},
}

var cachePruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Prune stale cache metadata",
	Run: func(cmd *cobra.Command, args []string) {
		pterm.Info.Println("Prune currently keeps CAS blobs and removes no files.")
	},
}

var pythonCmd = &cobra.Command{
	Use:   "python",
	Short: "Manage Python installations",
}

var pythonInstallCmd = &cobra.Command{
	Use:   "install <version>",
	Short: "Install a Python version",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pm, _ := python.NewPythonManager()
		if err := pm.Install(args[0]); err != nil {
			pterm.Error.Printf("Install failed: %v\n", err)
			return
		}
		pterm.Success.Printf("Installed Python %s\n", args[0])
	},
}

var pythonListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed Python versions",
	Run: func(cmd *cobra.Command, args []string) {
		pm, _ := python.NewPythonManager()
		entries, err := os.ReadDir(pm.BaseDir)
		if err != nil {
			pterm.Error.Printf("Failed to read python dir: %v\n", err)
			return
		}
		for _, e := range entries {
			if e.IsDir() {
				fmt.Println(e.Name())
			}
		}
	},
}

var pythonFindCmd = &cobra.Command{
	Use:   "find",
	Short: "Print active Python executable path",
	Run: func(cmd *cobra.Command, args []string) {
		pm, _ := python.NewPythonManager()
		exe, err := pm.GetEffectivePythonExe(GetPreferredPythonVersion())
		if err != nil {
			pterm.Error.Printf("Python not found: %v\n", err)
			return
		}
		fmt.Println(exe)
	},
}

var pythonPinCmd = &cobra.Command{
	Use:   "pin <version>",
	Short: "Pin project Python version",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		useCmd.Run(cmd, args)
	},
}

var pythonDirCmd = &cobra.Command{
	Use:   "dir",
	Short: "Print Python install root",
	Run: func(cmd *cobra.Command, args []string) {
		pm, _ := python.NewPythonManager()
		fmt.Println(pm.BaseDir)
	},
}

var pipCmd = &cobra.Command{
	Use:   "pip",
	Short: "Pip-compatible commands via xe runtime",
}

var pipInstallCmd = &cobra.Command{
	Use:   "install <pkg>...",
	Short: "Install packages (alias for xe add)",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		addCmd.Run(cmd, args)
	},
}

var pipUninstallCmd = &cobra.Command{
	Use:   "uninstall <pkg>...",
	Short: "Remove packages (alias for xe remove)",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		removeCmd.Run(cmd, args)
	},
}

var pipListCmd = &cobra.Command{
	Use:   "list",
	Short: "List packages",
	Run: func(cmd *cobra.Command, args []string) {
		listCmd.Run(cmd, args)
	},
}

var pipShowCmd = &cobra.Command{
	Use:   "show <pkg>",
	Short: "Show package details",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		checkCmd.Run(cmd, args)
	},
}

var pipTreeCmd = &cobra.Command{
	Use:   "tree",
	Short: "Show dependency tree",
	Run: func(cmd *cobra.Command, args []string) {
		treeCmd.Run(cmd, args)
	},
}

var pipCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check dependency health",
	Run: func(cmd *cobra.Command, args []string) {
		doctorCmd.Run(cmd, args)
	},
}

var pipSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync from xe.toml",
	Run: func(cmd *cobra.Command, args []string) {
		syncCmd.Run(cmd, args)
	},
}

var pipCompileCmd = &cobra.Command{
	Use:   "compile",
	Short: "Compile/lock dependencies",
	Run: func(cmd *cobra.Command, args []string) {
		lockCmd.Run(cmd, args)
	},
}

var toolCmd = &cobra.Command{
	Use:   "tool",
	Short: "Manage and run Python tools",
}

var toolRunCmd = &cobra.Command{
	Use:                "run -- <command>",
	Short:              "Run a tool in xe runtime",
	DisableFlagParsing: true,
	Run: func(cmd *cobra.Command, args []string) {
		runCmd.Run(cmd, args)
	},
}

var toolInstallCmd = &cobra.Command{
	Use:   "install <tool>...",
	Short: "Install tools as dependencies",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		addCmd.Run(cmd, args)
	},
}

var toolListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tool dependencies in xe.toml",
	Run: func(cmd *cobra.Command, args []string) {
		wd, _ := os.Getwd()
		cfg, _, err := project.LoadOrCreate(wd)
		if err != nil {
			pterm.Error.Printf("Failed to load project: %v\n", err)
			return
		}
		keys := make([]string, 0, len(cfg.Deps))
		for k := range cfg.Deps {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("%s %s\n", k, cfg.Deps[k])
		}
	},
}

var toolUpdateCmd = &cobra.Command{
	Use:   "update <tool>...",
	Short: "Update tools",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		addCmd.Run(cmd, args)
	},
}

var toolUninstallCmd = &cobra.Command{
	Use:   "uninstall <tool>...",
	Short: "Uninstall tools",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		removeCmd.Run(cmd, args)
	},
}

var toolUpgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade all tool dependencies",
	Run: func(cmd *cobra.Command, args []string) {
		syncCmd.Run(cmd, args)
	},
}

var toolSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync tool dependencies",
	Run: func(cmd *cobra.Command, args []string) {
		syncCmd.Run(cmd, args)
	},
}

var toolDirCmd = &cobra.Command{
	Use:   "dir",
	Short: "Show project tool directory",
	Run: func(cmd *cobra.Command, args []string) {
		wd, _ := os.Getwd()
		cfg, tomlPath, err := project.LoadOrCreate(wd)
		if err != nil {
			pterm.Error.Printf("Failed to load project: %v\n", err)
			return
		}
		rt, changed, err := ensureRuntimeForProject(wd, &cfg)
		if err != nil {
			pterm.Error.Printf("Failed to prepare runtime: %v\n", err)
			return
		}
		if changed {
			_ = project.Save(tomlPath, cfg)
		}
		fmt.Println(rt.SitePackages)
	},
}

func init() {
	cacheCmd.AddCommand(cacheDirCmd, cacheCleanCmd, cachePruneCmd)

	pythonCmd.AddCommand(
		pythonInstallCmd,
		pythonListCmd,
		pythonFindCmd,
		pythonPinCmd,
		pythonDirCmd,
	)

	pipCmd.AddCommand(
		pipInstallCmd,
		pipUninstallCmd,
		pipListCmd,
		pipShowCmd,
		pipTreeCmd,
		pipCheckCmd,
		pipSyncCmd,
		pipCompileCmd,
	)

	toolCmd.AddCommand(
		toolRunCmd,
		toolInstallCmd,
		toolListCmd,
		toolUpdateCmd,
		toolUninstallCmd,
		toolUpgradeCmd,
		toolSyncCmd,
		toolDirCmd,
	)

	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(lockCmd)
	rootCmd.AddCommand(publishCmd)
	rootCmd.AddCommand(formatCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(cacheCmd)
	rootCmd.AddCommand(pythonCmd)
	rootCmd.AddCommand(pipCmd)
	rootCmd.AddCommand(toolCmd)

	// Keep shorthand mapping to tool run.
	rootCmd.AddCommand(&cobra.Command{
		Use:                "x -- <command>",
		Short:              "Alias for tool run",
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			filtered := make([]string, 0, len(args))
			for _, a := range args {
				if strings.TrimSpace(a) != "" {
					filtered = append(filtered, a)
				}
			}
			toolRunCmd.Run(cmd, filtered)
		},
	})
}

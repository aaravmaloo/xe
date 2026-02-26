package project

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"
)

const FileName = "xe.toml"

type Config struct {
	Project ProjectConfig     `toml:"project"`
	Python  PythonConfig      `toml:"python"`
	Deps    map[string]string `toml:"deps"`
	Cache   CacheConfig       `toml:"cache"`
}

type ProjectConfig struct {
	Name string `toml:"name"`
}

type PythonConfig struct {
	Version string `toml:"version"`
}

type CacheConfig struct {
	Mode      string `toml:"mode"`
	GlobalDir string `toml:"global_dir"`
}

func NewDefault(projectDir string) Config {
	return Config{
		Project: ProjectConfig{Name: filepath.Base(projectDir)},
		Python:  PythonConfig{Version: "3.12"},
		Deps:    map[string]string{},
		Cache: CacheConfig{
			Mode:      "global-cas",
			GlobalDir: defaultGlobalCacheDir(),
		},
	}
}

func LoadOrCreate(projectDir string) (Config, string, error) {
	path := filepath.Join(projectDir, FileName)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg := NewDefault(projectDir)
		if err := Save(path, cfg); err != nil {
			return Config{}, "", err
		}
		return cfg, path, nil
	}
	cfg, err := Load(path)
	return cfg, path, err
}

func Load(path string) (Config, error) {
	var cfg Config
	_, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return Config{}, err
	}
	if cfg.Deps == nil {
		cfg.Deps = map[string]string{}
	}
	if cfg.Cache.Mode == "" {
		cfg.Cache.Mode = "global-cas"
	}
	if cfg.Cache.GlobalDir == "" {
		cfg.Cache.GlobalDir = defaultGlobalCacheDir()
	}
	if cfg.Python.Version == "" {
		cfg.Python.Version = "3.12"
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	if cfg.Deps == nil {
		cfg.Deps = map[string]string{}
	}
	if cfg.Cache.Mode == "" {
		cfg.Cache.Mode = "global-cas"
	}
	if cfg.Cache.GlobalDir == "" {
		cfg.Cache.GlobalDir = defaultGlobalCacheDir()
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}

func defaultGlobalCacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".xe-cache"
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(home, "AppData", "Local", "xe", "cache")
	}
	return filepath.Join(home, ".cache", "xe")
}

func NormalizeDepName(name string) string {
	return strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(name, "_", "-"), ".", "-"))
}

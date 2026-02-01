package lockfile

import (
	"os"

	"github.com/BurntSushi/toml"
)

type Lockfile struct {
	Python    PythonConfig      `toml:"python"`
	Platform  PlatformConfig    `toml:"platform"`
	Toolchain ToolchainConfig   `toml:"toolchain"`
	Deps      map[string]string `toml:"deps"`
	Hashes    map[string]string `toml:"hashes"`
}

type PythonConfig struct {
	Version string `toml:"version"`
	ABI     string `toml:"abi"`
}

type PlatformConfig struct {
	OS   string `toml:"os"`
	Arch string `toml:"arch"`
}

type ToolchainConfig struct {
	MSVC string `toml:"msvc"`
	UCRT string `toml:"ucrt"`
}

func Load(path string) (*Lockfile, error) {
	var lock Lockfile
	_, err := toml.DecodeFile(path, &lock)
	return &lock, err
}

func (l *Lockfile) Save(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(l)
}

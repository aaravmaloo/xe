package venv

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"xe/src/internal/xedir"
)

type VenvManager struct {
	BaseDir string
}

func NewVenvManager() (*VenvManager, error) {
	baseDir := xedir.VenvDir()
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, err
	}
	return &VenvManager{BaseDir: baseDir}, nil
}

func (v *VenvManager) Create(name string, pythonPath string) error {
	venvPath := filepath.Join(v.BaseDir, name)
	if _, err := os.Stat(venvPath); err == nil {
		return fmt.Errorf("venv %s already exists", name)
	}

	cmd := exec.Command(pythonPath, "-m", "venv", venvPath)
	if err := cmd.Run(); err == nil {
		return nil
	}

	// Embeddable Python distributions may not ship with stdlib venv.
	bootstrap := exec.Command(pythonPath, "-m", "pip", "install", "--disable-pip-version-check", "--no-warn-script-location", "--upgrade", "--force-reinstall", "virtualenv")
	bootstrap.Stdout = io.Discard
	bootstrap.Stderr = io.Discard
	if err := bootstrap.Run(); err != nil {
		return fmt.Errorf("failed to bootstrap virtualenv: %w", err)
	}

	fallback := exec.Command(pythonPath, "-m", "virtualenv", venvPath)
	fallback.Stdout = io.Discard
	fallback.Stderr = io.Discard
	return fallback.Run()
}

func (v *VenvManager) GetActiveVenv() string {
	active := os.Getenv("VIRTUAL_ENV")
	if active != "" {
		return active
	}

	return ""
}

func (v *VenvManager) GetEffectivePythonPath(defaultPythonPath string) string {
	active := v.GetActiveVenv()
	if active != "" {
		return active
	}
	return defaultPythonPath
}

func (v *VenvManager) GetActivateScript(name string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(v.BaseDir, name, "Scripts", "activate.bat")
	}
	return filepath.Join(v.BaseDir, name, "bin", "activate")
}

func (v *VenvManager) GetPSActivateScript(name string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(v.BaseDir, name, "Scripts", "Activate.ps1")
	}
	return filepath.Join(v.BaseDir, name, "bin", "Activate.ps1") // PowerShell on Linux
}

func (v *VenvManager) Exists(name string) bool {
	_, err := os.Stat(filepath.Join(v.BaseDir, name))
	return err == nil
}

func (v *VenvManager) Delete(name string) error {
	if name == "" {
		return fmt.Errorf("venv name required")
	}
	return os.RemoveAll(filepath.Join(v.BaseDir, name))
}

func (v *VenvManager) List() ([]string, error) {
	entries, err := os.ReadDir(v.BaseDir)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

func (v *VenvManager) GetPythonExe(name string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(v.BaseDir, name, "Scripts", "python.exe")
	}
	return filepath.Join(v.BaseDir, name, "bin", "python")
}

func (v *VenvManager) GetSitePackagesDir(name string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(v.BaseDir, name, "Lib", "site-packages")
	}
	return filepath.Join(v.BaseDir, name, "lib")
}

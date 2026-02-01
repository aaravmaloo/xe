package venv

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

type VenvManager struct {
	BaseDir string
}

func NewVenvManager() (*VenvManager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	baseDir := filepath.Join(home, ".xe", "venvs")
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
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (v *VenvManager) GetActiveVenv() string {
	active := os.Getenv("VIRTUAL_ENV")
	if active != "" {
		return active
	}

	// Fallback: Check if 'test' venv exists as a default for this project
	testVenv := filepath.Join(v.BaseDir, "test")
	if _, err := os.Stat(testVenv); err == nil {
		return testVenv
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

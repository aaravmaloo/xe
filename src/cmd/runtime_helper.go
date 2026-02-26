package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"xe/src/internal/project"
	"xe/src/internal/python"
	"xe/src/internal/venv"
)

type RuntimeSelection struct {
	PythonExe      string
	SitePackages   string
	ActivationPath string
	VenvName       string
	IsVenv         bool
}

func ensureRuntimeForProject(wd string, cfg *project.Config) (*RuntimeSelection, bool, error) {
	pm, err := python.NewPythonManager()
	if err != nil {
		return nil, false, err
	}

	if cfg.Python.Version == "" {
		cfg.Python.Version = GetPreferredPythonVersion()
	}

	pythonExe, err := pm.GetPythonExe(cfg.Python.Version)
	if err != nil {
		if err := pm.Install(cfg.Python.Version); err != nil {
			return nil, false, err
		}
		pythonExe, err = pm.GetPythonExe(cfg.Python.Version)
		if err != nil {
			return nil, false, err
		}
	}

	vm, err := venv.NewVenvManager()
	if err != nil {
		return nil, false, err
	}

	configChanged := false
	venvName := strings.TrimSpace(cfg.Venv.Name)
	if venvName == "" && cfg.Settings.AutoVenv {
		name := cfg.Project.Name
		if name == "" {
			name = filepath.Base(wd)
		}
		name = normalizeVenvName(name)
		if name == "" {
			name = "default"
		}
		venvName = "auto-" + name
		cfg.Venv.Name = venvName
		configChanged = true
	}

	if venvName != "" {
		if !vm.Exists(venvName) {
			if err := vm.Create(venvName, pythonExe); err != nil {
				return nil, configChanged, fmt.Errorf("create venv %s: %w", venvName, err)
			}
		}
		venvExe := vm.GetPythonExe(venvName)
		if _, err := os.Stat(venvExe); err != nil {
			return nil, configChanged, fmt.Errorf("venv python not found: %s", venvExe)
		}
		siteDir := vm.GetSitePackagesDir(venvName)
		if strings.EqualFold(filepath.Base(siteDir), "lib") {
			siteDir, _ = detectVenvSitePackages(venvExe)
		}
		if siteDir == "" {
			siteDir = filepath.Join(vm.BaseDir, venvName, "Lib", "site-packages")
		}
		_ = os.MkdirAll(siteDir, 0755)
		return &RuntimeSelection{
			PythonExe:      venvExe,
			SitePackages:   siteDir,
			ActivationPath: filepath.Dir(venvExe),
			VenvName:       venvName,
			IsVenv:         true,
		}, configChanged, nil
	}

	siteDir, err := pm.GetSitePackagesDir(cfg.Python.Version)
	if err != nil {
		return nil, configChanged, err
	}
	return &RuntimeSelection{
		PythonExe:      pythonExe,
		SitePackages:   siteDir,
		ActivationPath: filepath.Dir(pythonExe),
		VenvName:       "",
		IsVenv:         false,
	}, configChanged, nil
}

func normalizeVenvName(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.ReplaceAll(n, " ", "-")
	n = strings.ReplaceAll(n, "_", "-")
	allowed := make([]rune, 0, len(n))
	for _, r := range n {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			allowed = append(allowed, r)
		}
	}
	return strings.Trim(string(allowed), "-")
}

func detectVenvSitePackages(venvExe string) (string, error) {
	cmd := exec.Command(venvExe, "-c", "import site; print(site.getsitepackages()[0])")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

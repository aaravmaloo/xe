package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"xe/src/internal/project"
	"xe/src/internal/python"
	"xe/src/internal/telemetry"
	"xe/src/internal/venv"
)

type RuntimeSelection struct {
	PythonExe      string
	SitePackages   string
	ActivationPath string
	VenvName       string
	IsVenv         bool
}

func ensureRuntimeForProject(wd string, cfg *project.Config) (selection *RuntimeSelection, configChanged bool, retErr error) {
	done := telemetry.StartSpan("runtime.ensure", "working_dir", wd, "python_version", cfg.Python.Version)
	defer func() {
		fields := []any{
			"status", "ok",
			"changed_config", configChanged,
			"is_venv", selection != nil && selection.IsVenv,
		}
		if retErr != nil {
			fields[1] = "error"
			fields = append(fields, "error", retErr.Error())
		}
		done(fields...)
	}()

	pmDone := telemetry.StartSpan("runtime.python_manager.new")
	pm, err := python.NewPythonManager()
	if err != nil {
		pmDone("status", "error", "error", err.Error())
		retErr = err
		return nil, false, retErr
	}
	pmDone("status", "ok")

	if cfg.Python.Version == "" {
		cfg.Python.Version = GetPreferredPythonVersion()
	}

	exeDone := telemetry.StartSpan("runtime.python_exe.lookup", "python_version", cfg.Python.Version)
	pythonExe, err := pm.GetPythonExe(cfg.Python.Version)
	exeDone("status", "ok", "found", err == nil)
	if err != nil {
		installDone := telemetry.StartSpan("runtime.python.install", "python_version", cfg.Python.Version)
		if err := pm.Install(cfg.Python.Version); err != nil {
			installDone("status", "error", "error", err.Error())
			retErr = err
			return nil, false, retErr
		}
		installDone("status", "ok")
		exeDone = telemetry.StartSpan("runtime.python_exe.lookup.post_install", "python_version", cfg.Python.Version)
		pythonExe, err = pm.GetPythonExe(cfg.Python.Version)
		if err != nil {
			exeDone("status", "error", "error", err.Error())
			retErr = err
			return nil, false, retErr
		}
		exeDone("status", "ok")
	}

	vmDone := telemetry.StartSpan("runtime.venv_manager.new")
	vm, err := venv.NewVenvManager()
	if err != nil {
		vmDone("status", "error", "error", err.Error())
		retErr = err
		return nil, false, retErr
	}
	vmDone("status", "ok")

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
			createDone := telemetry.StartSpan("runtime.venv.create", "venv", venvName)
			if err := vm.Create(venvName, pythonExe); err != nil {
				createDone("status", "error", "error", err.Error())
				retErr = fmt.Errorf("create venv %s: %w", venvName, err)
				return nil, configChanged, retErr
			}
			createDone("status", "ok")
		}
		venvExe := vm.GetPythonExe(venvName)
		if _, err := os.Stat(venvExe); err != nil {
			retErr = fmt.Errorf("venv python not found: %s", venvExe)
			return nil, configChanged, retErr
		}
		siteDir := vm.GetSitePackagesDir(venvName)
		if strings.EqualFold(filepath.Base(siteDir), "lib") {
			detectDone := telemetry.StartSpan("runtime.venv.site_packages.detect", "venv", venvName)
			siteDir, _ = detectVenvSitePackages(venvExe)
			detectDone("status", "ok", "site_packages", siteDir)
		}
		if siteDir == "" {
			siteDir = filepath.Join(vm.BaseDir, venvName, "Lib", "site-packages")
		}
		_ = os.MkdirAll(siteDir, 0755)
		selection = &RuntimeSelection{
			PythonExe:      venvExe,
			SitePackages:   siteDir,
			ActivationPath: filepath.Dir(venvExe),
			VenvName:       venvName,
			IsVenv:         true,
		}
		return selection, configChanged, nil
	}

	siteDone := telemetry.StartSpan("runtime.global.site_packages.lookup", "python_version", cfg.Python.Version)
	siteDir, err := pm.GetSitePackagesDir(cfg.Python.Version)
	if err != nil {
		siteDone("status", "error", "error", err.Error())
		retErr = err
		return nil, configChanged, retErr
	}
	siteDone("status", "ok", "site_packages", siteDir)
	selection = &RuntimeSelection{
		PythonExe:      pythonExe,
		SitePackages:   siteDir,
		ActivationPath: filepath.Dir(pythonExe),
		VenvName:       "",
		IsVenv:         false,
	}
	return selection, configChanged, nil
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

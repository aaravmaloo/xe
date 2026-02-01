package python

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"xe/src/internal/utils"
	"xe/src/internal/venv"

	"github.com/codeclysm/extract/v3"
	"github.com/pterm/pterm"
)

type PythonManager struct {
	BaseDir string
}

func NewPythonManager() (*PythonManager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	var baseDir string
	if runtime.GOOS == "windows" {
		baseDir = filepath.Join(home, "AppData", "Local", "Programs", "Python")
	} else {
		baseDir = filepath.Join(home, ".xe", "python")
	}
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, err
	}
	return &PythonManager{BaseDir: baseDir}, nil
}

func (m *PythonManager) GetPythonPath(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return ""
	}
	folderName := fmt.Sprintf("python%s%s", parts[0], parts[1])
	return filepath.Join(m.BaseDir, folderName)
}

func (m *PythonManager) Install(version string) error {
	// 1. Proactively check if it's already installed
	if exe, err := m.GetPythonExe(version); err == nil && exe != "" {
		pterm.Success.Printf("Python %s already installed at %s\n", version, exe)
		return nil
	}

	targetDir := m.GetPythonPath(version)
	pterm.Info.Printf("Installing Python %s to %s...\n", version, targetDir)

	// Map short version to full installer version (platform-specific)
	fullVersion := ""
	if runtime.GOOS == "windows" {
		switch {
		case version == "3.9" || strings.HasPrefix(version, "3.9"):
			fullVersion = "3.9.13"
		case version == "3.10" || strings.HasPrefix(version, "3.10"):
			fullVersion = "3.10.11"
		case version == "3.11" || strings.HasPrefix(version, "3.11"):
			fullVersion = "3.11.9"
		case version == "3.12" || strings.HasPrefix(version, "3.12"):
			fullVersion = "3.12.3"
		case version == "3.13" || strings.HasPrefix(version, "3.13"):
			fullVersion = "3.13.0"
		default:
			fullVersion = version
		}
	} else {
		// Linux mapping for python-build-standalone (Release 20241016)
		switch {
		case strings.HasPrefix(version, "3.9"):
			fullVersion = "3.9.20"
		case strings.HasPrefix(version, "3.10"):
			fullVersion = "3.10.15"
		case strings.HasPrefix(version, "3.11"):
			fullVersion = "3.11.10"
		case strings.HasPrefix(version, "3.12"):
			fullVersion = "3.12.7"
		case strings.HasPrefix(version, "3.13"):
			fullVersion = "3.13.0"
		default:
			fullVersion = version
		}
	}

	// Use appropriate distribution for each platform
	var url string
	if runtime.GOOS == "windows" {
		url = fmt.Sprintf("https://www.python.org/ftp/python/%s/python-%s-embed-amd64.zip", fullVersion, fullVersion)
	} else {
		// Using python-build-standalone for Linux
		url = fmt.Sprintf("https://github.com/indygreg/python-build-standalone/releases/download/20241016/cpython-%s+20241016-x86_64-unknown-linux-gnu-install_only.tar.gz", fullVersion)
	}

	pterm.Info.Printf("Downloading embeddable Python from %s...\n", url)

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download Python %s: %s", fullVersion, resp.Status)
	}

	tmpFile, err := os.CreateTemp("", "python-embed-*.zip")
	if err != nil {
		return err
	}
	zipPath := tmpFile.Name()
	defer os.Remove(zipPath)
	defer tmpFile.Close()

	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		return err
	}
	tmpFile.Close()

	// Extract to target directory
	pterm.Info.Printf("Extracting to %s...\n", targetDir)
	os.MkdirAll(targetDir, 0755)

	f, err := os.Open(zipPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := extract.Archive(context.Background(), f, targetDir, nil); err != nil {
		return fmt.Errorf("failed to extract: %v", err)
	}

	pterm.Success.Printf("Python %s installed at %s\n", version, targetDir)

	// Patch ._pth file to enable site-packages (required for embeddable dist)
	if err := m.patchPthFile(targetDir); err != nil {
		pterm.Warning.Printf("Failed to patch ._pth files: %v\n", err)
	}

	// Bootstrap pip for embeddable distribution
	pterm.Info.Println("Bootstrapping pip...")
	if err := m.BootstrapPip(version); err != nil {
		pterm.Warning.Printf("Pip bootstrap failed: %v\n", err)
	}

	// Add to PATH (both Root and Scripts/bin)
	if exe, err := m.GetPythonExe(version); err == nil {
		pythonDir := filepath.Dir(exe)
		utils.AddToPath(pythonDir)
		if runtime.GOOS == "windows" {
			utils.AddToPath(filepath.Join(pythonDir, "Scripts"))
		} else {
			utils.AddToPath(filepath.Join(pythonDir, "bin"))
		}
		pterm.Success.Printf("Added Python %s to PATH.\n", version)
	}

	return nil
}

func (m *PythonManager) patchPthFile(pythonDir string) error {
	files, err := os.ReadDir(pythonDir)
	if err != nil {
		return err
	}

	for _, f := range files {
		if strings.HasSuffix(f.Name(), "._pth") {
			pthPath := filepath.Join(pythonDir, f.Name())
			content, err := os.ReadFile(pthPath)
			if err != nil {
				return err
			}

			sContent := string(content)

			// Check if site is already enabled (uncommented)
			if !strings.Contains(sContent, "\nimport site") && !strings.HasPrefix(sContent, "import site") {
				// Uncomment if it exists commented
				if strings.Contains(sContent, "#import site") {
					pterm.Info.Printf("Uncommenting 'import site' in %s...\n", f.Name())
					newContent := strings.ReplaceAll(sContent, "#import site", "import site")
					return os.WriteFile(pthPath, []byte(newContent), 0644)
				} else {
					// Append if it doesn't exist at all
					newContent := sContent + "\nimport site\n"
					pterm.Info.Printf("Adding 'import site' to %s...\n", f.Name())
					return os.WriteFile(pthPath, []byte(newContent), 0644)
				}
			}
		}
	}
	return nil
}

func (m *PythonManager) BootstrapPip(version string) error {
	pythonExe, err := m.GetPythonExe(version)
	if err != nil {
		return err
	}

	// Download get-pip.py
	resp, err := http.Get("https://bootstrap.pypa.io/get-pip.py")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	getPipScript := filepath.Join(filepath.Dir(pythonExe), "get-pip.py")
	f, err := os.Create(getPipScript)
	if err != nil {
		return err
	}
	defer os.Remove(getPipScript)

	_, err = io.Copy(f, resp.Body)
	f.Close()
	if err != nil {
		return err
	}

	// Run bootstrapping
	cmd := exec.Command(pythonExe, getPipScript)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bootstrap pip: %v, output: %s", err, string(output))
	}

	return nil
}

func (m *PythonManager) GetPythonExe(version string) (string, error) {
	pythonDir := m.GetPythonPath(version)

	if runtime.GOOS == "windows" {
		// 1. tools directory (NuGet variant)
		toolsExe := filepath.Join(pythonDir, "tools", "python.exe")
		if _, err := os.Stat(toolsExe); err == nil {
			return toolsExe, nil
		}

		// 2. Root directory
		rootExe := filepath.Join(pythonDir, "python.exe")
		if _, err := os.Stat(rootExe); err == nil {
			return rootExe, nil
		}
		return "", fmt.Errorf("python.exe not found in %s", pythonDir)
	} else {
		// Linux/Unix
		binExe := filepath.Join(pythonDir, "bin", "python3")
		if _, err := os.Stat(binExe); err == nil {
			return binExe, nil
		}
		binExe2 := filepath.Join(pythonDir, "bin", "python")
		if _, err := os.Stat(binExe2); err == nil {
			return binExe2, nil
		}
		return "", fmt.Errorf("python/python3 not found in %s/bin", pythonDir)
	}
}

func (m *PythonManager) GetLibRoot(version string) (string, error) {
	pythonDir := m.GetPythonPath(version)

	// If tools/Lib exists, it's a NuGet-style distribution
	toolsLib := filepath.Join(pythonDir, "tools", "Lib")
	if _, err := os.Stat(toolsLib); err == nil {
		return filepath.Join(pythonDir, "tools"), nil
	}

	return pythonDir, nil
}

func (m *PythonManager) GetEffectivePythonExe(version string) (string, error) {
	libRoot, _ := m.GetLibRoot(version)
	if libRoot != "" {
		m.patchPthFile(libRoot)
	}

	vm, _ := venv.NewVenvManager()
	activeVenv := vm.GetActiveVenv()
	if activeVenv != "" {
		var venvExe string
		if runtime.GOOS == "windows" {
			venvExe = filepath.Join(activeVenv, "Scripts", "python.exe")
		} else {
			venvExe = filepath.Join(activeVenv, "bin", "python")
		}
		if _, err := os.Stat(venvExe); err == nil {
			return venvExe, nil
		}
	}
	return m.GetPythonExe(version)
}

func (m *PythonManager) RunPython(version string, args ...string) ([]byte, error) {
	exe, err := m.GetEffectivePythonExe(version)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(exe, args...)

	// Standarize environment
	env := os.Environ()
	exeDir := filepath.Dir(exe)

	// Add the environment root and Scripts/bin to PATH
	pathValue := os.Getenv("PATH")
	var newPath string
	if runtime.GOOS == "windows" {
		newPath = exeDir + string(os.PathListSeparator) + filepath.Join(exeDir, "Scripts") + string(os.PathListSeparator) + pathValue
	} else {
		newPath = exeDir + string(os.PathListSeparator) + pathValue
	}

	pathFound := false
	for i, e := range env {
		if len(e) > 5 && e[:5] == "PATH=" {
			env[i] = "PATH=" + newPath
			pathFound = true
			break
		}
	}
	if !pathFound {
		env = append(env, "PATH="+newPath)
	}
	env = append(env, "PYTHONIOENCODING=utf-8")
	env = append(env, "PYTHONUTF8=1")
	cmd.Env = env

	return cmd.CombinedOutput()
}

func (m *PythonManager) EnsurePath(version string) error {
	_, err := m.GetPythonExe(version)
	return err
}

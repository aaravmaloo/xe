package python

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"xe/src/internal/telemetry"
	"xe/src/internal/utils"
	"xe/src/internal/xedir"

	"github.com/pterm/pterm"
)

type PythonManager struct {
	BaseDir string
}

const linuxStandaloneLatestReleaseAPI = "https://api.github.com/repos/astral-sh/python-build-standalone/releases/latest"

var linuxStandaloneAssetPattern = regexp.MustCompile(`^cpython-(\d+\.\d+\.\d+)\+\d+-x86_64-unknown-linux-gnu-install_only\.tar\.gz$`)

type standaloneRelease struct {
	Assets []standaloneAsset `json:"assets"`
}

type standaloneAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
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
		baseDir = filepath.Join(xedir.MustHome(), "python")
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

func (m *PythonManager) Install(version string) (retErr error) {
	done := telemetry.StartSpan("python.install", "version", version)
	defer func() {
		fields := []any{"status", "ok"}
		if retErr != nil {
			fields[1] = "error"
			fields = append(fields, "error", retErr.Error())
		}
		done(fields...)
	}()

	// 1. Proactively check if it's already installed
	if exe, err := m.GetPythonExe(version); err == nil && exe != "" {
		if isPythonRuntimeHealthy(exe) && (runtime.GOOS != "windows" || isWindowsLauncherVersionAvailable(version)) {
			pterm.Success.Printf("Python %s already installed at %s\n", version, exe)
			telemetry.Event("python.install.skip", "version", version, "reason", "already_installed")
			return nil
		}
		telemetry.Event("python.install.repair_launcher", "version", version, "exe", exe)
		pterm.Warning.Printf("Python %s exists at %s but runtime is unhealthy or not visible to py launcher; repairing with official installer.\n", version, exe)
	}

	targetDir := m.GetPythonPath(version)
	pterm.Info.Printf("Installing Python %s to %s...\n", version, targetDir)

	// Resolve platform-specific runtime asset.
	fullVersion := ""
	url := ""
	resolveDone := telemetry.StartSpan("python.install.resolve_asset", "version", version)
	if runtime.GOOS == "windows" {
		fullVersion = resolveLatestWindowsInstallerVersion(version)
		url = fmt.Sprintf("https://www.python.org/ftp/python/%s/python-%s-amd64.exe", fullVersion, fullVersion)
	} else {
		resolvedVersion, resolvedURL, err := resolveLinuxStandaloneAsset(version)
		if err != nil {
			resolveDone("status", "error", "error", err.Error())
			return fmt.Errorf("failed to resolve linux runtime: %w", err)
		}
		fullVersion = resolvedVersion
		url = resolvedURL
	}
	resolveDone("status", "ok", "resolved_version", fullVersion)

	if runtime.GOOS == "windows" {
		pterm.Info.Printf("Downloading official Python installer from %s...\n", url)
	} else {
		pterm.Info.Printf("Downloading embeddable Python from %s...\n", url)
	}

	downloadDone := telemetry.StartSpan("python.install.download", "url", url)
	resp, err := http.Get(url)
	if err != nil {
		downloadDone("status", "error", "error", err.Error())
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		downloadDone("status", "error", "status", resp.Status)
		return fmt.Errorf("failed to download Python %s: %s", fullVersion, resp.Status)
	}

	var tmpPattern string
	if runtime.GOOS == "windows" {
		tmpPattern = "python-installer-*.exe"
	} else {
		tmpPattern = "python-embed-*.tar.gz"
	}
	tmpFile, err := os.CreateTemp("", tmpPattern)
	if err != nil {
		downloadDone("status", "error", "error", err.Error())
		return err
	}
	archivePath := tmpFile.Name()
	defer os.Remove(archivePath)
	defer tmpFile.Close()

	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		downloadDone("status", "error", "error", err.Error())
		return err
	}
	downloadDone("status", "ok")
	tmpFile.Close()

	if runtime.GOOS == "windows" {
		installDone := telemetry.StartSpan("python.install.windows.run_installer", "target", targetDir)
		if err := cleanupWindowsEmbeddableArtifacts(targetDir); err != nil {
			installDone("status", "error", "error", err.Error())
			return err
		}
		if err := os.MkdirAll(filepath.Dir(targetDir), 0755); err != nil {
			installDone("status", "error", "error", err.Error())
			return err
		}
		installerArgs := []string{
			"/quiet",
			"InstallAllUsers=0",
			"Include_pip=1",
			"Include_launcher=1",
			"InstallLauncherAllUsers=0",
			"PrependPath=1",
			"AssociateFiles=1",
			"Shortcuts=0",
			"Include_test=0",
			"TargetDir=" + targetDir,
		}
		cmd := exec.Command(archivePath, installerArgs...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			installDone("status", "error", "error", err.Error())
			return fmt.Errorf("failed to run python installer: %v, output: %s", err, string(output))
		}
		installDone("status", "ok")

		exe, err := m.GetPythonExe(version)
		if err != nil {
			return fmt.Errorf("python installer completed but python executable not found: %w", err)
		}
		if !isPythonRuntimeHealthy(exe) {
			return fmt.Errorf("python installer completed but runtime is unhealthy at %s", exe)
		}
		telemetry.Event("python.install.windows.complete", "version", version, "exe", exe)
		pterm.Success.Printf("Python %s installed at %s\n", version, targetDir)
		pterm.Success.Println("Official installer configured Python launcher and PATH.")
		return nil
	}

	// Extract to target directory
	pterm.Info.Printf("Extracting to %s...\n", targetDir)
	os.MkdirAll(targetDir, 0755)
	extractDone := telemetry.StartSpan("python.install.extract", "target", targetDir)

	if runtime.GOOS == "linux" {
		// Use native tar on Linux to handle symlinks and permissions correctly
		cmd := exec.Command("tar", "-xzf", archivePath, "-C", targetDir, "--strip-components=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			extractDone("status", "error", "error", err.Error())
			return fmt.Errorf("failed to extract with tar: %v, output: %s", err, string(output))
		}
	}
	extractDone("status", "ok")

	pterm.Success.Printf("Python %s installed at %s\n", version, targetDir)

	// Patch ._pth file to enable site-packages (required for embeddable dist)
	patchDone := telemetry.StartSpan("python.install.patch_pth")
	if err := m.patchPthFile(targetDir); err != nil {
		patchDone("status", "error", "error", err.Error())
		pterm.Warning.Printf("Failed to patch ._pth files: %v\n", err)
	} else {
		patchDone("status", "ok")
	}

	// Bootstrap pip for embeddable distribution
	pterm.Info.Println("Bootstrapping pip...")
	pipDone := telemetry.StartSpan("python.install.bootstrap_pip", "version", version)
	if err := m.BootstrapPip(version); err != nil {
		pipDone("status", "error", "error", err.Error())
		pterm.Warning.Printf("Pip bootstrap failed: %v\n", err)
	} else {
		pipDone("status", "ok")
	}

	// Add to PATH (both Root and Scripts/bin)
	if exe, err := m.GetPythonExe(version); err == nil {
		pythonDir := filepath.Dir(exe)
		utils.AddToPath(pythonDir)
		utils.AddToPath(filepath.Join(pythonDir, "bin"))
		pterm.Success.Printf("Added Python %s to PATH.\n", version)
	}

	return nil
}

func resolveLinuxStandaloneAsset(version string) (string, string, error) {
	release, err := fetchLatestStandaloneRelease()
	if err != nil {
		return "", "", err
	}
	return selectLinuxStandaloneAsset(version, release.Assets)
}

func fetchLatestStandaloneRelease() (standaloneRelease, error) {
	req, err := http.NewRequest(http.MethodGet, linuxStandaloneLatestReleaseAPI, nil)
	if err != nil {
		return standaloneRelease{}, err
	}
	req.Header.Set("User-Agent", "xe")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return standaloneRelease{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return standaloneRelease{}, fmt.Errorf("github releases API failed: %s (%s)", resp.Status, strings.TrimSpace(string(body)))
	}

	var release standaloneRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return standaloneRelease{}, err
	}

	return release, nil
}

func selectLinuxStandaloneAsset(version string, assets []standaloneAsset) (string, string, error) {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid python version %q", version)
	}

	type candidate struct {
		version string
		url     string
	}

	versionPrefix := parts[0] + "." + parts[1] + "."
	exactRequested := len(parts) >= 3
	candidates := make([]candidate, 0)

	for _, asset := range assets {
		m := linuxStandaloneAssetPattern.FindStringSubmatch(asset.Name)
		if len(m) < 2 {
			continue
		}
		candidateVersion := m[1]
		if exactRequested {
			if candidateVersion == version {
				return candidateVersion, asset.BrowserDownloadURL, nil
			}
			continue
		}
		if strings.HasPrefix(candidateVersion, versionPrefix) {
			candidates = append(candidates, candidate{
				version: candidateVersion,
				url:     asset.BrowserDownloadURL,
			})
		}
	}

	if exactRequested {
		return "", "", fmt.Errorf("no standalone build found for python %s on x86_64 linux", version)
	}
	if len(candidates) == 0 {
		return "", "", fmt.Errorf("no standalone builds found for python %s on x86_64 linux", version)
	}

	sort.Slice(candidates, func(i, j int) bool {
		return compareVersion(candidates[i].version, candidates[j].version) > 0
	})

	return candidates[0].version, candidates[0].url, nil
}

func resolveLatestPatchVersion(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) >= 3 {
		return version
	}
	if len(parts) != 2 {
		return version
	}

	candidates := listPatchVersions(version)
	if len(candidates) == 0 {
		return version + ".0"
	}

	sort.Slice(candidates, func(i, j int) bool {
		return compareVersion(candidates[i], candidates[j]) > 0
	})
	return candidates[0]
}

func listPatchVersions(version string) []string {
	base := "https://www.python.org/ftp/python/"
	resp, err := http.Get(base)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	re := regexp.MustCompile(`href="(\d+\.\d+\.\d+)/"`)
	matches := re.FindAllStringSubmatch(string(body), -1)
	candidates := make([]string, 0, len(matches))
	prefix := version + "."
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		v := m[1]
		if strings.HasPrefix(v, prefix) {
			candidates = append(candidates, v)
		}
	}
	return candidates
}

func resolveLatestWindowsInstallerVersion(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) >= 3 {
		return version
	}
	if len(parts) != 2 {
		return version
	}

	latest := resolveLatestPatchVersion(version)
	if windowsInstallerExists(latest) {
		return latest
	}

	if fallback := windowsInstallerFallback(version); fallback != "" {
		if windowsInstallerExists(fallback) {
			return fallback
		}
	}

	candidates := listPatchVersions(version)
	sort.Slice(candidates, func(i, j int) bool {
		return compareVersion(candidates[i], candidates[j]) > 0
	})
	for _, v := range candidates {
		if windowsInstallerExists(v) {
			return v
		}
	}

	if fallback := windowsInstallerFallback(version); fallback != "" {
		return fallback
	}
	if latest != "" {
		return latest
	}
	return version + ".0"
}

func windowsInstallerFallback(version string) string {
	switch version {
	case "3.9":
		return "3.9.13"
	case "3.10":
		return "3.10.11"
	case "3.11":
		return "3.11.9"
	case "3.12":
		return "3.12.3"
	case "3.13":
		return "3.13.0"
	default:
		return ""
	}
}

func windowsInstallerExists(version string) bool {
	if strings.TrimSpace(version) == "" {
		return false
	}
	url := fmt.Sprintf("https://www.python.org/ftp/python/%s/python-%s-amd64.exe", version, version)
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func compareVersion(a, b string) int {
	pa := strings.Split(a, ".")
	pb := strings.Split(b, ".")
	n := len(pa)
	if len(pb) > n {
		n = len(pb)
	}
	for i := 0; i < n; i++ {
		va := 0
		vb := 0
		if i < len(pa) {
			va, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			vb, _ = strconv.Atoi(pb[i])
		}
		if va > vb {
			return 1
		}
		if va < vb {
			return -1
		}
	}
	return 0
}

func windowsLauncherSelector(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return version
}

func isWindowsLauncherVersionAvailable(version string) bool {
	if runtime.GOOS != "windows" {
		return false
	}

	selector := windowsLauncherSelector(version)
	if strings.TrimSpace(selector) == "" {
		return false
	}

	candidates := []string{"py"}
	if winDir := os.Getenv("WINDIR"); winDir != "" {
		candidates = append(candidates, filepath.Join(winDir, "py.exe"))
	}
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		candidates = append(candidates, filepath.Join(localAppData, "Programs", "Python", "Launcher", "py.exe"))
	}

	for _, candidate := range candidates {
		cmd := exec.Command(candidate, "-"+selector, "-V")
		if err := cmd.Run(); err == nil {
			return true
		}
	}

	return false
}

func cleanupWindowsEmbeddableArtifacts(targetDir string) error {
	if runtime.GOOS != "windows" {
		return nil
	}

	entries, err := os.ReadDir(targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if strings.HasSuffix(strings.ToLower(entry.Name()), "._pth") {
			telemetry.Event("python.install.windows.cleanup_embeddable", "target", targetDir)
			return os.RemoveAll(targetDir)
		}
	}
	return nil
}

func isPythonRuntimeHealthy(exe string) bool {
	if strings.TrimSpace(exe) == "" {
		return false
	}

	cmd := exec.Command(exe, "-c", "import encodings,site; print('ok')")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "ok")
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

func (m *PythonManager) GetSitePackagesDir(version string) (string, error) {
	libRoot, err := m.GetLibRoot(version)
	if err != nil {
		return "", err
	}

	if runtime.GOOS == "windows" {
		site := filepath.Join(libRoot, "Lib", "site-packages")
		if err := os.MkdirAll(site, 0755); err != nil {
			return "", err
		}
		return site, nil
	}

	parts := strings.Split(version, ".")
	majorMinor := version
	if len(parts) >= 2 {
		majorMinor = parts[0] + "." + parts[1]
	}
	site := filepath.Join(libRoot, "lib", "python"+majorMinor, "site-packages")
	if err := os.MkdirAll(site, 0755); err != nil {
		return "", err
	}
	return site, nil
}

func (m *PythonManager) GetEffectivePythonExe(version string) (string, error) {
	libRoot, _ := m.GetLibRoot(version)
	if libRoot != "" {
		m.patchPthFile(libRoot)
	}
	return m.GetPythonExe(version)
}

func (m *PythonManager) RunPython(version string, args ...string) (output []byte, retErr error) {
	arg0 := ""
	if len(args) > 0 {
		arg0 = args[0]
	}
	done := telemetry.StartSpan("python.run", "version", version, "arg0", arg0, "arg_count", len(args))
	defer func() {
		fields := []any{"status", "ok", "output_bytes", len(output)}
		if retErr != nil {
			fields[1] = "error"
			fields = append(fields, "error", retErr.Error())
		}
		done(fields...)
	}()

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

	output, retErr = cmd.CombinedOutput()
	return output, retErr
}

func (m *PythonManager) EnsurePath(version string) error {
	_, err := m.GetPythonExe(version)
	return err
}

package resolver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"xe/src/internal/python"
	"xe/src/internal/venv"

	"github.com/codeclysm/extract/v3"
	"github.com/pterm/pterm"
)

type Package struct {
	Name        string
	Version     string
	DownloadURL string
	Hash        string
}

type PipReport struct {
	Install []PipInstallItem `json:"install"`
}

type PipInstallItem struct {
	Metadata     PipMetadata     `json:"metadata"`
	DownloadInfo PipDownloadInfo `json:"download_info"`
}

type PipMetadata struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type PipDownloadInfo struct {
	Url         string         `json:"url"`
	ArchiveInfo PipArchiveInfo `json:"archive_info"`
}

type PipArchiveInfo struct {
	Hashes map[string]string `json:"hashes"`
}

type Resolver struct {
	MaxJobs int
	Cache   string
}

func NewResolver() *Resolver {
	maxJobs := runtime.NumCPU()
	if maxJobs < 1 {
		maxJobs = 1
	}
	return &Resolver{
		MaxJobs: maxJobs,
	}
}

func (r *Resolver) Resolve(pkgName string, pythonVersion string) ([]Package, error) {
	pm, _ := python.NewPythonManager()

	// Use a temporary file for the report to avoid stdout encoding issues
	tempFile, err := os.CreateTemp("", "xe-report-*.json")
	if err != nil {
		return nil, err
	}
	reportPath := tempFile.Name()
	tempFile.Close()
	defer os.Remove(reportPath)

	// Use pip install --report to get all dependencies in one go (dry-run)
	output, err := pm.RunPython(pythonVersion, "-m", "pip", "install", pkgName, "--dry-run", "--report", reportPath)
	if err != nil {
		return nil, fmt.Errorf("dependency resolution failed: %v, output: %s", err, string(output))
	}

	reportData, err := os.ReadFile(reportPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read pip report: %v", err)
	}

	var report PipReport
	if err := json.Unmarshal(reportData, &report); err != nil {
		return nil, fmt.Errorf("failed to parse pip report: %v", err)
	}

	var packages []Package
	for _, item := range report.Install {
		hash := item.DownloadInfo.ArchiveInfo.Hashes["sha256"]
		packages = append(packages, Package{
			Name:        item.Metadata.Name,
			Version:     item.Metadata.Version,
			DownloadURL: item.DownloadInfo.Url,
			Hash:        hash,
		})
	}

	return packages, nil
}

func (r *Resolver) DownloadParallel(packages []Package, version string) error {
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, r.MaxJobs)
	errChan := make(chan error, len(packages))

	multi := pterm.DefaultMultiPrinter
	multi.Start()
	defer multi.Stop()

	for _, pkg := range packages {
		wg.Add(1)

		// Create a spinner for each package in the multi-printer
		spinner, _ := pterm.DefaultSpinner.WithWriter(multi.NewWriter()).WithText("Installing " + pkg.Name + " (" + pkg.Version + ")...").Start()

		go func(p Package, sp *pterm.SpinnerPrinter) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			err := r.downloadAndInstallPackage(p, version)
			if err != nil {
				sp.Fail("Failed: " + p.Name + " (" + err.Error() + ")")
				errChan <- err
				return
			}
			sp.Success("Installed " + p.Name + " (" + p.Version + ")")
		}(pkg, spinner)
	}

	wg.Wait()
	close(errChan)

	if len(errChan) > 0 {
		return <-errChan
	}

	return nil
}

func (r *Resolver) downloadAndInstallPackage(pkg Package, version string) error {
	if pkg.DownloadURL == "" {
		return fmt.Errorf("no download URL for %s", pkg.Name)
	}

	// 3. Download to cache
	home, _ := os.UserHomeDir()
	cacheDir := filepath.Join(home, ".xe", "cache")
	os.MkdirAll(cacheDir, 0755)

	cachePath := filepath.Join(cacheDir, pkg.Name+"-"+pkg.Version+".whl")

	// Check if already cached and valid
	cached := false
	if _, err := os.Stat(cachePath); err == nil {
		if pkg.Hash != "" {
			if valid, _ := verifyChecksum(cachePath, pkg.Hash); valid {
				cached = true
			}
		} else {
			cached = true // optimistically assume valid if no hash provided (legacy fallback)
		}
	}

	if !cached {
		resp, err := http.Get(pkg.DownloadURL)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		out, err := os.Create(cachePath)
		if err != nil {
			return err
		}

		if _, err = io.Copy(out, resp.Body); err != nil {
			out.Close()
			return err
		}
		out.Close()

		// Verify download
		if pkg.Hash != "" {
			if valid, err := verifyChecksum(cachePath, pkg.Hash); !valid {
				return fmt.Errorf("checksum mismatch for %s: %v", pkg.Name, err)
			}
		}
	}

	// 4. Extract to site-packages
	vm, _ := venv.NewVenvManager()
	pm, _ := python.NewPythonManager()

	globalPath := pm.GetPythonPath(version)
	effectivePath := vm.GetEffectivePythonPath(globalPath)

	sitePackages := findSitePackages(effectivePath)
	if sitePackages == "" {
		sitePackages = filepath.Join(effectivePath, "Lib", "site-packages")
		os.MkdirAll(sitePackages, 0755)
	}

	f, err := os.Open(cachePath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := extract.Archive(context.Background(), f, sitePackages, nil); err != nil {
		return fmt.Errorf("failed to extract %s: %v", pkg.Name, err)
	}

	return nil
}

type Toolchain struct {
	MSVC    string
	UCRT    string
	Version string
}

func GetCurrentToolchain() Toolchain {
	// In a real implementation, this would detect MSVC and UCRT versions
	return Toolchain{
		MSVC: "19.38",
		UCRT: "10.0.22621",
	}
}

func verifyChecksum(path, expectedHash string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return false, err
	}

	actualHash := hex.EncodeToString(hasher.Sum(nil))
	return actualHash == expectedHash, nil
}


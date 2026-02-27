package engine

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"xe/src/internal/cache"
	"xe/src/internal/project"
	"xe/src/internal/python"
	"xe/src/internal/resolver"
	"xe/src/internal/telemetry"

	"github.com/codeclysm/extract/v3"
)

type Installer struct {
	Resolver *resolver.Resolver
	CAS      *cache.CAS
}

type SolveGraph struct {
	PythonVersion string             `json:"python_version"`
	Requirements  []string           `json:"requirements"`
	Packages      []resolver.Package `json:"packages"`
}

func NewInstaller(globalCacheDir string) (*Installer, error) {
	cas, err := cache.New(globalCacheDir)
	if err != nil {
		return nil, err
	}
	return &Installer{
		Resolver: resolver.NewResolver(),
		CAS:      cas,
	}, nil
}

func (i *Installer) Install(ctx context.Context, cfg project.Config, requirements []string, projectDir string, installSitePackages string) (result []resolver.Package, retErr error) {
	done := telemetry.StartSpan(
		"install.total",
		"python_version", cfg.Python.Version,
		"raw_requirements", len(requirements),
	)
	defer func() {
		fields := []any{"status", "ok", "resolved_packages", len(result)}
		if retErr != nil {
			fields[1] = "error"
			fields = append(fields, "error", retErr.Error())
		}
		done(fields...)
	}()

	// CLI Entry -> Config Loader -> Requirements Parser
	reqs := normalizeRequirements(requirements)
	if len(reqs) == 0 {
		return nil, nil
	}

	// Resolve Cache Hit?
	cacheKey := solveKey(cfg.Python.Version, reqs)
	var graph SolveGraph
	cacheDone := telemetry.StartSpan("install.solution_cache.load")
	hit, err := i.CAS.LoadSolution(cacheKey, &graph)
	if err != nil {
		cacheDone("status", "error", "error", err.Error())
		retErr = err
		return nil, retErr
	}
	cacheDone("status", "ok", "hit", hit, "package_count", len(graph.Packages))

	if !hit {
		// Parallel Dependency Resolver
		resolveDone := telemetry.StartSpan("install.resolve_parallel", "requirements", len(reqs))
		solved, err := i.resolveParallel(ctx, cfg.Python.Version, reqs)
		if err != nil {
			resolveDone("status", "error", "error", err.Error())
			retErr = err
			return nil, retErr
		}
		resolveDone("status", "ok", "resolved_packages", len(solved))

		// Speculative Solve Engine + Store Solution Cache
		graph = SolveGraph{
			PythonVersion: cfg.Python.Version,
			Requirements:  reqs,
			Packages:      dedupePackages(solved),
		}
		saveDone := telemetry.StartSpan("install.solution_cache.save", "package_count", len(graph.Packages))
		if err := i.CAS.SaveSolution(cacheKey, graph); err != nil {
			saveDone("status", "error", "error", err.Error())
			retErr = err
			return nil, retErr
		}
		saveDone("status", "ok")
	}

	// Load Pre-Solved Graph -> Predictive Scheduler -> Download Planner
	planDone := telemetry.StartSpan("install.download_plan.build", "packages", len(graph.Packages))
	downloadPlan := make([]resolver.Package, len(graph.Packages))
	copy(downloadPlan, graph.Packages)
	sort.Slice(downloadPlan, func(a, b int) bool {
		return downloadPlan[a].Name < downloadPlan[b].Name
	})
	planDone("status", "ok")

	if strings.TrimSpace(installSitePackages) == "" {
		targetDone := telemetry.StartSpan("install.target_site_packages.resolve", "python_version", cfg.Python.Version)
		pm, pmErr := python.NewPythonManager()
		if pmErr == nil {
			site, siteErr := pm.GetSitePackagesDir(cfg.Python.Version)
			if siteErr == nil {
				installSitePackages = site
			}
		}
		if strings.TrimSpace(installSitePackages) == "" {
			installSitePackages = filepath.Join(projectDir, "xe", "site-packages")
		}
		targetDone("status", "ok", "site_packages", installSitePackages)
	}
	if err := os.MkdirAll(installSitePackages, 0755); err != nil {
		retErr = err
		return nil, retErr
	}

	workers := runtime.NumCPU() * 2
	if workers < 2 {
		workers = 2
	}
	extractWorkers := extractionWorkers()
	workersDone := telemetry.StartSpan(
		"install.download_and_extract",
		"workers", workers,
		"extract_workers", extractWorkers,
		"packages", len(downloadPlan),
	)
	workersStatus := "ok"
	defer func() {
		workersDone("status", workersStatus)
	}()

	jobs := make(chan resolver.Package)
	errCh := make(chan error, len(downloadPlan))
	extractSem := make(chan struct{}, extractWorkers)
	var wg sync.WaitGroup

	for workerIdx := 0; workerIdx < workers; workerIdx++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pkg := range jobs {
				pkgDone := telemetry.StartSpan("install.package", "name", pkg.Name, "version", pkg.Version)
				if isInstalledInSitePackages(installSitePackages, pkg) {
					pkgDone("status", "skipped", "reason", "already_installed")
					continue
				}
				if pkg.DownloadURL == "" {
					pkgDone("status", "skipped", "reason", "missing_download_url")
					continue
				}
				downloadDone := telemetry.StartSpan("install.package.download", "name", pkg.Name)
				blob, err := i.CAS.StoreBlobFromURL(pkg.DownloadURL, pkg.Hash)
				if err != nil {
					downloadDone("status", "error", "error", err.Error())
					pkgDone("status", "error", "stage", "download", "error", err.Error())
					errCh <- fmt.Errorf("download %s: %w", pkg.Name, err)
					continue
				}
				downloadDone("status", "ok")

				extractSem <- struct{}{}
				extractDone := telemetry.StartSpan("install.package.extract", "name", pkg.Name)
				if err := installWheelBlob(blob, installSitePackages); err != nil {
					<-extractSem
					extractDone("status", "error", "error", err.Error())
					pkgDone("status", "error", "stage", "extract", "error", err.Error())
					errCh <- fmt.Errorf("install %s: %w", pkg.Name, err)
					continue
				}
				<-extractSem
				extractDone("status", "ok")
				pkgDone("status", "ok")
			}
		}()
	}

	for _, pkg := range downloadPlan {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			workersStatus = "error"
			retErr = ctx.Err()
			return nil, retErr
		case jobs <- pkg:
		}
	}
	close(jobs)
	wg.Wait()
	close(errCh)
	if len(errCh) > 0 {
		workersStatus = "error"
		retErr = <-errCh
		return nil, retErr
	}

	// Environment Linker / Post Install Hooks are represented by runtime wiring in `xe run`.
	result = graph.Packages
	return result, nil
}

func (i *Installer) resolveParallel(ctx context.Context, pythonVersion string, reqs []string) ([]resolver.Package, error) {
	done := telemetry.StartSpan("resolve.total", "requirements", len(reqs), "python_version", pythonVersion)
	var (
		mu       sync.Mutex
		all      []resolver.Package
		wg       sync.WaitGroup
		firstErr error
	)
	for _, req := range reqs {
		r := req
		wg.Add(1)
		go func() {
			defer wg.Done()
			reqDone := telemetry.StartSpan("resolve.requirement", "requirement", r)
			pkgs, err := i.Resolver.Resolve(r, pythonVersion)
			mu.Lock()
			defer mu.Unlock()
			if err != nil && firstErr == nil {
				firstErr = err
				reqDone("status", "error", "error", err.Error())
				return
			}
			all = append(all, pkgs...)
			reqDone("status", "ok", "packages", len(pkgs))
		}()
	}
	wg.Wait()
	if firstErr != nil {
		done("status", "error", "error", firstErr.Error())
		return nil, firstErr
	}
	done("status", "ok", "packages", len(all))
	return all, nil
}

func installWheelBlob(blobPath, sitePackages string) error {
	f, err := os.Open(blobPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return extract.Archive(context.Background(), f, sitePackages, nil)
}

func extractionWorkers() int {
	workers := runtime.NumCPU() / 2
	if workers < 1 {
		workers = 1
	}
	if workers > 4 {
		workers = 4
	}
	return workers
}

func normalizePackageIdentity(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.ReplaceAll(n, "-", "_")
	n = strings.ReplaceAll(n, ".", "_")
	return n
}

func isInstalledInSitePackages(sitePackages string, pkg resolver.Package) bool {
	entries, err := os.ReadDir(sitePackages)
	if err != nil {
		return false
	}

	targetName := normalizePackageIdentity(pkg.Name)
	targetVersion := strings.TrimSpace(pkg.Version)
	if targetName == "" || targetVersion == "" {
		return false
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".dist-info") {
			continue
		}
		base := strings.TrimSuffix(name, ".dist-info")
		sep := strings.LastIndex(base, "-")
		if sep <= 0 || sep >= len(base)-1 {
			continue
		}
		installedName := normalizePackageIdentity(base[:sep])
		installedVersion := strings.TrimSpace(base[sep+1:])
		if installedName == targetName && installedVersion == targetVersion {
			return true
		}
	}
	return false
}

func solveKey(pythonVersion string, reqs []string) string {
	h := sha1.New()
	h.Write([]byte(pythonVersion))
	h.Write([]byte("|"))
	for _, r := range reqs {
		h.Write([]byte(r))
		h.Write([]byte("|"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func normalizeRequirements(reqs []string) []string {
	out := make([]string, 0, len(reqs))
	for _, r := range reqs {
		trimmed := strings.TrimSpace(r)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	sort.Strings(out)
	return out
}

func dedupePackages(pkgs []resolver.Package) []resolver.Package {
	seen := map[string]resolver.Package{}
	for _, p := range pkgs {
		key := strings.ToLower(p.Name) + "==" + p.Version
		seen[key] = p
	}
	out := make([]resolver.Package, 0, len(seen))
	for _, p := range seen {
		out = append(out, p)
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].Name == out[b].Name {
			return out[a].Version < out[b].Version
		}
		return out[a].Name < out[b].Name
	})
	return out
}

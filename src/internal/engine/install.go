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

func (i *Installer) Install(ctx context.Context, cfg project.Config, requirements []string, projectDir string, installSitePackages string) ([]resolver.Package, error) {
	// CLI Entry -> Config Loader -> Requirements Parser
	reqs := normalizeRequirements(requirements)
	if len(reqs) == 0 {
		return nil, nil
	}

	// Resolve Cache Hit?
	cacheKey := solveKey(cfg.Python.Version, reqs)
	var graph SolveGraph
	hit, err := i.CAS.LoadSolution(cacheKey, &graph)
	if err != nil {
		return nil, err
	}

	if !hit {
		// Parallel Dependency Resolver
		solved, err := i.resolveParallel(ctx, cfg.Python.Version, reqs)
		if err != nil {
			return nil, err
		}
		// Speculative Solve Engine + Store Solution Cache
		graph = SolveGraph{
			PythonVersion: cfg.Python.Version,
			Requirements:  reqs,
			Packages:      dedupePackages(solved),
		}
		if err := i.CAS.SaveSolution(cacheKey, graph); err != nil {
			return nil, err
		}
	}

	// Load Pre-Solved Graph -> Predictive Scheduler -> Download Planner
	downloadPlan := make([]resolver.Package, len(graph.Packages))
	copy(downloadPlan, graph.Packages)
	sort.Slice(downloadPlan, func(a, b int) bool {
		return downloadPlan[a].Name < downloadPlan[b].Name
	})

	if strings.TrimSpace(installSitePackages) == "" {
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
	}

	workers := runtime.NumCPU() * 2
	if workers < 2 {
		workers = 2
	}
	jobs := make(chan resolver.Package)
	errCh := make(chan error, len(downloadPlan))
	var wg sync.WaitGroup

	for workerIdx := 0; workerIdx < workers; workerIdx++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pkg := range jobs {
				if pkg.DownloadURL == "" {
					continue
				}
				blob, err := i.CAS.StoreBlobFromURL(pkg.DownloadURL, pkg.Hash)
				if err != nil {
					errCh <- fmt.Errorf("download %s: %w", pkg.Name, err)
					continue
				}
				if err := installWheelBlob(blob, installSitePackages); err != nil {
					errCh <- fmt.Errorf("install %s: %w", pkg.Name, err)
				}
			}
		}()
	}

	for _, pkg := range downloadPlan {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return nil, ctx.Err()
		case jobs <- pkg:
		}
	}
	close(jobs)
	wg.Wait()
	close(errCh)
	if len(errCh) > 0 {
		return nil, <-errCh
	}

	// Environment Linker / Post Install Hooks are represented by runtime wiring in `xe run`.
	return graph.Packages, nil
}

func (i *Installer) resolveParallel(ctx context.Context, pythonVersion string, reqs []string) ([]resolver.Package, error) {
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
			pkgs, err := i.Resolver.Resolve(r, pythonVersion)
			mu.Lock()
			defer mu.Unlock()
			if err != nil && firstErr == nil {
				firstErr = err
				return
			}
			all = append(all, pkgs...)
		}()
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return all, nil
}

func installWheelBlob(blobPath, sitePackages string) error {
	if err := os.MkdirAll(sitePackages, 0755); err != nil {
		return err
	}
	f, err := os.Open(blobPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return extract.Archive(context.Background(), f, sitePackages, nil)
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

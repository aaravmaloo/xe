package resolver

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pterm/pterm"
)

type PackageMetadata struct {
	Name        string
	Version     string
	Summary     string
	HomePage    string
	Author      string
	AuthorEmail string
	License     string
	Location    string
	Requires    []string
	RequiredBy  []string
}

func ParseMetadataFile(path string) (*PackageMetadata, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	meta := &PackageMetadata{
		Location: filepath.Dir(filepath.Dir(path)),
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) < 2 {
			continue
		}
		key, value := parts[0], parts[1]
		switch key {
		case "Name":
			meta.Name = value
		case "Version":
			meta.Version = value
		case "Summary":
			meta.Summary = value
		case "Author":
			meta.Author = value
		case "Author-email":
			meta.AuthorEmail = value
		case "License":
			meta.License = value
		case "Home-page":
			meta.HomePage = value
		case "Requires-Dist":
			// Simplified parsing for Requires-Dist
			dep := strings.Split(value, " ")[0]
			meta.Requires = append(meta.Requires, dep)
		}
	}
	return meta, scanner.Err()
}

func GetInstalledPackageMetadata(pythonPath, pkgName string) (*PackageMetadata, error) {
	sitePackages := findSitePackages(pythonPath)
	if sitePackages == "" {
		return nil, fmt.Errorf("site-packages not found in %s", pythonPath)
	}

	pterm.Debug.Printf("Checking for %s in %s\n", pkgName, sitePackages)

	files, err := os.ReadDir(sitePackages)
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		if f.IsDir() && strings.HasPrefix(strings.ToLower(f.Name()), strings.ToLower(pkgName)) && strings.HasSuffix(f.Name(), ".dist-info") {
			metadataPath := filepath.Join(sitePackages, f.Name(), "METADATA")
			if _, err := os.Stat(metadataPath); err == nil {
				return ParseMetadataFile(metadataPath)
			}
		}
	}

	return nil, fmt.Errorf("package %s not found in %s", pkgName, sitePackages)
}

func ListInstalledPackages(pythonPath string) ([]PackageMetadata, error) {
	sitePackages := findSitePackages(pythonPath)
	if sitePackages == "" {
		pterm.Debug.Printf("No site-packages found in %s\n", pythonPath)
		return []PackageMetadata{}, nil
	}

	pterm.Debug.Printf("Listing packages in %s\n", sitePackages)

	files, err := os.ReadDir(sitePackages)
	if err != nil {
		return nil, err
	}

	var packages []PackageMetadata
	for _, f := range files {
		if f.IsDir() && strings.HasSuffix(f.Name(), ".dist-info") {
			metadataPath := filepath.Join(sitePackages, f.Name(), "METADATA")
			meta, err := ParseMetadataFile(metadataPath)
			if err == nil {
				packages = append(packages, *meta)
			}
		}
	}
	return packages, nil
}

func findSitePackages(pythonPath string) string {
	// Common layouts:
	// 1. venv: pythonPath/Lib/site-packages
	// 2. nuget: pythonPath/site-packages
	// 3. unix: pythonPath/lib/pythonX.Y/site-packages

	paths := []string{
		filepath.Join(pythonPath, "Lib", "site-packages"),
		filepath.Join(pythonPath, "site-packages"),
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

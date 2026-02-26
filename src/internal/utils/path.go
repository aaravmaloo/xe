package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"xe/src/internal/xedir"

	"github.com/pterm/pterm"
)

func AddToPath(dir string) error {
	pterm.Info.Printf("Ensuring %s is in system PATH...\n", dir)

	if runtime.GOOS == "windows" {

		script := fmt.Sprintf(`
			$targetDir = '%s'
			$currentPath = [Environment]::GetEnvironmentVariable('Path', 'User')
			if ($null -eq $currentPath) { $currentPath = '' }
			
			$pathArray = $currentPath.Split(';', [System.StringSplitOptions]::RemoveEmptyEntries)
			if ($pathArray -notcontains $targetDir) {
				$newPath = ($pathArray + $targetDir) -join ';'
				[Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
				Write-Output "Added to PATH"
			} else {
				Write-Output "Already in PATH"
			}
		`, dir)

		return exec.Command("powershell", "-Command", script).Run()
	} else {
		// For Linux, we suggest adding to shell profile
		pterm.Warning.Println("Automatic PATH update on Linux is limited. Please ensure the following is in your .bashrc or .zshrc:")
		pterm.Info.Printf("export PATH=\"$PATH:%s\"\n", dir)
		return nil
	}
}

func CreateShim(name, target string) error {
	shimDir := xedir.ShimDir()
	if err := os.MkdirAll(shimDir, 0755); err != nil {
		return err
	}

	if runtime.GOOS == "windows" {
		shimPath := filepath.Join(shimDir, name+".bat")
		content := fmt.Sprintf("@echo off\n\"%s\" %%*", target)
		return os.WriteFile(shimPath, []byte(content), 0755)
	} else {
		shimPath := filepath.Join(shimDir, name)
		content := fmt.Sprintf("#!/bin/sh\nexec \"%s\" \"$@\"", target)
		err := os.WriteFile(shimPath, []byte(content), 0755)
		if err != nil {
			return err
		}
		return os.Chmod(shimPath, 0755)
	}
}

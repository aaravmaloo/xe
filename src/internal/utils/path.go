package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pterm/pterm"
)

func AddToPath(dir string) error {
	pterm.Info.Printf("Ensuring %s is in system PATH...\n", dir)

	// Use powershell to check and append to the User PATH persistently
	// This command checks if the directory exists in the User Path and appends it if not.
	script := fmt.Sprintf(`
		$dir = "%s"
		$oldPath = [Environment]::GetEnvironmentVariable("Path", "User")
		if ($oldPath -split ";" -notcontains $dir) {
			$newPath = $oldPath + (if ($oldPath.EndsWith(";")) {""} else {";"}) + $dir
			[Environment]::SetEnvironmentVariable("Path", $newPath, "User")
			Write-Output "Added to PATH"
		} else {
			Write-Output "Already in PATH"
		}
	`, dir)

	cmd := exec.Command("powershell", "-Command", script)
	return cmd.Run()
}

func CreateShim(name, target string) error {
	home, _ := os.UserHomeDir()
	shimDir := filepath.Join(home, ".xe", "bin")
	if err := os.MkdirAll(shimDir, 0755); err != nil {
		return err
	}

	shimPath := filepath.Join(shimDir, name+".bat")
	content := fmt.Sprintf("@echo off\n\"%s\" %%*", target)
	return os.WriteFile(shimPath, []byte(content), 0755)
}

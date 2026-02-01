package core

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func CreateSnapshot(name string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	xeDir := filepath.Join(home, ".xe")
	snapsDir := filepath.Join(xeDir, "snaps")
	if err := os.MkdirAll(snapsDir, 0755); err != nil {
		return err
	}

	snapPath := filepath.Join(snapsDir, fmt.Sprintf("%s_%d.zip", name, time.Now().Unix()))

	// Create zip of the .xe directory (excluding snaps themselves)
	return zipDirectory(xeDir, snapPath, []string{"snaps"})
}

func RestoreSnapshot(name string) error {
	// Logic to unzip and replace current .xe state
	return nil
}

func zipDirectory(source, target string, exclude []string) error {
	zipfile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer zipfile.Close()

	archive := zip.NewWriter(zipfile)
	defer archive.Close()

	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Handle exclusions
		for _, ex := range exclude {
			if strings.Contains(path, ex) && path != source {
				return nil
			}
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		header.Name, err = filepath.Rel(source, path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(writer, file)
		return err
	})
}

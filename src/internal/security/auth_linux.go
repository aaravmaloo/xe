//go:build linux

package security

import (
	"os"
	"path/filepath"
)

const CredentialTarget = "xe_pypi_token"

func getCredPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".xe")
	os.MkdirAll(dir, 0700)
	return filepath.Join(dir, "credentials"), nil
}

func SaveToken(token string) error {
	path, err := getCredPath()
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(token), 0600)
}

func GetToken() (string, error) {
	path, err := getCredPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func RevokeToken() error {
	path, err := getCredPath()
	if err != nil {
		return err
	}
	return os.Remove(path)
}

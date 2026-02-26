//go:build linux

package security

import (
	"os"
	"path/filepath"
	"xe/src/internal/xedir"
)

const CredentialTarget = "xe_pypi_token"

func getCredPath() (string, error) {
	dir := xedir.MustHome()
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

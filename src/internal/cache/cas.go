package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type CAS struct {
	Root string
}

func New(root string) (*CAS, error) {
	c := &CAS{Root: root}
	if err := os.MkdirAll(c.blobDir(), 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(c.solutionDir(), 0755); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *CAS) StoreBlobFromURL(url, expectedSha256 string) (string, error) {
	if expectedSha256 != "" {
		target := c.blobPath(expectedSha256)
		if _, err := os.Stat(target); err == nil {
			return target, nil
		}
	}

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: %s", resp.Status)
	}

	tmp, err := os.CreateTemp(c.Root, "xe-download-*")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, hash), resp.Body); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}

	actual := hex.EncodeToString(hash.Sum(nil))
	if expectedSha256 != "" && !strings.EqualFold(expectedSha256, actual) {
		return "", fmt.Errorf("checksum mismatch: expected=%s actual=%s", expectedSha256, actual)
	}

	target := c.blobPath(actual)
	if _, err := os.Stat(target); err == nil {
		return target, nil
	}
	if err := os.Rename(tmpPath, target); err != nil {
		return "", err
	}
	return target, nil
}

func (c *CAS) SaveSolution(key string, value any) error {
	p := filepath.Join(c.solutionDir(), key+".json")
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(value)
}

func (c *CAS) LoadSolution(key string, out any) (bool, error) {
	p := filepath.Join(c.solutionDir(), key+".json")
	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer f.Close()
	return true, json.NewDecoder(f).Decode(out)
}

func (c *CAS) blobDir() string {
	return filepath.Join(c.Root, "cas", "blobs")
}

func (c *CAS) solutionDir() string {
	return filepath.Join(c.Root, "cas", "solutions")
}

func (c *CAS) blobPath(sha string) string {
	prefix := "00"
	if len(sha) >= 2 {
		prefix = sha[:2]
	}
	_ = os.MkdirAll(filepath.Join(c.blobDir(), prefix), 0755)
	return filepath.Join(c.blobDir(), prefix, sha+".whl")
}

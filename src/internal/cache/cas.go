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
	"xe/src/internal/telemetry"
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
	done := telemetry.StartSpan("cas.store_blob", "url", url)
	if expectedSha256 != "" {
		target := c.blobPath(expectedSha256)
		if _, err := os.Stat(target); err == nil {
			done("status", "ok", "cache_hit", true)
			return target, nil
		}
	}

	downloadDone := telemetry.StartSpan("cas.download", "url", url)
	resp, err := http.Get(url)
	if err != nil {
		downloadDone("status", "error", "error", err.Error())
		done("status", "error", "error", err.Error())
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		downloadDone("status", "error", "status", resp.Status)
		err = fmt.Errorf("download failed: %s", resp.Status)
		done("status", "error", "error", err.Error())
		return "", err
	}

	tmp, err := os.CreateTemp(c.Root, "xe-download-*")
	if err != nil {
		downloadDone("status", "error", "error", err.Error())
		done("status", "error", "error", err.Error())
		return "", err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, hash), resp.Body); err != nil {
		tmp.Close()
		downloadDone("status", "error", "error", err.Error())
		done("status", "error", "error", err.Error())
		return "", err
	}
	downloadDone("status", "ok")
	if err := tmp.Close(); err != nil {
		done("status", "error", "error", err.Error())
		return "", err
	}

	actual := hex.EncodeToString(hash.Sum(nil))
	if expectedSha256 != "" && !strings.EqualFold(expectedSha256, actual) {
		err = fmt.Errorf("checksum mismatch: expected=%s actual=%s", expectedSha256, actual)
		done("status", "error", "error", err.Error())
		return "", err
	}

	target := c.blobPath(actual)
	if _, err := os.Stat(target); err == nil {
		done("status", "ok", "cache_hit", true)
		return target, nil
	}
	if err := os.Rename(tmpPath, target); err != nil {
		done("status", "error", "error", err.Error())
		return "", err
	}
	done("status", "ok", "cache_hit", false)
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

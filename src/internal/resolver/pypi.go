package resolver

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// PypiResponse represents the JSON structure from Pypi's JSON API
type PypiResponse struct {
	Info struct {
		Name         string            `json:"name"`
		Version      string            `json:"version"`
		Summary      string            `json:"summary"`
		HomePage     string            `json:"home_page"`
		Author       string            `json:"author"`
		AuthorEmail  string            `json:"author_email"`
		License      string            `json:"license"`
		RequiresDist []string          `json:"requires_dist"`
		ProjectUrls  map[string]string `json:"project_urls"`
	} `json:"info"`
	Releases map[string][]struct {
		Filename string `json:"filename"`
		URL      string `json:"url"`
		Hashes   struct {
			Sha256 string `json:"sha256"`
		} `json:"hashes"`
		Packagetype string `json:"packagetype"`
	} `json:"releases"`
}

func FetchMetadataFromPypi(pkgName string) (*PypiResponse, error) {
	url := fmt.Sprintf("https://pypi.org/pypi/%s/json", pkgName)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("package %s not found on PyPI", pkgName)
	}

	var pypiResp PypiResponse
	if err := json.NewDecoder(resp.Body).Decode(&pypiResp); err != nil {
		return nil, err
	}

	return &pypiResp, nil
}

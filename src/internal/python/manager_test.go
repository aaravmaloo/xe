package python

import "testing"

func TestSelectLinuxStandaloneAssetMajorMinorPicksHighestPatch(t *testing.T) {
	assets := []standaloneAsset{
		{
			Name:               "cpython-3.14.1+20260211-x86_64-unknown-linux-gnu-install_only.tar.gz",
			BrowserDownloadURL: "https://example.invalid/3.14.1",
		},
		{
			Name:               "cpython-3.14.3+20260211-x86_64-unknown-linux-gnu-install_only.tar.gz",
			BrowserDownloadURL: "https://example.invalid/3.14.3",
		},
		{
			Name:               "cpython-3.13.7+20260211-x86_64-unknown-linux-gnu-install_only.tar.gz",
			BrowserDownloadURL: "https://example.invalid/3.13.7",
		},
	}

	version, url, err := selectLinuxStandaloneAsset("3.14", assets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != "3.14.3" {
		t.Fatalf("expected 3.14.3, got %s", version)
	}
	if url != "https://example.invalid/3.14.3" {
		t.Fatalf("unexpected URL: %s", url)
	}
}

func TestSelectLinuxStandaloneAssetExactPatch(t *testing.T) {
	assets := []standaloneAsset{
		{
			Name:               "cpython-3.14.1+20260211-x86_64-unknown-linux-gnu-install_only.tar.gz",
			BrowserDownloadURL: "https://example.invalid/3.14.1",
		},
		{
			Name:               "cpython-3.14.3+20260211-x86_64-unknown-linux-gnu-install_only.tar.gz",
			BrowserDownloadURL: "https://example.invalid/3.14.3",
		},
	}

	version, url, err := selectLinuxStandaloneAsset("3.14.1", assets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != "3.14.1" {
		t.Fatalf("expected 3.14.1, got %s", version)
	}
	if url != "https://example.invalid/3.14.1" {
		t.Fatalf("unexpected URL: %s", url)
	}
}

func TestSelectLinuxStandaloneAssetExactPatchMissing(t *testing.T) {
	assets := []standaloneAsset{
		{
			Name:               "cpython-3.14.3+20260211-x86_64-unknown-linux-gnu-install_only.tar.gz",
			BrowserDownloadURL: "https://example.invalid/3.14.3",
		},
	}

	if _, _, err := selectLinuxStandaloneAsset("3.14.1", assets); err == nil {
		t.Fatal("expected error for missing exact patch")
	}
}

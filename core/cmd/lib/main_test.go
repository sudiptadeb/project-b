package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The binary and the release tool must agree on the project and the manifest URL, and the
// pinned key must be the one the build Mac signs with.
func TestReleaseConstantsMatchTheRepo(t *testing.T) {
	b, err := os.ReadFile("../../../amarit.json")
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Project  string `json:"project"`
		Releases string `json:"releases"`
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Project != releaseProject || cfg.Releases != releaseManifest {
		t.Fatalf("amarit.json says %s %s; the binary says %s %s", cfg.Project, cfg.Releases, releaseProject, releaseManifest)
	}

	home, _ := os.UserHomeDir()
	sha1, err := os.ReadFile(filepath.Join(home, "PW_TOOLS", "project-b-signing", "identity.sha1"))
	if err != nil {
		t.Skip("not the build Mac")
	}
	if got := strings.TrimSpace(string(sha1)); got != signingLeafSHA1 {
		t.Fatalf("the signing key is %s; the binary pins %s", got, signingLeafSHA1)
	}
}

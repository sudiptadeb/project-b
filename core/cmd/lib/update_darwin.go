package main

import (
	"log"
	"os"
	"path/filepath"

	"projectb/update"
)

// signingLeafSHA1 is the certificate tools/package.sh signs with (its SHA-1, as in the trust
// profile): a downloaded bundle must be signed with it.
const signingLeafSHA1 = "59b20dfe02ee393a05173eec84daa8c662c845be"

// updateConfig describes this build to the updater. A binary running from a build directory
// rather than an installed bundle has no version, and never updates.
func updateConfig(profileDir string) update.Config {
	exe, err := os.Executable()
	if err != nil {
		return update.Config{}
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	bundle := update.BundleOf(exe)
	if bundle == "" {
		return update.Config{}
	}
	version, err := update.Version(bundle)
	if err != nil {
		log.Printf("update: %v; updates are off", err)
		return update.Config{}
	}
	return update.Config{
		Project:  releaseProject,
		Version:  version,
		Releases: releaseManifest,
		StateDir: filepath.Join(profileDir, "updates"),
		App:      bundle,
		Verify:   update.SignedBy(signingLeafSHA1),
	}
}

package main

import (
	"os"
	"path/filepath"

	"projectb/profiles"
	"projectb/relaunch"
	"projectb/timeline"
	"projectb/update"
)

// signingLeafSHA1 is the certificate tools/package.sh signs with (its SHA-1, as in the trust
// profile): a downloaded bundle must be signed with it.
const signingLeafSHA1 = "59b20dfe02ee393a05173eec84daa8c662c845be"

// installedBundle is the app bundle this binary runs from, or "" for a binary in a build
// directory.
func installedBundle() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return update.BundleOf(exe)
}

// appVersion is the version the installed bundle says it is, or "" for a build directory.
func appVersion() string {
	bundle := installedBundle()
	if bundle == "" {
		return ""
	}
	version, _ := update.Version(bundle)
	return version
}

// updateConfig describes this build to the updater, for a copy on the profile c. A copy on
// a listed profile shares the root's update folder with every copy on the root, and
// relaunches them all; one on a folder named outright keeps its update state in that folder
// and relaunches alone, so tests and ChromeDriver never touch the copies on the root. A
// binary running from a build directory rather than an installed bundle has no version, and
// never updates; its folder still holds the automatic-updates setting.
func updateConfig(c profiles.Choice) update.Config {
	cfg := update.Config{StateDir: filepath.Join(c.Dir, "updates")}
	if c.ID != "" {
		cfg.StateDir = filepath.Join(list.Root(), "update")
		cfg.Copies = relaunch.Copies{List: list, Profile: c.ID}
	}
	bundle := installedBundle()
	if bundle == "" {
		return cfg
	}
	version, err := update.Version(bundle)
	if err != nil {
		timeline.Errorf("update: %v; updates are off", err)
		return cfg
	}
	cfg.Project = releaseProject
	cfg.Version = version
	cfg.Releases = releaseManifest
	cfg.App = bundle
	cfg.Verify = update.SignedBy(signingLeafSHA1)
	return cfg
}

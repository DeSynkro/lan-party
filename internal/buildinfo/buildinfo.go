// Package buildinfo carries the version and commit stamped into a build.
package buildinfo

// Version is the release version, set at build time:
//
//	-ldflags "-X github.com/desynkro/lan-party/internal/buildinfo.Version=0.1.0"
var Version = "dev"

// Commit is the short git hash of the build, set the same way.
var Commit = "unknown"

// Summary is what the app shows in its title and about page.
func Summary() string {
	if Commit == "unknown" {
		return Version
	}
	return Version + " (" + Commit + ")"
}

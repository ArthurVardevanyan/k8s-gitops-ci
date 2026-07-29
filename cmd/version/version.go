// Package version holds ldflags-injected build metadata.
package version

import "fmt"

// BuildVersion is set by ldflags.
var BuildVersion = "dev"

// BuildTime is set by ldflags.
var BuildTime = "unknown"

// Commit is set by ldflags.
var Commit = "unknown"

// String returns the version line.
func String() string {
	return fmt.Sprintf("k8s-gitops-ci version %s (commit %s, built %s)", BuildVersion, Commit, BuildTime)
}

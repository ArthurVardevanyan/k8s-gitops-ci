// Package version holds ldflags-injected build metadata.
package version

import "fmt"

// BuildVersion is set by ldflags.
var BuildVersion = "dev"

// BuildTime is set by ldflags.
var BuildTime = "unknown"

// Commit is set by ldflags.
var Commit = "unknown"

// String returns the version line with commit and build metadata.
func String() string {
	return fmt.Sprintf("k8s-gitops-ci version %s (commit %s, built %s)", BuildVersion, Commit, BuildTime)
}

// Short returns the version line without commit or build metadata.
func Short() string {
	return fmt.Sprintf("k8s-gitops-ci version %s", BuildVersion)
}

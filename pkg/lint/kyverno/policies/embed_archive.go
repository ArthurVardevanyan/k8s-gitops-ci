//go:build embedschemas

package policies

import "embed"

//go:embed policies.tar.gz
var policiesArchive embed.FS

// archiveBytes returns the embedded policy archive bytes. This variant is
// compiled only with the `embedschemas` build tag.
func archiveBytes() ([]byte, error) {
	return policiesArchive.ReadFile("policies.tar.gz")
}

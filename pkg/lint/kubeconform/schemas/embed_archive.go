//go:build embedschemas

package schemas

import "embed"

//go:embed schemas.tar.gz
var schemasArchive embed.FS

// archiveBytes returns the embedded schema archive bytes. This variant is
// compiled only with the `embedschemas` build tag.
func archiveBytes() ([]byte, error) {
	return schemasArchive.ReadFile("schemas.tar.gz")
}

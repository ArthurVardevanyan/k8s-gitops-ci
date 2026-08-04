//go:build !embedschemas

package schemas

// archiveBytes reports that no schema archive is compiled in. This variant is
// used by default (without the `embedschemas` build tag), keeping the package
// importable as a library without the gitignored, build-time archive. Consumers
// must provide schemas explicitly via kubeconform.Options.SchemaDir.
func archiveBytes() ([]byte, error) {
	return nil, ErrNoEmbeddedArchive
}

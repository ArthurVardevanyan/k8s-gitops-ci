//go:build !embedschemas

package policies

// archiveBytes reports that no policy archive is compiled in. This variant is
// used by default (without the `embedschemas` build tag), keeping the package
// importable as a library without the gitignored, build-time archive. Consumers
// must provide policies explicitly via Options.PolicyPath.
func archiveBytes() ([]byte, error) {
	return nil, ErrNoEmbeddedArchive
}

package image

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/exempt"
)

// ValidationError records an unpinned image.
type ValidationError struct {
	File, Kind, Name, Image, Message string
	// Annotations holds the owning resource's metadata.annotations, so
	// callers that route findings through the shared check/exempt engine
	// (rather than this package's own ValidateBytesWithExemptions) can
	// evaluate annotation-based exemptions themselves. Only populated by
	// ValidateBytesRaw.
	Annotations map[string]string
	// Repo is the tag/digest-independent "registry/repo" key (Ref.RepoKey)
	// for Image, so a caller can offer a repo-level exemption match
	// alongside the exact Image value. Only populated by ValidateBytesRaw.
	Repo string
}

func (e ValidationError) String() string {
	if e.Kind != "" && e.Name != "" {
		return fmt.Sprintf("%s: %s %q image %q: %s", e.File, e.Kind, e.Name, e.Image, e.Message)
	}
	return fmt.Sprintf("%s: image %q: %s", e.File, e.Image, e.Message)
}

// DeduplicatedError aggregates image findings.
type DeduplicatedError struct {
	Kind, Name, Image string
	Count             int
}

func (d DeduplicatedError) String() string {
	if d.Kind != "" && d.Name != "" {
		return fmt.Sprintf("%s %q image %q not pinned to SHA digest (%d overlay(s))", d.Kind, d.Name, d.Image, d.Count)
	}
	return fmt.Sprintf("image %q not pinned to SHA digest (%d overlay(s))", d.Image, d.Count)
}

// Deduplicate groups image findings by Kind+Name+Image, returning one
// DeduplicatedError per unique combination with a count of how many
// occurrences (e.g. across overlays) matched.
func Deduplicate(errs []ValidationError) []DeduplicatedError {
	seen := make(map[string]*DeduplicatedError)
	order := make([]string, 0, len(errs))
	for _, e := range errs {
		key := fmt.Sprintf("%s/%s/%s", e.Kind, e.Name, e.Image)
		if d, ok := seen[key]; ok {
			d.Count++
			continue
		}
		seen[key] = &DeduplicatedError{Kind: e.Kind, Name: e.Name, Image: e.Image, Count: 1}
		order = append(order, key)
	}
	out := make([]DeduplicatedError, 0, len(order))
	for _, k := range order {
		out = append(out, *seen[k])
	}
	return out
}

// ExemptedImage records an annotation-exempted image.
type ExemptedImage struct {
	File, Kind, Name, Image string
}

func (e ExemptedImage) String() string {
	return fmt.Sprintf("%s: image %q exempt via %s annotation", e.File, e.Image, exempt.Key(exempt.IDImageChecksum))
}

// Ref parses an OCI image reference.
type Ref struct {
	Registry, Repo, Tag, Digest, Raw string

	// Bare is true when the raw reference carried no explicit registry
	// host (e.g. "nginx:latest" or "alpine"), so the registry had to be
	// defaulted. Bare references are today acceptable at resolve time only
	// if a resolve step supplies a registry; for pinning and FQDN
	// enforcement they are flagged.
	Bare bool
}

// ParseImageRef parses a raw image string.
func ParseImageRef(raw string) *Ref {
	if raw == "" {
		return nil
	}
	ref := &Ref{Raw: raw}
	atParts := strings.Split(raw, "@")
	if len(atParts) == 2 {
		ref.Digest = atParts[1]
		raw = atParts[0]
	}
	parts := strings.SplitN(raw, "/", 2)
	if len(parts) == 2 && (strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":") || parts[0] == "localhost") {
		// "localhost" is a valid explicit registry host per OCI/Docker
		// reference rules even though it contains neither "." nor ":" -
		// without this it would be misclassified as a bare shortname
		// (defaulted to docker.io), wrongly flagging e.g.
		// "localhost/myapp:tag" for image-fqdn.
		ref.Registry = parts[0]
		ref.Repo = parts[1]
	} else {
		ref.Bare = true
		ref.Registry = "docker.io"
		ref.Repo = raw
	}
	if ref.Repo != "" {
		lastColon := strings.LastIndex(ref.Repo, ":")
		lastSlash := strings.LastIndex(ref.Repo, "/")
		if lastColon != -1 && lastColon > lastSlash {
			ref.Tag = ref.Repo[lastColon+1:]
			ref.Repo = ref.Repo[:lastColon]
		}
	}
	return ref
}

// RepoKey returns a tag/digest-independent identifier for the image
// ("registry/repo"), suitable for a repo-level exemption that should match
// every tag or digest of that repo rather than one exact reference. Uses
// the registry as parsed (docker.io when defaulted for a bare shortname).
func (r *Ref) RepoKey() string {
	if r == nil {
		return ""
	}
	return r.Registry + "/" + r.Repo
}

// gcpComputeSelfLinkRe matches GCP Compute Engine resource self-links that
// legitimately appear under an `image:`-named key (e.g. a GCP disk image
// reference) but are not container-image references at all. They are bare
// (no registry host) and carry no tag, so without this guard they would be
// misclassified as shortname container images and flagged for FQDN/pinning.
var gcpComputeSelfLinkRe = regexp.MustCompile(`^projects/[^/]+/(?:global/images|zones/[^/]+/disks)/`)

// isImageRef reports whether ref is an OCI image reference that image
// enforcement (pinning + FQDN) should evaluate. Templated/placeholder
// values (containing spaces) and GCP Compute Engine self-links are excluded,
// but bare shortnames and explicit docker.io refs are enforced, so an
// unpinned docker.io image is no longer silently skipped.
func isImageRef(ref *Ref) bool {
	if ref == nil || strings.Contains(ref.Raw, " ") {
		return false
	}
	return !gcpComputeSelfLinkRe.MatchString(ref.Raw)
}

// isResolvableRef reports whether a ref can be live-resolved against its
// registry (used only by the offline VerifyFileTagDigests helper). Unlike
// pinning/FQDN enforcement this still requires an explicit non-Docker-Hub
// registry and a tag, because resolution talks to a registry.
func isResolvableRef(ref *Ref) bool {
	return isImageRef(ref) && !ref.Bare && ref.Registry != "docker.io" && ref.Tag != ""
}

// ValidateFile validates image pinning in a file.
func ValidateFile(path string) []ValidationError {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	errs, _ := ValidateBytesWithExemptions(data, path)
	return errs
}

// ValidateBytes validates image pinning in bytes, silently accepting annotation exemptions.
func ValidateBytes(data []byte, source string) []ValidationError {
	errs, _ := ValidateBytesWithExemptions(data, source)
	return errs
}

// ValidateFileWithExemptions validates image pinning, returning exempted images.
func ValidateFileWithExemptions(path string) ([]ValidationError, []ExemptedImage) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}
	return ValidateBytesWithExemptions(data, path)
}

// ValidateBytesWithExemptions validates image pinning and returns exempted images.
func ValidateBytesWithExemptions(data []byte, source string) ([]ValidationError, []ExemptedImage) {
	var errs []ValidationError
	var exempted []ExemptedImage
	dec := yaml.NewDecoder(newBytesReader(data))
	for {
		var doc yaml.Node
		if err := dec.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			break
		}
		if len(doc.Content) == 0 {
			continue
		}
		mapping := doc.Content[0]
		if mapping.Kind != yaml.MappingNode {
			continue
		}
		kind := quickString(findKey(mapping, "kind"))
		name := quickName(mapping)
		ann := extractAnnotations(mapping)
		for _, img := range extractImages(mapping, "") {
			ref := ParseImageRef(img)
			if !isImageRef(ref) {
				continue
			}
			if exempt.Accepts(ann, exempt.IDImageChecksum, img) {
				exempted = append(exempted, ExemptedImage{File: source, Kind: kind, Name: name, Image: img})
				continue
			}
			if ref.Digest == "" {
				errs = append(errs, ValidationError{
					File: source, Kind: kind, Name: name, Image: img,
					Message: "image is not pinned to a SHA digest",
				})
			}
		}
	}
	return errs, exempted
}

// imageCandidate is a single image reference extracted from a document,
// carrying the owning resource's annotation set so the shared exempt engine
// can evaluate annotation exemptions uniformly.
type imageCandidate struct {
	img  string
	ref  *Ref
	kind string
	name string
	ann  map[string]string
}

// collectImageCandidates decodes each mapping document in data and returns
// every parsed image reference it contains, discarding templated values and
// GCP Compute Engine self-links via isImageRef.
func collectImageCandidates(data []byte) []imageCandidate {
	var out []imageCandidate
	dec := yaml.NewDecoder(newBytesReader(data))
	for {
		var doc yaml.Node
		if err := dec.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			break
		}
		if len(doc.Content) == 0 {
			continue
		}
		mapping := doc.Content[0]
		if mapping.Kind != yaml.MappingNode {
			continue
		}
		kind := quickString(findKey(mapping, "kind"))
		name := quickName(mapping)
		ann := extractAnnotations(mapping)
		for _, img := range extractImages(mapping, "") {
			if ref := ParseImageRef(img); isImageRef(ref) {
				out = append(out, imageCandidate{img: img, ref: ref, kind: kind, name: name, ann: ann})
			}
		}
	}
	return out
}

// ValidateBytesRaw validates image pinning in bytes without applying any
// exemption filtering - every unpinned image is returned as a finding,
// including ones that would be annotation-exempted, each carrying the
// owning resource's annotations. Use this (instead of ValidateBytes /
// ValidateBytesWithExemptions) when the caller routes findings through the
// shared pkg/validator/check + pkg/validator/exempt engine, so that engine
// can apply exemptions (both annotation and EXEMPTIONS-selector modes)
// uniformly and record an audit-trail entry - matching how every other
// check in this repo is wired, rather than this package silently deciding
// exemptions on its own before the finding ever reaches the shared engine.
func ValidateBytesRaw(data []byte, source string) []ValidationError {
	var errs []ValidationError
	for _, c := range collectImageCandidates(data) {
		if c.ref.Digest == "" {
			errs = append(errs, ValidationError{
				File: source, Kind: c.kind, Name: c.name, Image: c.img,
				Message:     "image is not pinned to a SHA digest",
				Annotations: c.ann,
				Repo:        c.ref.RepoKey(),
			})
		}
	}
	return errs
}

// ValidateFQDNBytesRaw validates that every image reference uses an explicit
// registry host, flagging bare shortnames (e.g. "nginx:latest" or "alpine")
// that would otherwise silently default to a registry at resolve time. It
// mirrors ValidateBytesRaw: no exemption filtering here - the caller is
// expected to route findings through the shared check/exempt engine so
// exemption evaluation and audit-trail recording happen uniformly.
func ValidateFQDNBytesRaw(data []byte, source string) []ValidationError {
	var errs []ValidationError
	for _, c := range collectImageCandidates(data) {
		if c.ref.Bare {
			errs = append(errs, ValidationError{
				File: source, Kind: c.kind, Name: c.name, Image: c.img,
				Message:     "image uses a bare shortname without an explicit registry",
				Annotations: c.ann,
			})
		}
	}
	return errs
}

// ExtractImagesFromFile extracts all image values from a file.
func ExtractImagesFromFile(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return extractImagesFromBytes(data)
}

// VerifyTagDigest resolves a tag and compares it to a digest.
func VerifyTagDigest(ref *Ref, client *http.Client) *ValidationError {
	if ref == nil || client == nil || ref.Tag == "" {
		return &ValidationError{Message: fmt.Sprintf("failed to resolve tag %q: missing tag", ref.Raw)}
	}
	url := fmt.Sprintf("https://%s/v2/%s/manifests/%s", ref.Registry, ref.Repo, ref.Tag)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodHead, url, nil)
	if err != nil {
		return &ValidationError{Image: ref.Raw, Message: fmt.Sprintf("failed to resolve tag %q: %v", ref.Tag, err)}
	}
	resp, err := client.Do(req)
	if err != nil {
		return &ValidationError{Image: ref.Raw, Message: fmt.Sprintf("registry request failed: %v", err)}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return &ValidationError{Image: ref.Raw, Message: fmt.Sprintf("registry returned %d", resp.StatusCode)}
	}
	digest := resp.Header.Get("Docker-Content-Digest")
	if digest == "" {
		return &ValidationError{Image: ref.Raw, Message: "registry did not return Docker-Content-Digest header"}
	}
	if ref.Digest != "" && ref.Digest != digest {
		return &ValidationError{Image: ref.Raw, Message: fmt.Sprintf("tag %q resolves to %s but image specifies %s", ref.Tag, digest, ref.Digest)}
	}
	return nil
}

// VerifyFileTagDigests verifies all OCI images in a file.
func VerifyFileTagDigests(path string, client *http.Client) []ValidationError {
	imgs := ExtractImagesFromFile(path)
	var errs []ValidationError
	for _, img := range imgs {
		ref := ParseImageRef(img)
		if !isResolvableRef(ref) || ref.Digest == "" {
			continue
		}
		if err := VerifyTagDigest(ref, client); err != nil {
			errs = append(errs, *err)
		}
	}
	return errs
}

// DefaultClient returns an HTTP client suitable for OCI registry calls.
func DefaultClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}
}

// AuthenticatedClient returns a client with bearer token for 401 challenges.
func AuthenticatedClient(client *http.Client, registry, repo string) *http.Client {
	if client == nil {
		client = DefaultClient()
	}
	if registry == "docker.io" {
		return client
	}
	return client
}

var imageRe = regexp.MustCompile(`(?i)image:\s*["']?([^\s"']+)["']?`)

func extractImagesFromBytes(data []byte) []string {
	var imgs []string
	for _, line := range strings.Split(string(data), "\n") {
		m := imageRe.FindStringSubmatch(line)
		if len(m) >= 2 {
			imgs = append(imgs, m[1])
		}
	}
	return dedupStrings(imgs)
}

func extractImages(node *yaml.Node, parentKey string) []string {
	if node == nil {
		return nil
	}
	var imgs []string
	switch node.Kind {
	case yaml.ScalarNode:
		if parentKey == "image" {
			imgs = append(imgs, node.Value)
		}
	case yaml.MappingNode:
		// operator split-version pairing: an `image:` key and a sibling
		// `version: sha256:...` key *within this same mapping* combine
		// into one pinned image reference. The version sibling must be
		// looked up within the same mapping as the image key it pins,
		// not "whichever image was found most recently during the
		// recursive walk" - otherwise a digest can misattach to an
		// unrelated image found earlier elsewhere in the document.
		var imageVal, versionDigest string
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i].Value
			child := node.Content[i+1]
			switch {
			case key == "image" && child.Kind == yaml.ScalarNode:
				imageVal = child.Value
			case key == "version" && child.Kind == yaml.ScalarNode && strings.HasPrefix(child.Value, "sha256:"):
				versionDigest = child.Value
			case key == "image" && child.Kind == yaml.MappingNode:
				if ref := findKey(child, "reference"); ref != nil && ref.Kind == yaml.ScalarNode {
					imgs = append(imgs, ref.Value)
				}
			}
		}
		if imageVal != "" && versionDigest != "" && !strings.Contains(imageVal, "@") {
			imgs = append(imgs, imageVal+"@"+versionDigest)
		}
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i].Value
			child := node.Content[i+1]
			if key == "image" && versionDigest != "" {
				continue // already emitted combined above
			}
			imgs = append(imgs, extractImages(child, key)...)
		}
	case yaml.SequenceNode:
		for _, c := range node.Content {
			imgs = append(imgs, extractImages(c, parentKey)...)
		}
	case yaml.DocumentNode, yaml.AliasNode:
		// Not expected in a decoded resource body; nothing to extract.
	}
	return dedupStrings(imgs)
}

func extractAnnotations(mapping *yaml.Node) map[string]string {
	ann := make(map[string]string)
	if meta := findKey(mapping, "metadata"); meta != nil && meta.Kind == yaml.MappingNode {
		obj := findKey(meta, "annotations")
		if obj == nil || obj.Kind != yaml.MappingNode {
			return ann
		}
		for i := 0; i < len(obj.Content); i += 2 {
			ann[obj.Content[i].Value] = obj.Content[i+1].Value
		}
	}
	return ann
}

func quickName(mapping *yaml.Node) string {
	if meta := findKey(mapping, "metadata"); meta != nil && meta.Kind == yaml.MappingNode {
		if n := quickString(findKey(meta, "name")); n != "" {
			return n
		}
	}
	return ""
}

func quickString(n *yaml.Node) string {
	if n == nil {
		return ""
	}
	return n.Value
}

func findKey(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func dedupStrings(sl []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, s := range sl {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func newBytesReader(b []byte) *strings.Reader {
	return strings.NewReader(string(b))
}

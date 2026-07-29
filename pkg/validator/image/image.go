package image

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/exempt"
	"gopkg.in/yaml.v3"
)

// ValidationError records an unpinned image.
type ValidationError struct {
	File, Kind, Name, Image, Message string
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

// ExemptedImage records an annotation-exempted image.
type ExemptedImage struct {
	File, Image string
}

func (e ExemptedImage) String() string {
	return fmt.Sprintf("%s: image %q exempt via %s annotation", e.File, e.Image, exempt.Key(exempt.IDImageChecksum))
}

// ImageRef parses an OCI image reference.
type ImageRef struct {
	Registry, Repo, Tag, Digest, Raw string
}

// ParseImageRef parses a raw image string.
func ParseImageRef(raw string) *ImageRef {
	if raw == "" {
		return nil
	}
	ref := &ImageRef{Raw: raw}
	atParts := strings.Split(raw, "@")
	if len(atParts) == 2 {
		ref.Digest = atParts[1]
		raw = atParts[0]
	}
	parts := strings.SplitN(raw, "/", 2)
	if len(parts) == 2 && (strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":")) {
		ref.Registry = parts[0]
		ref.Repo = parts[1]
	} else {
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
	if ref.Tag == "" && ref.Digest == "" {
		ref.Tag = "latest"
	}
	return ref
}

// isOCIImageRef returns true for non-Docker-Hub refs.
func isOCIImageRef(ref *ImageRef) bool {
	return ref != nil && ref.Registry != "docker.io" && !strings.Contains(ref.Raw, " ")
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
			if err == io.EOF {
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
			if !isOCIImageRef(ref) {
				continue
			}
			if exempt.Accepts(ann, exempt.IDImageChecksum, img) {
				exempted = append(exempted, ExemptedImage{File: source, Image: img})
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

// ExtractImagesFromFile extracts all image values from a file.
func ExtractImagesFromFile(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return extractImagesFromBytes(data)
}

// VerifyTagDigest resolves a tag and compares it to a digest.
func VerifyTagDigest(ref *ImageRef, client *http.Client) *ValidationError {
	if ref == nil || client == nil || ref.Tag == "" {
		return &ValidationError{Message: fmt.Sprintf("failed to resolve tag %q: missing tag", ref.Raw)}
	}
	url := fmt.Sprintf("https://%s/v2/%s/manifests/%s", ref.Registry, ref.Repo, ref.Tag)
	req, err := http.NewRequest(http.MethodHead, url, nil)
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
		if !isOCIImageRef(ref) || ref.Digest == "" {
			continue
		}
		if err := VerifyTagDigest(ref, client); err != nil {
			errs = append(errs, *err)
		}
	}
	return errs
}

// DefaultClient returns an insecure-skip-verify HTTP client for tests.
func DefaultClient() *http.Client {
	return &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
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

// TokenResponse models an OCI token response.
type TokenResponse struct {
	Token       string `json:"token"`
	AccessToken string `json:"access_token"`
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
		// operator split-version pairing
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i].Value
			child := node.Content[i+1]
			if key == "version" && child.Kind == yaml.ScalarNode && strings.HasPrefix(child.Value, "sha256:") && len(imgs) > 0 {
				last := imgs[len(imgs)-1]
				if !strings.Contains(last, "@") {
					imgs[len(imgs)-1] = last + "@" + child.Value
				}
			}
			imgs = append(imgs, extractImages(child, key)...)
		}
	case yaml.SequenceNode:
		for _, c := range node.Content {
			imgs = append(imgs, extractImages(c, parentKey)...)
		}
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

func parseAuthHeader(header string) (realm, service, scope string) {
	// simplistic parsing
	parts := strings.Split(header, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(p, "Bearer ") {
			p = strings.TrimPrefix(p, "Bearer ")
		}
		if strings.HasPrefix(p, "realm=") {
			realm = strings.Trim(strings.TrimPrefix(p, "realm="), `"`)
		}
		if strings.HasPrefix(p, "service=") {
			service = strings.Trim(strings.TrimPrefix(p, "service="), `"`)
		}
		if strings.HasPrefix(p, "scope=") {
			scope = strings.Trim(strings.TrimPrefix(p, "scope="), `"`)
		}
	}
	return
}

func fetchToken(realm, service, scope string, client *http.Client) (string, error) {
	url := fmt.Sprintf("%s?service=%s&scope=%s", realm, service, scope)
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var tr TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", err
	}
	if tr.Token != "" {
		return tr.Token, nil
	}
	return tr.AccessToken, nil
}

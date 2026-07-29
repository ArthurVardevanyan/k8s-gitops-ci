package image

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseImageRef(t *testing.T) {
	cases := []struct {
		raw                               string
		registry, repo, tag, digest, full string
	}{
		{"registry.io/ns/repo:tag@sha256:abc", "registry.io", "ns/repo", "tag", "sha256:abc", ""},
		{"repo:latest", "docker.io", "repo", "latest", "", ""},
	}
	for _, c := range cases {
		ref := ParseImageRef(c.raw)
		if ref == nil {
			t.Fatalf("ParseImageRef(%q) = nil", c.raw)
		}
		if ref.Registry != c.registry || ref.Repo != c.repo || ref.Tag != c.tag || ref.Digest != c.digest {
			t.Errorf("ParseImageRef(%q) = {%q %q %q %q}", c.raw, ref.Registry, ref.Repo, ref.Tag, ref.Digest)
		}
	}
}

func TestIsOCIImageRef(t *testing.T) {
	if isOCIImageRef(ParseImageRef("alpine")) {
		t.Error("docker shortname is not OCI")
	}
	if !isOCIImageRef(ParseImageRef("registry.io/alpine")) {
		t.Error("registry ref is OCI")
	}
}

func TestValidateBytes_NoPinning(t *testing.T) {
	data := []byte(`kind: Deployment
metadata:
  name: d
spec:
  template:
    spec:
      containers:
      - image: registry.io/repo:tag
`)
	errs := ValidateBytes(data, "x.yaml")
	if len(errs) != 1 || !strings.Contains(errs[0].Message, "SHA digest") {
		t.Errorf("expected unpinned error: %v", errs)
	}
}

func TestValidateBytes_Pinned(t *testing.T) {
	data := []byte(`kind: Deployment
metadata:
  name: d
spec:
  template:
    spec:
      containers:
      - image: registry.io/repo@sha256:abc
`)
	errs := ValidateBytes(data, "x.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors: %v", errs)
	}
}

func TestExtractImagesFromFile(t *testing.T) {
	data := []byte(`containers:
- image: registry.io/a:b
- image: registry.io/c:d
`)
	imgs := extractImagesFromBytes(data)
	if len(imgs) != 2 {
		t.Errorf("expected 2 images: %v", imgs)
	}
}

func TestVerifyTagDigest_Match(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Docker-Content-Digest", "sha256:abc")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	ref := &ImageRef{Registry: strings.TrimPrefix(ts.URL, "https://"), Repo: "repo", Tag: "latest"}
	client := ts.Client()
	if err := VerifyTagDigest(ref, client); err != nil {
		t.Errorf("expected match: %v", err)
	}
}

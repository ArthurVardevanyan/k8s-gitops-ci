package exempt

import "testing"

func TestExemptable(t *testing.T) {
	if !Exemptable(IDImageChecksum) {
		t.Error("image-checksum should be exemptable")
	}
	if Exemptable(IDClusterIdentity) {
		t.Error("cluster-identity should never be exemptable")
	}
}

func TestRegisterExemptable(t *testing.T) {
	RegisterExemptable("custom")
	if !Exemptable("custom") {
		t.Error("custom should be exemptable after registration")
	}
	RegisterExemptable(IDClusterIdentity)
	if Exemptable(IDClusterIdentity) {
		t.Error("register should ignore cluster-identity")
	}
}

func TestKey(t *testing.T) {
	if got := Key(IDImageChecksum); got != AnnotationPrefix+"exempt-image-checksum" {
		t.Errorf("Key = %q", got)
	}
}

func TestAccepts(t *testing.T) {
	ann := map[string]string{Key(IDImageChecksum): "reg/repo:tag@sha256:abc"}
	if !Accepts(ann, IDImageChecksum, "reg/repo:tag@sha256:abc") {
		t.Error("expected match")
	}
	if Accepts(ann, IDImageChecksum, "other") {
		t.Error("expected no match")
	}
}

func TestSelectorMatches(t *testing.T) {
	sel := Selector{Check: IDImageChecksum, Value: "img@sha256:abc"}
	s := Scalar{Value: "img@sha256:abc", File: "deploy.yaml"}
	if !SelectorMatches(sel, s, IDImageChecksum) {
		t.Error("expected match")
	}
}

func TestEvaluate(t *testing.T) {
	s := Scalar{Value: "img@sha256:abc", File: "deploy.yaml", Path: "/spec/template/spec/containers/0/image"}
	ann := map[string]string{}
	selectors := []Selector{{Check: IDImageChecksum, Value: "img@sha256:abc"}}
	ok, applied := Evaluate(IDImageChecksum, s, ann, selectors)
	if !ok || applied.Value != "img@sha256:abc" {
		t.Errorf("expected exemption: %v %+v", ok, applied)
	}
}

func TestPathMatches(t *testing.T) {
	cases := []struct {
		sel, find string
		want      bool
	}{
		{"/spec/template/spec/containers/0/image", "spec/template/spec/containers/0/image", true},
		{"containers[].image", "spec/template/spec/containers/0/image", true},
		{"containers[].image", "spec/containers/5/image", true},
		{"spec.replicas", "spec/replicas", true},
		{"spec.replicas", "metadata/labels", false},
	}
	for _, c := range cases {
		if got := pathMatches(c.sel, c.find); got != c.want {
			t.Errorf("pathMatches(%q,%q) = %v want %v", c.sel, c.find, got, c.want)
		}
	}
}

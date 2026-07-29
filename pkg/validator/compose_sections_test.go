package validator

import (
	"errors"
	"testing"

	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
)

func TestComposePRChecksSection(t *testing.T) {
	s := ComposePRChecksSection(errors.New("title"), nil, nil)
	if !s.Error {
		t.Errorf("expected error section")
	}
}

func TestComposeLintingSection(t *testing.T) {
	s := ComposeLintingSection(map[string]string{"golangci": "issues"})
	if !s.Error {
		t.Errorf("expected error section")
	}
}

func TestComposeStaticChecksSection(t *testing.T) {
	s := ComposeStaticChecksSection(map[string]string{})
	if s.Error {
		t.Errorf("expected no error section")
	}
}

func TestComposeResourceComplianceSection(t *testing.T) {
	s := ComposeResourceComplianceSection([]check.Finding{{CheckID: "x", Message: "m"}})
	if !s.Error {
		t.Errorf("expected error section")
	}
}

func TestComposeKyvernoSection(t *testing.T) {
	s := ComposeKyvernoSection("")
	if s.Error {
		t.Errorf("expected no error section")
	}
}

func TestRenderSubDropdown(t *testing.T) {
	out := RenderSubDropdown("Title", "Body")
	if out == "" {
		t.Errorf("expected dropdown")
	}
}

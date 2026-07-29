package validator

import "testing"

func TestQuickKind(t *testing.T) {
	if got := quickKind([]byte("apiVersion: v1\nkind: Pod\n")); got != "Pod" {
		t.Errorf("quickKind = %q", got)
	}
}

func TestQuickAPIVersion(t *testing.T) {
	if got := quickAPIVersion([]byte("apiVersion: apps/v1\nkind: Deployment\n")); got != "apps/v1" {
		t.Errorf("quickAPIVersion = %q", got)
	}
}

func TestIsKyvernoPolicyDoc(t *testing.T) {
	data := []byte("apiVersion: kyverno.io/v1\nkind: ClusterPolicy\n")
	if !isKyvernoPolicyDoc(data) {
		t.Errorf("expected kyverno policy")
	}
}

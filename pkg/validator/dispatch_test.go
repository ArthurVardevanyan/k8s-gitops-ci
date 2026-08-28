package validator

import "testing"

func TestQuickKind(t *testing.T) {
	t.Parallel()
	// The raw header text is not the scalar value. A quoted or
	// comment-suffixed kind used to be returned verbatim, matching no
	// registered kind and silently skipping every check for the document -
	// invisible, because runtime checks are non-exemptable.
	tests := []struct {
		name, in, want string
	}{
		{"plain", "apiVersion: v1\nkind: Pod\n", "Pod"},
		{"double quoted", "apiVersion: v1\nkind: \"Pod\"\n", "Pod"},
		{"single quoted", "apiVersion: v1\nkind: 'Pod'\n", "Pod"},
		{"trailing comment", "apiVersion: v1\nkind: Pod # the workload\n", "Pod"},
		{"quoted with hash", "apiVersion: v1\nkind: \"Pod # not a comment\"\n", "Pod # not a comment"},
		{"double quoted then comment", "apiVersion: v1\nkind: \"Pod\" # workload\n", "Pod"},
		{"single quoted then comment", "apiVersion: v1\nkind: 'Pod'  # workload\n", "Pod"},
		{"hash without leading space is not a comment", "apiVersion: v1\nkind: Pod#1\n", "Pod#1"},
		{"extra whitespace", "apiVersion: v1\nkind:    Pod   \n", "Pod"},
		{"absent", "apiVersion: v1\n", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := quickKind([]byte(tt.in)); got != tt.want {
				t.Errorf("quickKind = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestQuickAPIVersion(t *testing.T) {
	t.Parallel()
	// Same normalization applies here: a quoted apiVersion would otherwise
	// defeat the Kyverno group regex.
	tests := []struct {
		name, in, want string
	}{
		{"plain", "apiVersion: apps/v1\nkind: Deployment\n", "apps/v1"},
		{"double quoted", "apiVersion: \"apps/v1\"\nkind: Deployment\n", "apps/v1"},
		{"single quoted", "apiVersion: 'kyverno.io/v1'\nkind: Policy\n", "kyverno.io/v1"},
		{"trailing comment", "apiVersion: apps/v1 # pinned\nkind: Deployment\n", "apps/v1"},
		{"quoted then comment", "apiVersion: \"apps/v1\" # pinned\nkind: Deployment\n", "apps/v1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := quickAPIVersion([]byte(tt.in)); got != tt.want {
				t.Errorf("quickAPIVersion = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsKyvernoPolicyDoc(t *testing.T) {
	t.Parallel()
	data := []byte("apiVersion: kyverno.io/v1\nkind: ClusterPolicy\n")
	if !isKyvernoPolicyDoc(data) {
		t.Errorf("expected kyverno policy")
	}
}

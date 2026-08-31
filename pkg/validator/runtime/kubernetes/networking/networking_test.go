package networking

import (
	"strings"
	"testing"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

// The networking rules had no behavioral coverage at all: the only test
// touching them proved one rule ID was registered. That is enough to keep a
// check in the report and nothing like enough to know it validates anything,
// which is how network-policy/port-range-invalid came to accept an endPort
// paired with a named port - the branch upstream rejects outright.

type netCase struct {
	name     string
	doc      string
	want     int
	contains string
}

func runNetCases(t *testing.T, run func([]byte, string) []runtime.Finding, cases []netCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := run([]byte(tc.doc), "test.yaml")
			if len(findings) != tc.want {
				t.Fatalf("got %d finding(s), want %d: %v", len(findings), tc.want, findings)
			}
			if tc.contains != "" && !strings.Contains(findings[0].Message, tc.contains) {
				t.Errorf("message %q does not contain %q", findings[0].Message, tc.contains)
			}
		})
	}
}

// netpol builds a NetworkPolicy with the given ingress ports block.
func netpol(ports string) string {
	return "apiVersion: networking.k8s.io/v1\nkind: NetworkPolicy\nmetadata:\n  name: np\n" +
		"spec:\n  podSelector: {}\n  ingress:\n    - ports:\n" + ports
}

func svc(spec string) string {
	return "apiVersion: v1\nkind: Service\nmetadata:\n  name: s\nspec:\n" + spec
}

func TestNetworkPolicyPortRangeInvalid(t *testing.T) {
	runNetCases(t, newPortRangeInvalidCheck().Run, []netCase{
		{name: "valid range", doc: netpol("        - port: 80\n          endPort: 90\n"), want: 0},
		{name: "endPort equal to port", doc: netpol("        - port: 80\n          endPort: 80\n"), want: 0},
		{name: "no endPort", doc: netpol("        - port: 80\n"), want: 0},
		{
			name: "endPort below port", doc: netpol("        - port: 90\n          endPort: 80\n"),
			want: 1, contains: "must be >= start",
		},
		{
			name: "endPort without port", doc: netpol("        - endPort: 80\n"),
			want: 1, contains: "port is not specified",
		},
		{
			// Upstream rejects this outright: a named port has no numeric
			// value to bound. Reading the name as port 0 made every
			// positive endPort look in range.
			name: "endPort with a named port", doc: netpol("        - port: http\n          endPort: 8080\n"),
			want: 1, contains: "non-numeric",
		},
	})
}

func TestNetworkPolicyProtocolInvalid(t *testing.T) {
	runNetCases(t, newProtocolInvalidCheck().Run, []netCase{
		{name: "TCP", doc: netpol("        - port: 80\n          protocol: TCP\n"), want: 0},
		{name: "UDP", doc: netpol("        - port: 80\n          protocol: UDP\n"), want: 0},
		{name: "SCTP", doc: netpol("        - port: 80\n          protocol: SCTP\n"), want: 0},
		{name: "omitted", doc: netpol("        - port: 80\n"), want: 0},
		{
			name: "lowercase is not the enum value", doc: netpol("        - port: 80\n          protocol: tcp\n"),
			want: 1, contains: "tcp",
		},
		{name: "unknown", doc: netpol("        - port: 80\n          protocol: QUIC\n"), want: 1},
	})
}

func TestNetworkPolicyPolicyTypeInvalid(t *testing.T) {
	base := "apiVersion: networking.k8s.io/v1\nkind: NetworkPolicy\nmetadata:\n  name: np\n" +
		"spec:\n  podSelector: {}\n  policyTypes:\n"
	runNetCases(t, newPolicyTypeInvalidCheck().Run, []netCase{
		{name: "Ingress", doc: base + "    - Ingress\n", want: 0},
		{name: "Egress", doc: base + "    - Egress\n", want: 0},
		{name: "both", doc: base + "    - Ingress\n    - Egress\n", want: 0},
		{name: "unknown", doc: base + "    - Sideways\n", want: 1, contains: "Sideways"},
		{name: "lowercase is not the enum value", doc: base + "    - ingress\n", want: 1},
	})
}

func TestServiceTypeInvalid(t *testing.T) {
	runNetCases(t, newTypeInvalidCheck().Run, []netCase{
		{name: "ClusterIP", doc: svc("  type: ClusterIP\n"), want: 0},
		{name: "NodePort", doc: svc("  type: NodePort\n"), want: 0},
		{name: "LoadBalancer", doc: svc("  type: LoadBalancer\n"), want: 0},
		{name: "ExternalName", doc: svc("  type: ExternalName\n  externalName: example.com\n"), want: 0},
		// Defaulting supplies ClusterIP, so an unrendered manifest omitting
		// the field is not an error.
		{name: "omitted", doc: svc("  ports:\n    - port: 80\n"), want: 0},
		{name: "unknown", doc: svc("  type: Internal\n"), want: 1, contains: "Internal"},
		{name: "lowercase is not the enum value", doc: svc("  type: clusterip\n"), want: 1},
	})
}

func TestServiceSessionAffinityInvalid(t *testing.T) {
	runNetCases(t, newSessionAffinityInvalidCheck().Run, []netCase{
		{name: "ClientIP", doc: svc("  sessionAffinity: ClientIP\n"), want: 0},
		{name: "None", doc: svc("  sessionAffinity: None\n"), want: 0},
		{name: "omitted", doc: svc("  ports:\n    - port: 80\n"), want: 0},
		{name: "unknown", doc: svc("  sessionAffinity: Sticky\n"), want: 1, contains: "Sticky"},
		{name: "lowercase is not the enum value", doc: svc("  sessionAffinity: clientip\n"), want: 1},
	})
}

func TestIngressPathTypeInvalid(t *testing.T) {
	// pathType == "" omits the field entirely; pass `""` (a quoted empty
	// string) to emit an explicitly-empty value. The two are different cases
	// upstream and must not be conflated here.
	ing := func(pathType string) string {
		s := "apiVersion: networking.k8s.io/v1\nkind: Ingress\nmetadata:\n  name: i\n" +
			"spec:\n  rules:\n    - host: example.com\n      http:\n        paths:\n" +
			"          - path: /\n            backend:\n              service:\n" +
			"                name: s\n                port:\n                  number: 80\n"
		if pathType != "" {
			s += "            pathType: " + pathType + "\n"
		}
		return s
	}
	runNetCases(t, newPathTypeInvalidCheck().Run, []netCase{
		{name: "Exact", doc: ing("Exact"), want: 0},
		{name: "Prefix", doc: ing("Prefix"), want: 0},
		{name: "ImplementationSpecific", doc: ing("ImplementationSpecific"), want: 0},
		// Upstream reports a nil pathType as Required; that branch is
		// deliberately not ported, since an unrendered manifest omits it.
		{name: "omitted is not reported", doc: ing(""), want: 0},
		// But networking.k8s.io/v1 has no pathType defaulter, so an
		// explicit "" arrives as a non-nil pointer and upstream returns
		// NotSupported. Skipping it missed a real admission rejection.
		{name: "explicitly empty is reported", doc: ing(`""`), want: 1},
		{name: "unknown", doc: ing("Regex"), want: 1, contains: "Regex"},
		{name: "lowercase is not the enum value", doc: ing("prefix"), want: 1},
		{
			name: "a rule without an http block is skipped",
			doc: "apiVersion: networking.k8s.io/v1\nkind: Ingress\nmetadata:\n  name: i\n" +
				"spec:\n  rules:\n    - host: example.com\n",
			want: 0,
		},
	})
}

package k8scni

import (
	"sync"

	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
)

var registerOnce sync.Once

// Register registers every check in this package with the check registry.
//
// Registration funnels through runtime.RegisterAll so that each check must
// carry an UpstreamRef in upstreamRefs citing the exact upstream function it
// ports or imports; RegisterAll panics on a check with no valid citation.
func Register() {
	registerOnce.Do(register)
}

func register() {
	checks := []runtime.Check{
		newConfigInvalidCheck(),
		newOVNNetConfInvalidCheck(),
	}
	runtime.RegisterAll(checks, upstreamRefs)
}

// init registers this package's checks. The package is blank-imported by
// pkg/validator/register_checks.go purely for this side effect - k8scni is a
// sibling of runtime/kubernetes (a different upstream family entirely: the
// k8s.cni.cncf.io CRD and, for the OVN tier, ovn-kubernetes - not
// kubernetes/kubernetes), so it is wired in alongside that import rather
// than nested inside it.
func init() {
	Register()
}

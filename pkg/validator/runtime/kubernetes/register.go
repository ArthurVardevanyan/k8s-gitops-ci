package kubernetes

import (
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/check"
	runtime "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime"
	_ "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime/kubernetes/admissionregistration/validation" // registers admissionregistration checks
	_ "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime/kubernetes/apps/validation"                  // registers apps checks
	_ "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime/kubernetes/batch/validation"                 // registers batch checks
	"github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime/kubernetes/core/validation"
	_ "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime/kubernetes/networking/validation" // registers networking checks
	_ "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime/kubernetes/policy/validation"     // registers policy checks
	_ "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime/kubernetes/rbac/validation"       // registers rbac checks
	_ "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime/kubernetes/storage/validation"    // registers storage checks
)

// Register registers all Kubernetes validation checks with the check registry.
// This function is called via init() in each subpackage.
func Register(c runtime.Check) {
	check.Register(runtime.CheckToRegistered(c))
}

// init registers all core validation checks.
func init() {
	validation.Register()
}

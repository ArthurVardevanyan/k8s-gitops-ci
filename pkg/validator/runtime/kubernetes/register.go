package kubernetes

import (
	_ "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime/kubernetes/admissionregistration" // registers admissionregistration checks
	_ "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime/kubernetes/apiextensions"         // registers apiextensions checks
	_ "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime/kubernetes/apps"                  // registers apps checks
	_ "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime/kubernetes/autoscaling"           // registers autoscaling checks
	_ "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime/kubernetes/batch"                 // registers batch checks
	_ "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime/kubernetes/core"                  // registers core checks
	_ "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime/kubernetes/networking"            // registers networking checks
	_ "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime/kubernetes/policy"                // registers policy checks
	_ "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime/kubernetes/rbac"                  // registers rbac checks
	_ "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/validator/runtime/kubernetes/storage"               // registers storage checks
)

// This package exists only to blank-import every check sub-package, so
// that importing it registers the whole family.
//
// It deliberately exports no Register helper. It previously offered
// Register(runtime.Check), which called check.Register directly and so
// bypassed runtime.RegisterAll - the registrar that panics when a check has
// no valid UpstreamRef. Any caller using it could install an uncited check
// while the package documented that this was impossible. It had no callers.

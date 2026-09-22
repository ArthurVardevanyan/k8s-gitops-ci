// Package all registers every built-in forge in the global registry via
// init().  This allows the CLI binary (cmd/k8s-gitops-ci) to wire all
// available forges with a single blank import — adding a new forge is a
// one-line blank import in this file and a new pkg/forge/<name> package.
package all

import (
	_ "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/forge/github" // registers github forge via init()
	_ "github.com/ArthurVardevanyan/k8s-gitops-ci/pkg/forge/gitlab" // registers gitlab forge via init()
)

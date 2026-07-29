# k8s-gitops-ci

Generic GitOps CI engine for Kubernetes manifests.

This is the org-agnostic core. Organization-specific configuration is injected
through the `provider` seams and exported package override variables.

## Build

```sh
task build
```

## Test

```sh
task test
```

## Lint

```sh
task lint
```

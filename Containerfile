# ── Stage 1: hermetic engine build ────────────────────────────────────────
# Cross-compiles the Go binary inside the image (CGO off → pure static
# binary) so the image is reproducible, hermetic, and truly multi-arch:
# the build stage always uses a native amd64 golang base and cross-compiles
# for GOARCH, avoiding qemu for the expensive go-build phase. Only stage 2's
# RUN steps (microdnf + npm + curl) execute under emulation when the target
# platform differs from the host.
FROM golang:1.27@sha256:7bffdb405cd12940d2980daa49a86ef575ed4525a17ee7d0c9562547357ab46a AS builder

# Version metadata — filled at build time from the pipeline
ARG BUILD_VERSION=local
ARG BUILD_COMMIT=unknown
ARG BUILD_TIME="1970-01-01T00:00:00Z"

WORKDIR /src

# Copy module files first for Docker cache optimisation:
# this layer is reused whenever go.mod/go.sum are unchanged (common).
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Run the schema/policy pull scripts (they download from the upstream
# repo at a pinned SHA and write schemas.tar.gz / policies.tar.gz into
# pkg/ — the same artefacts that `task schemas:pull` / `policies:pull`
# produce in the Taskfile).  The scripts only need git, curl and mktemp,
# all available in the golang base.
RUN ./scripts/pull-schemas.sh && \
    ./scripts/pull-policies.sh

# Cross-compile for the target GOARCH (amd64 is native, arm64 is cross).
# CGO_ENABLED=0 ensures a fully static binary; -tags=embedschemas embeds
# the schema/policy archives so the binary works standalone.
ARG GOARCH=amd64
RUN CGO_ENABLED=0 GOOS=linux GOARCH="${GOARCH}" \
    go build -ldflags \
    "-s -w \
    -X github.com/ArthurVardevanyan/k8s-gitops-ci/cmd/version.BuildVersion=${BUILD_VERSION} \
    -X github.com/ArthurVardevanyan/k8s-gitops-ci/cmd/version.BuildTime=${BUILD_TIME} \
    -X github.com/ArthurVardevanyan/k8s-gitops-ci/cmd/version.Commit=${BUILD_COMMIT}" \
    -tags=embedschemas \
    -trimpath \
    -o /out/k8s-gitops-ci ./cmd/k8s-gitops-ci

# ── Stage 2: minimal runtime with all vendored tools ──────────────────────
# Runtime stage: UBI-minimal base, installs OS packages, per-arch CLI
# binaries, Node.js linter tooling, and copies the cross-compiled binary
# from the builder stage.  The per-arch download logic (`uname -m`) makes
# this stage work with `podman build --platform linux/amd64,linux/arm64`.
# renovate: datasource=docker depName=registry.access.redhat.com/ubi10/ubi-minimal versioning=docker
# ── Stage 2: minimal runtime with all vendored tools ──────────────────────
# Runtime stage: UBI-minimal base, installs OS packages, per-arch CLI
# binaries, Node.js linter tooling, and copies the cross-compiled binary
# from the builder stage.  The per-arch download logic (`uname -m`) makes
# this stage work with `podman build --platform linux/amd64,linux/arm64`.
FROM registry.access.redhat.com/ubi10/ubi-minimal:10.2-1789645153@sha256:04febb4a74cc9ef3eca05ef851d92957276cc6e82fe8cb1ee44abf5114d440d8 AS runtime

# ── Version pins for vendored CLI tools ───────────────────────────────────
# Downstream consumers can override these at build time (e.g. `--build-arg
# GH_VERSION=2.100.0`); the defaults here are stable and versioned.
# renovate: datasource=github-releases depName=cli/cli versioning=semver
ARG GH_VERSION=2.101.0
# renovate: datasource=gitlab-releases depName=gitlab-org/cli versioning=semver
ARG GLAB_VERSION=1.118.0
# renovate: datasource=github-releases depName=kubernetes-sigs/kustomize extractVersion=/(.*)$/ versioning=semver
ARG KUSTOMIZE_VERSION=5.8.1
# renovate: datasource=github-releases depName=koalaman/shellcheck versioning=semver
ARG SHELLCHECK_VERSION=0.11.0
# renovate: datasource=npm depName=prettier versioning=semver
ARG PRETTIER_VERSION=3.5.3
# renovate: datasource=npm depName=markdownlint-cli versioning=semver
ARG MARKDOWNLINT_CLI_VERSION=0.44.0

# ── OS packages (available in UBI repos) ──────────────────────────────────
RUN microdnf install -y --nodocs --setopt=install_weak_deps=0 \
    --setopt=tsflags=nodocs \
    bash \
    ca-certificates \
    curl-minimal \
    git \
    gzip \
    nodejs \
    npm \
    tar \
    xz \
 && microdnf clean all \
 && rm -rf /var/cache/*

# ── Vendored CLI binaries (per-arch downloads; uname -m makes this work
#     under `podman build --platform linux/amd64,linux/arm64`): ───────────
RUN set -euxo pipefail; \
    ARCH="$(uname -m)"; \
    case "$ARCH" in \
      x86_64)  GOARCH=amd64; SCARCH=linux.x86_64 ;; \
      aarch64) GOARCH=arm64; SCARCH=linux.aarch64 ;; \
      *)       echo "unsupported arch: $ARCH" >&2; exit 1 ;; \
    esac; \
    # github-cli \
    curl -fsSL "https://github.com/cli/cli/releases/download/v${GH_VERSION}/gh_${GH_VERSION}_linux_${GOARCH}.tar.gz" \
      | tar xz --no-same-owner --strip-components=2 -C /usr/local/bin "gh_${GH_VERSION}_linux_${GOARCH}/bin/gh"; \
    # glab \
    curl -fsSL "https://gitlab.com/gitlab-org/cli/-/releases/v${GLAB_VERSION}/downloads/glab_${GLAB_VERSION}_linux_${GOARCH}.tar.gz" \
      | tar xz --no-same-owner --strip-components=1 -C /usr/local/bin "bin/glab"; \
    # kustomize \
    curl -fsSL "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv${KUSTOMIZE_VERSION}/kustomize_v${KUSTOMIZE_VERSION}_linux_${GOARCH}.tar.gz" \
      | tar xz --no-same-owner -C /usr/local/bin kustomize; \
    # shellcheck \
    curl -fsSL "https://github.com/koalaman/shellcheck/releases/download/v${SHELLCHECK_VERSION}/shellcheck-v${SHELLCHECK_VERSION}.${SCARCH}.tar.xz" \
      | tar xJ --no-same-owner --strip-components=1 -C /usr/local/bin "shellcheck-v${SHELLCHECK_VERSION}/shellcheck"; \
    chmod 0755 /usr/local/bin/gh /usr/local/bin/glab /usr/local/bin/kustomize /usr/local/bin/shellcheck

# ── Node.js-based lint tools (global npm packages) ────────────────────────
RUN npm install -g "prettier@${PRETTIER_VERSION}" "markdownlint-cli@${MARKDOWNLINT_CLI_VERSION}" \
 && npm cache clean --force

# ── Copy the cross-compiled binary from the builder stage ─────────────────
COPY --from=builder --chmod=0755 /out/k8s-gitops-ci /usr/local/bin/k8s-gitops-ci

ENTRYPOINT ["/usr/local/bin/k8s-gitops-ci"]
CMD ["--help"]

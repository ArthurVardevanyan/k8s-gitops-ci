FROM registry.access.redhat.com/ubi10/ubi-minimal:10.2-1790075626@sha256:e3a5632d7ae8a97e06f634522d06187f12793e90ac0d7b51bc671c83a96d8eda

# CLI tools installed as standalone binaries — versions pinned via ARGs
# so downstream consumers can override at build time.
ARG GH_VERSION=2.101.0
ARG GLAB_VERSION=1.118.0
ARG KUSTOMIZE_VERSION=5.8.1
ARG SHELLCHECK_VERSION=0.11.0
ARG PRETTIER_VERSION=3.5.3
ARG MARKDOWNLINT_CLI_VERSION=0.44.0

# OS packages available in UBI repos
RUN microdnf install -y --nodocs --setopt=install_weak_deps=0 \
    bash \
    ca-certificates \
    curl-minimal \
    git \
    gzip \
    nodejs \
    npm \
    tar \
    xz \
 && microdnf clean all

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

# Node.js-based lint tools
RUN npm install -g "prettier@${PRETTIER_VERSION}" "markdownlint-cli@${MARKDOWNLINT_CLI_VERSION}" \
 && npm cache clean --force

COPY --chmod=0755 bin/k8s-gitops-ci /usr/local/bin/k8s-gitops-ci

ENTRYPOINT ["/usr/local/bin/k8s-gitops-ci"]
CMD ["--help"]

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

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

# CLI tools installed as standalone binaries — versions pinned via ARGs
# so downstream consumers can override at build time.
ARG GH_VERSION=2.101.0
ARG GLAB_VERSION=1.117.0
ARG KUSTOMIZE_VERSION=5.8.1
ARG SHELLCHECK_VERSION=0.11.0

RUN set -eux; \
    ARCH="$(uname -m)"; \
    case "$ARCH" in \
      x86_64)  GOARCH=amd64; GLABARCH=Linux_x86_64; SCARCH=linux.x86_64 ;; \
      aarch64) GOARCH=arm64;  GLABARCH=Linux_arm64;  SCARCH=linux.aarch64 ;; \
      *)       echo "unsupported arch: $ARCH" >&2; exit 1 ;; \
    esac; \
    # github-cli \
    curl -fsSL "https://github.com/cli/cli/releases/download/v${GH_VERSION}/gh_${GH_VERSION}_linux_${GOARCH}.tar.gz" \
      | tar xz --strip-components=2 -C /usr/local/bin "gh_${GH_VERSION}_linux_${GOARCH}/bin/gh"; \
    # glab \
    curl -fsSL "https://gitlab.com/gitlab-org/cli/-/releases/v${GLAB_VERSION}/downloads/glab_${GLAB_VERSION}_${GLABARCH}.tar.gz" \
      | tar xz --strip-components=1 -C /usr/local/bin "bin/glab"; \
    # kustomize \
    curl -fsSL "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv${KUSTOMIZE_VERSION}/kustomize_v${KUSTOMIZE_VERSION}_linux_${GOARCH}.tar.gz" \
      | tar xz -C /usr/local/bin kustomize; \
    # shellcheck \
    curl -fsSL "https://github.com/koalaman/shellcheck/releases/download/v${SHELLCHECK_VERSION}/shellcheck-v${SHELLCHECK_VERSION}.${SCARCH}.tar.xz" \
      | tar xJ --strip-components=1 -C /usr/local/bin "shellcheck-v${SHELLCHECK_VERSION}/shellcheck"

# Node.js-based lint tools
RUN npm install -g prettier markdownlint-cli \
 && npm cache clean --force

COPY bin/k8s-gitops-ci /usr/local/bin/k8s-gitops-ci

ENTRYPOINT ["/usr/local/bin/k8s-gitops-ci"]
CMD ["--help"]

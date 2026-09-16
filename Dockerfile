FROM alpine:3.21

RUN apk add --no-cache \
    bash \
    ca-certificates \
    git \
    glab \
    kustomize \
    nodejs \
    npm \
    shellcheck \
 && npm install -g prettier markdownlint-cli \
 && npm cache clean --force

COPY bin/k8s-gitops-ci /usr/local/bin/k8s-gitops-ci

ENTRYPOINT ["/usr/local/bin/k8s-gitops-ci"]
CMD ["--help"]

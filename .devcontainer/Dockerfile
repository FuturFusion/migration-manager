ARG GO_VERSION=1.24
ARG DEBIAN_VERSION=bookworm

# Build DEB files for libnbd

FROM debian:${DEBIAN_VERSION} AS build
ARG LIBNBD_VERSION=1.22.5
ARG LIBNBD_DEB_VERSION=1.22.5-1

# Install requirements for deb build of libnbd.
RUN apt update && \
    apt install -y devscripts debhelper dh-python python3-dev libfuse3-dev libgnutls28-dev libxml2-dev pkgconf gnutls-bin nbdkit nbdkit-plugin-dev qemu-utils qemu-system-common ocaml-nox ocamlbuild ocaml-findlib dh-ocaml bash-completion jq iproute2

# Build libnbd deb packages.
RUN mkdir -p /tmp/debs && \
    cd /tmp/debs && \
    dget -u https://deb.debian.org/debian/pool/main/libn/libnbd/libnbd_${LIBNBD_DEB_VERSION}.dsc && \
    cd libnbd-${LIBNBD_VERSION} && \
    debuild -b -uc -us

# Go development container

FROM golang:${GO_VERSION}-${DEBIAN_VERSION}
ARG USERNAME=vscode
ARG USER_UID=1000
ARG USER_GID=1000

# Install necessary tools.
RUN echo 'deb [trusted=yes] https://repo.goreleaser.com/apt/ /' | tee /etc/apt/sources.list.d/goreleaser.list && \
    apt update && \
    apt install -y \
      aspell \
      aspell-en \
      fish \
      goreleaser \
      jq \
      less \
      libfuse3-3 \
      libxml2 \
      locales \
      locales-all \
      man \
      ocaml-4.13.1 \
      python3.11-venv \
      ruby-mdl \
      shellcheck \
      sudo \
      vim \
      zsh

# Install tools for building UI.
RUN curl -sL https://deb.nodesource.com/setup_22.x | sudo -E bash - && \
    apt update && \
    apt install nodejs && \
    npm install --global yarn

# Copy libnbd deb packages from build container and install the packages.
RUN mkdir -p /tmp/debs
COPY --from=build /tmp/debs/*.deb /tmp/debs
RUN cd /tmp/debs && dpkg --install *.deb

# Add vscode user and add it to sudoers.
RUN groupadd -g 1000 $USERNAME && \
    useradd -s /bin/bash -u $USER_UID -g $USER_GID -m $USERNAME && \
    mkdir -p /etc/sudoers.d && \
    echo $USERNAME ALL=\(root\) NOPASSWD:ALL > /etc/sudoers.d/$USERNAME && \
    chmod 0440 /etc/sudoers.d/$USERNAME

# Setup for vscode user.
USER $USERNAME
ENV EDITOR=vi \
    LANG=en_US.UTF-8
# Build Go tools with user vscode to ensure correct file and directory permissions for the build artifacts.
RUN go install -v github.com/google/go-licenses@latest && \
    go install -v github.com/766b/go-outliner@latest && \
    GOTOOLCHAIN="" go install -v golang.org/x/tools/gopls@latest && \
    go install -v github.com/go-delve/delve/cmd/dlv@latest && \
    go install -v golang.org/x/tools/cmd/goimports@latest && \
    go install -v golang.org/x/vuln/cmd/govulncheck@latest && \
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin
USER root

# Since we use a volume for /go to persist the content between executions, we need to preserve the binaries.
RUN mv /go/bin/* /usr/local/bin

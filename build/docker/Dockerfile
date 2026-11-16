# Build container. Ubuntu 24.04 is the oldest base with GTK4 and WebKitGTK 6.0,
# which is what sets the minimum glibc a release runs on.
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
        build-essential \
        ca-certificates \
        curl \
        file \
        git \
        libgtk-4-dev \
        libwebkitgtk-6.0-dev \
        nodejs \
        npm \
        pkg-config \
        xz-utils \
    && rm -rf /var/lib/apt/lists/*

# Toolchain checksums come from https://go.dev/dl/?mode=json
ARG GO_VERSION=1.27.4
ARG GO_SHA256=7226736c2339719cda28fbf3a54d71aa63bcefe376e98cef00034268dab24840
RUN curl -fsSL -o /tmp/go.tar.gz "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" \
    && printf '%s  %s\n' "${GO_SHA256}" /tmp/go.tar.gz | sha256sum -c - \
    && tar -xzf /tmp/go.tar.gz -C /usr/local \
    && rm /tmp/go.tar.gz
ENV PATH=/usr/local/go/bin:${PATH}

ARG WAILS_VERSION=v3.1.0
RUN GOBIN=/usr/local/bin go install "github.com/wailsapp/wails/v3/cmd/wails3@${WAILS_VERSION}"

WORKDIR /work

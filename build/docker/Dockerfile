# Build container. Optional, for people who would rather not install a C
# toolchain and the GL headers. The AppImage is built here so it is reproducible.
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
        build-essential \
        ca-certificates \
        curl \
        file \
        git \
        libgl1-mesa-dev \
        libwayland-dev \
        libxkbcommon-dev \
        pkg-config \
        wayland-protocols \
        xorg-dev \
        xz-utils \
    && rm -rf /var/lib/apt/lists/*

# Toolchain checksums come from https://go.dev/dl/?mode=json
ARG GO_VERSION=1.27.4
ARG GO_SHA256=7226736c2339719cda28fbf3a54d71aa63bcefe376e98cef00034268dab24840
RUN curl -fsSL -o /tmp/go.tar.gz "https://go.dev/dl/${GO_VERSION}.linux-amd64.tar.gz" \
    && printf '%s  %s\n' "${GO_SHA256}" /tmp/go.tar.gz | sha256sum -c - \
    && tar -xzf /tmp/go.tar.gz -C /usr/local \
    && rm /tmp/go.tar.gz
ENV PATH=/usr/local/go/bin:${PATH}

WORKDIR /work

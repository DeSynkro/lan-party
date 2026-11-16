#!/usr/bin/env bash
# Installs the pinned Go toolchain into .tools/go so the project does not
# depend on whatever Go happens to be on PATH.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
version="$(tr -d '[:space:]' < "${root}/.go-version")"
tools="${root}/.tools"
target="${tools}/go"

case "$(uname -m)" in
    x86_64) arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    *) printf 'unsupported architecture: %s\n' "$(uname -m)" >&2; exit 1 ;;
esac

# Checksums come from https://go.dev/dl/?mode=json
case "${arch}" in
    amd64) sha=7226736c2339719cda28fbf3a54d71aa63bcefe376e98cef00034268dab24840 ;;
    arm64) sha=3392582988a273a87ab5043ee1b2c9c17a2c3c00580bf9326c47a2e0aaaeac92 ;;
esac

if [ -x "${target}/bin/go" ] && [ "$("${target}/bin/go" env GOVERSION)" = "go${version}" ]; then
    printf 'go %s already installed\n' "${version}"
    exit 0
fi

file="go${version}.linux-${arch}.tar.gz"
tmp="$(mktemp -d)"
trap 'rm -rf "${tmp}"' EXIT

printf 'downloading %s\n' "${file}"
curl -fsSL -o "${tmp}/${file}" "https://go.dev/dl/${file}"
printf '%s  %s\n' "${sha}" "${tmp}/${file}" | sha256sum -c -

mkdir -p "${tools}"
rm -rf "${target}"
tar -xzf "${tmp}/${file}" -C "${tools}"

printf 'installed go %s into %s\n' "${version}" "${target}"

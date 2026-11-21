#!/usr/bin/env bash
# Builds the standalone LAN Party relay release zips.
#
# Usage:
#   VERSION=1.2.0 ARCHES="amd64 arm64" build/relay.sh
#   VERSION=1.2.0 ARCHES=host build/relay.sh
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${root}"

go_bin="${GO:-}"
if [ -z "${go_bin}" ] && [ -x "${root}/.tools/go/bin/go" ]; then
    go_bin="${root}/.tools/go/bin/go"
fi
go_bin="${go_bin:-go}"
version="${VERSION:-}"
if [ -z "${version}" ]; then
    version="$(git -C "${root}" describe --tags --abbrev=0 --match 'v[0-9]*' 2>/dev/null |
        sed -e 's/^v//' || true)"
fi
version="${version:-dev}"
arches="${ARCHES:-amd64 arm64}"
out="${root}/dist"
work="${root}/.cache/relay"

rm -rf "${work}"
mkdir -p "${work}" "${out}"
rm -f "${out}"/lan-party-relay_*.zip

for requested_arch in ${arches}; do
    arch="${requested_arch}"
    if [ "${arch}" = "host" ]; then
        arch="$("${go_bin}" env GOARCH)"
    fi
    case "${arch}" in
        amd64 | arm64) ;;
        *)
            echo "unsupported relay architecture: ${arch}" >&2
            exit 1
            ;;
    esac

    name="lan-party-relay_${version}_linux_${arch}"
    pkg="${work}/${name}"
    mkdir -p "${pkg}"

    CGO_ENABLED=0 GOOS=linux GOARCH="${arch}" "${go_bin}" build -trimpath \
        -ldflags "-s -w -X github.com/desynkro/lan-party/internal/buildinfo.Version=${version}" \
        -o "${pkg}/lan-party-relay" ./cmd/lan-party-relay

    install -m 0644 "${root}/relay/README.md" "${pkg}/README.md"
    install -m 0644 "${root}/LICENSE" "${root}/NOTICE" "${root}/THIRD_PARTY_NOTICES.md" "${pkg}/"

    cp -a "${root}/relay/docker" "${pkg}/docker"
    rm -rf "${pkg}/docker/data" "${pkg}/docker/.env"
    # Source-checkout only; the zip uses the prebuilt binary and published image.
    rm -f "${pkg}/docker/compose.override.yaml" "${pkg}/docker/Dockerfile.dev"
    cp -a "${root}/relay/native" "${pkg}/native"
    cp "${root}/relay/.dockerignore" "${pkg}/.dockerignore"

    "${go_bin}" version -m "${pkg}/lan-party-relay" > "${pkg}/lan-party-relay-build.txt"

    (cd "${work}" && zip -qr "${out}/${name}.zip" "${name}")
    printf 'built %s\n' "${out}/${name}.zip"
done

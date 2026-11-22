#!/usr/bin/env bash
# Builds the LAN Party web host release zips.
#
# Usage:
#   VERSION=1.3.0 ARCHES="amd64 arm64" build/web.sh
#   VERSION=dev ARCHES=host build/web.sh
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
work="${root}/.cache/web"

rm -rf "${work}"
mkdir -p "${work}" "${out}"
rm -f "${out}"/lan-party-web_*.zip

for requested_arch in ${arches}; do
    arch="${requested_arch}"
    if [ "${arch}" = "host" ]; then
        arch="$("${go_bin}" env GOARCH)"
    fi
    case "${arch}" in
        amd64 | arm64) ;;
        *)
            echo "unsupported web host architecture: ${arch}" >&2
            exit 1
            ;;
    esac

    name="lan-party-web_${version}_linux_${arch}"
    pkg="${work}/${name}"
    mkdir -p "${pkg}"

    CGO_ENABLED=0 GOOS=linux GOARCH="${arch}" "${go_bin}" build -trimpath \
        -ldflags "-s -w -X github.com/desynkro/lan-party/internal/buildinfo.Version=${version}" \
        -o "${pkg}/lan-party-web" ./server/cmd/lan-party-web

    install -m 0644 "${root}/LICENSE" "${root}/NOTICE" "${root}/THIRD_PARTY_NOTICES.md" "${pkg}/"
    sed 's|](\.\./docs/web-host.md)|](docs/web-host.md)|' "${root}/server/README.md" >"${pkg}/README.md"

    mkdir -p "${pkg}/docker" "${pkg}/native"
    (cd "${root}/server/docker" && tar --exclude=./data --exclude=./tls --exclude=./.env --exclude=./compose.override.yaml -cf - .) |
        tar -xf - -C "${pkg}/docker"
    (cd "${root}/server/native" && tar --exclude=./data --exclude=./web.env -cf - .) |
        tar -xf - -C "${pkg}/native"
    mkdir -p "${pkg}/docs"
    sed -e 's|](\.\./server/docker/README.md)|](../docker/README.md)|' \
        -e 's|](\.\./server/native/README.md)|](../native/README.md)|' \
        "${root}/docs/web-host.md" >"${pkg}/docs/web-host.md"

    "${go_bin}" version -m "${pkg}/lan-party-web" >"${pkg}/lan-party-web-build.txt"

    (cd "${work}" && zip -qr "${out}/${name}.zip" "${name}")
    printf 'built %s\n' "${out}/${name}.zip"
done

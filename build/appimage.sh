#!/usr/bin/env bash
# Builds the Linux AppImage. Everything it needs is downloaded into .tools and
# checked against a pinned checksum, so nothing is installed on the machine.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${root}"

go_bin="${root}/.tools/go/bin/go"
if [ ! -x "${go_bin}" ]; then
    go_bin="$(command -v go)"
fi

version="${VERSION:-}"
if [ -z "${version}" ]; then
    version="$(git describe --tags --abbrev=0 --match 'v[0-9]*' 2>/dev/null | sed 's/^v//')" || true
    version="${version:-dev}"
fi
out="${root}/dist"
work="${root}/.cache/appimage"
tool="${root}/.tools/appimagetool"

appimagetool_url="https://github.com/AppImage/appimagetool/releases/download/1.9.1/appimagetool-x86_64.AppImage"
appimagetool_sha="ed4ce84f0d9caff66f50bcca6ff6f35aae54ce8135408b3fa33abfc3cb384eb0"

if [ ! -x "${tool}" ]; then
    mkdir -p "$(dirname "${tool}")"
    curl -fsSL -o "${tool}" "${appimagetool_url}"
    printf '%s  %s\n' "${appimagetool_sha}" "${tool}" | sha256sum -c -
    chmod +x "${tool}"
fi

rm -rf "${work}"
mkdir -p "${work}/usr/bin" "${work}/usr/share/applications" "${out}"

"${go_bin}" build -trimpath \
    -ldflags "-s -w -X github.com/desynkro/lan-party/internal/buildinfo.Version=${version}" \
    -o "${work}/usr/bin/lan-party" ./cmd/lan-party

install -m 644 "${root}/internal/ui/assets/icon.png" "${work}/lan-party.png"

cat > "${work}/lan-party.desktop" <<DESKTOP
[Desktop Entry]
Type=Application
Name=LAN Party
Comment=One port, not the whole network
Exec=lan-party
Icon=lan-party
Categories=Network;Game;
Terminal=false
DESKTOP
install -m 644 "${work}/lan-party.desktop" "${work}/usr/share/applications/lan-party.desktop"

cat > "${work}/AppRun" <<'APPRUN'
#!/bin/sh
here="$(dirname "$(readlink -f "$0")")"
exec "${here}/usr/bin/lan-party" "$@"
APPRUN
chmod 755 "${work}/AppRun"

target="${out}/lan-party_${version}_linux_amd64.AppImage"
ARCH=x86_64 "${tool}" --appimage-extract-and-run "${work}" "${target}"

printf 'built %s\n' "${target}"

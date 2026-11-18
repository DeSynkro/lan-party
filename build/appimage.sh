#!/usr/bin/env bash
# Builds the Linux AppImage. The current package uses the system GTK4 runtime;
# bundling GTK is a separate packaging decision.
set -euo pipefail

# Keep the Go code at the baseline x86-64 level for older CPUs.
export GOAMD64=v1

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
mkdir -p \
    "${work}/usr/bin" \
    "${work}/usr/share/applications" \
    "${work}/usr/share/icons/hicolor/256x256/apps" \
    "${work}/usr/share/licenses/lan-party" \
    "${work}/brand" \
    "${work}/internal/gtkui/assets" \
    "${out}"

"${go_bin}" build -trimpath \
    -ldflags "-s -w -X github.com/desynkro/lan-party/internal/buildinfo.Version=${version}" \
    -o "${work}/usr/bin/lan-party" ./cmd/lan-party

cp -a "${root}/brand/fonts" "${work}/brand/fonts"
cp -a "${root}/brand/png" "${work}/brand/png"
cp -a "${root}/internal/gtkui/style.css" "${work}/internal/gtkui/style.css"
cp -a "${root}/internal/gtkui/assets/fonts" "${work}/internal/gtkui/assets/fonts"
cp -a "${root}/internal/gtkui/assets/icons" "${work}/internal/gtkui/assets/icons"

install -m 644 "${root}/LICENSE" \
    "${work}/usr/share/licenses/lan-party/LICENSE"
install -m 644 "${root}/NOTICE" \
    "${work}/usr/share/licenses/lan-party/NOTICE"
install -m 644 "${root}/THIRD_PARTY_NOTICES.md" \
    "${work}/usr/share/licenses/lan-party/THIRD_PARTY_NOTICES.md"
install -m 644 "${root}/brand/fonts/OFL-SpaceGrotesk.txt" \
    "${work}/usr/share/licenses/lan-party/OFL-SpaceGrotesk.txt"
install -m 644 "${root}/internal/gtkui/assets/fonts/OFL-IBMPlexSans.txt" \
    "${work}/usr/share/licenses/lan-party/OFL-IBMPlexSans.txt"
install -m 644 "${root}/internal/gtkui/assets/fonts/OFL-JetBrainsMono.txt" \
    "${work}/usr/share/licenses/lan-party/OFL-JetBrainsMono.txt"

source_rev="$(git -C "${root}" describe --tags --always --dirty 2>/dev/null || echo unknown)"
cat > "${work}/usr/share/licenses/lan-party/SOURCE" <<SOURCE
LAN Party ${version}
Source: https://github.com/DeSynkro/lan-party
Source revision: ${source_rev}
License: GPLv3
SOURCE

install -m 644 "${root}/brand/png/icon-app-256.png" "${work}/lan-party.png"
install -m 644 "${root}/brand/png/icon-app-256.png" \
    "${work}/usr/share/icons/hicolor/256x256/apps/lan-party.png"

cat > "${work}/lan-party.desktop" <<DESKTOP
[Desktop Entry]
Type=Application
Name=LAN Party
Comment=A LAN cable across the internet
Exec=lan-party
Icon=lan-party
Categories=Network;Game;
Terminal=false
StartupWMClass=lan-party
DESKTOP
install -m 644 "${work}/lan-party.desktop" \
    "${work}/usr/share/applications/lan-party.desktop"

cat > "${work}/AppRun" <<'APPRUN'
#!/bin/sh
here="$(dirname "$(readlink -f "$0")")"
cd "$here" || exit 1
exec "$here/usr/bin/lan-party" "$@"
APPRUN
chmod 755 "${work}/AppRun"

target="${out}/lan-party_${version}_linux_amd64.AppImage"
tmp_target="${out}/.lan-party_${version}_linux_amd64.tmp.AppImage"
rm -f "${tmp_target}"
ARCH=x86_64 "${tool}" --appimage-extract-and-run "${work}" "${tmp_target}"
mv -f "${tmp_target}" "${target}"

printf 'built %s\n' "${target}"

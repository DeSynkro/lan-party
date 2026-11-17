#!/usr/bin/env bash
# Builds a test AppImage for the GTK interface. It uses the system GTK4 and is
# not the final portable package; the Fyne AppImage remains separate.
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
work="${root}/.cache/appimage-gtk"
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
    "${work}/brand" \
    "${work}/internal/gtkui/assets" \
    "${out}"

"${go_bin}" build -trimpath \
    -ldflags "-s -w -X github.com/desynkro/lan-party/internal/buildinfo.Version=${version}" \
    -o "${work}/usr/bin/gtk-probe" ./cmd/gtk-probe

cp -a "${root}/brand/fonts" "${work}/brand/fonts"
cp -a "${root}/brand/png" "${work}/brand/png"
cp -a "${root}/internal/gtkui/style.css" "${work}/internal/gtkui/style.css"
cp -a "${root}/internal/gtkui/assets/fonts" "${work}/internal/gtkui/assets/fonts"
cp -a "${root}/internal/gtkui/assets/icons" "${work}/internal/gtkui/assets/icons"

install -m 644 "${root}/brand/png/icon-app-256.png" "${work}/lan-party.png"
install -m 644 "${root}/brand/png/icon-app-256.png" \
    "${work}/usr/share/icons/hicolor/256x256/apps/lan-party.png"

cat > "${work}/lan-party-gtk.desktop" <<DESKTOP
[Desktop Entry]
Type=Application
Name=LAN Party (GTK)
Comment=A LAN cable across the internet
Exec=gtk-probe
Icon=lan-party
Categories=Network;Game;
Terminal=false
StartupWMClass=gtk-probe
DESKTOP
install -m 644 "${work}/lan-party-gtk.desktop" \
    "${work}/usr/share/applications/lan-party-gtk.desktop"

cat > "${work}/AppRun" <<'APPRUN'
#!/bin/sh
here="$(dirname "$(readlink -f "$0")")"
cd "$here" || exit 1
exec "$here/usr/bin/gtk-probe" "$@"
APPRUN
chmod 755 "${work}/AppRun"

target="${out}/lan-party-gtk_${version}_linux_amd64.AppImage"
ARCH=x86_64 "${tool}" --appimage-extract-and-run "${work}" "${target}"

printf 'built %s\n' "${target}"

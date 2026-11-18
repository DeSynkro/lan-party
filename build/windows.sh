#!/usr/bin/env bash
# Builds the portable Windows zip. Run this inside an MSYS2 UCRT64 shell.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${root}"

prefix="${MSYSTEM_PREFIX:-/ucrt64}"
go_bin="${GO_BIN:-go}"
version="${VERSION:-dev}"
out="${root}/dist"
work="${root}/.cache/windows"
app="${work}/lan-party"
archive="${out}/lan-party_${version}_windows_amd64.zip"

export CC="${CC:-gcc}"
export CGO_ENABLED=1
export PKG_CONFIG_PATH="${PKG_CONFIG_PATH:-${prefix}/lib/pkgconfig}"

rm -rf "${work}"
mkdir -p "${app}/share/lan-party" "${out}"

"${go_bin}" build -trimpath \
    -ldflags "-s -w -H=windowsgui -X github.com/desynkro/lan-party/internal/buildinfo.Version=${version}" \
    -o "${app}/lan-party.exe" ./cmd/lan-party

# A console build for diagnosing startup problems. The release exe above is a
# GUI subsystem binary and does not open a terminal.
"${go_bin}" build -trimpath \
    -ldflags "-s -w -X github.com/desynkro/lan-party/internal/buildinfo.Version=${version}" \
    -o "${app}/lan-party-console.exe" ./cmd/lan-party

# App assets are found from <exe dir>/share/lan-party by assetRoot.
mkdir -p "${app}/share/lan-party/internal/gtkui"
cp -a "${root}/brand" "${app}/share/lan-party/brand"
cp -a "${root}/internal/gtkui/style.css" "${app}/share/lan-party/internal/gtkui/style.css"
cp -a "${root}/internal/gtkui/assets" "${app}/share/lan-party/internal/gtkui/assets"

# Licence files travel with the build.
mkdir -p "${app}/share/lan-party/licenses"
install -m 644 "${root}/LICENSE" "${root}/NOTICE" "${root}/THIRD_PARTY_NOTICES.md" \
    "${app}/share/lan-party/licenses/"
install -m 644 "${root}/brand/fonts/OFL-SpaceGrotesk.txt" \
    "${app}/share/lan-party/licenses/"
install -m 644 "${root}/internal/gtkui/assets/fonts/OFL-IBMPlexSans.txt" \
    "${app}/share/lan-party/licenses/"
install -m 644 "${root}/internal/gtkui/assets/fonts/OFL-JetBrainsMono.txt" \
    "${app}/share/lan-party/licenses/"

source_rev="$(git -C "${root}" describe --tags --always --dirty 2>/dev/null || echo unknown)"
cat > "${app}/share/lan-party/licenses/SOURCE" <<SOURCE
LAN Party ${version}
Source: https://github.com/DeSynkro/lan-party
Source revision: ${source_rev}
License: GPLv3
SOURCE

# Copy the UCRT64 DLL closure. ldd gives direct dependencies; repeat over every
# copied DLL so GTK's transitive dependencies and the pixbuf loaders come too.
for _ in 1 2 3 4 5; do
    changed=0
    while IFS= read -r file; do
        while IFS= read -r dll; do
            base="$(basename "${dll}")"
            if [ ! -f "${app}/${base}" ]; then
                cp -a "${dll}" "${app}/"
                changed=1
            fi
        done < <(PATH="${app}:${prefix}/bin:${PATH}" ldd "${file}" 2>/dev/null |
            grep -oE "${prefix}/[^ ]+\.dll" || true)
    done < <(find "${app}" -type f \( -name '*.exe' -o -name '*.dll' \))
    [ "${changed}" -eq 0 ] && break
done

# Compiler runtime is occasionally hidden from ldd.
for dll in libgcc_s_seh-1.dll libwinpthread-1.dll libstdc++-6.dll; do
    if [ -f "${prefix}/bin/${dll}" ] && [ ! -f "${app}/${dll}" ]; then
        cp -a "${prefix}/bin/${dll}" "${app}/"
    fi
done

# GTK data directories. Dereference symlinks so Explorer's zip can extract them.
for dir in share/glib-2.0 share/gtk-4.0 share/icons share/fontconfig etc/gtk-4.0 etc/fonts; do
    if [ -d "${prefix}/${dir}" ]; then
        mkdir -p "${app}/$(dirname "${dir}")"
        cp -aL "${prefix}/${dir}" "${app}/${dir}"
    fi
done

if [ -d "${prefix}/lib/gio/modules" ]; then
    mkdir -p "${app}/lib/gio"
    cp -aL "${prefix}/lib/gio/modules" "${app}/lib/gio/modules"
fi

# gdk-pixbuf loaders with a cache that uses paths relative to this folder.
mkdir -p "${app}/lib/gdk-pixbuf-2.0/2.10.0/loaders"
if [ -d "${prefix}/lib/gdk-pixbuf-2.0/2.10.0/loaders" ]; then
    cp -aL "${prefix}/lib/gdk-pixbuf-2.0/2.10.0/loaders/." \
        "${app}/lib/gdk-pixbuf-2.0/2.10.0/loaders/"
fi
(cd "${app}" && "${prefix}/bin/gdk-pixbuf-query-loaders.exe" \
    lib/gdk-pixbuf-2.0/2.10.0/loaders/*.dll \
    > lib/gdk-pixbuf-2.0/2.10.0/loaders.cache)

if [ -d "${app}/share/glib-2.0/schemas" ]; then
    "${prefix}/bin/glib-compile-schemas.exe" "${app}/share/glib-2.0/schemas"
fi

cat > "${app}/lan-party.cmd" <<'CMD'
@echo off
setlocal
cd /d "%~dp0"
set "GTK_DATA_PREFIX=%~dp0"
set "GTK_EXE_PREFIX=%~dp0"
set "GDK_PIXBUF_MODULE_FILE=%~dp0lib\gdk-pixbuf-2.0\2.10.0\loaders.cache"
start "" "%~dp0lan-party.exe"
CMD

cat > "${app}/README.txt" <<README
LAN Party ${version} (Windows test build)

Run lan-party.cmd. The first launch may show a Windows Defender SmartScreen
warning because this build is not signed.

If it does not start, run lan-party-console.exe from a Command Prompt to see
the log.

The host needs one UDP port forwarded and a public address. The first host run
may ask for Windows Firewall access; allow it on private networks.
README

"${go_bin}" version -m "${app}/lan-party.exe" > "${out}/lan-party-build.txt"
(cd "${work}" && zip -qr "${archive}" lan-party)
printf 'built %s\n' "${archive}"

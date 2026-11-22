#!/usr/bin/env bash
# Prepare the web host: create ./data and ./tls, write .env for this user, and
# set the admin password. Run it from this directory. No root needed if your
# user owns the directory; use sudo only when the files are root-owned.
set -euo pipefail

dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$dir"

check=0
for arg in "$@"; do
    case "$arg" in
        --check) check=1 ;;
        -h|--help)
            printf 'Usage: %s [--check]\n' "$(basename "$0")"
            printf '  --check  print what would happen without changing anything\n'
            exit 0
            ;;
        *)
            printf 'unknown option: %s\n' "$arg" >&2
            exit 2
            ;;
    esac
done

uid="${SUDO_UID:-$(id -u)}"
gid="${SUDO_GID:-$(id -g)}"

if [ "$check" = 1 ]; then
    printf 'data directory: %s/data (0700, owner %s:%s)\n' "$dir" "$uid" "$gid"
    printf 'tls directory:  %s/tls (0700)\n' "$dir"
    printf 'env file:       %s/.env (0600)\n' "$dir"
    if [ -f "$dir/data/web.json" ]; then
        printf 'admin password: already set\n'
    else
        printf 'admin password: would be set with docker compose run\n'
    fi
    exit 0
fi

umask 077
mkdir -p data tls
chmod 700 data tls
if [ "$(id -u)" = 0 ] && { [ "$uid" != 0 ] || [ "$gid" != 0 ]; }; then
    chown "$uid:$gid" data tls
fi

if [ ! -f .env ]; then
    sed -e "s/^PUID=.*/PUID=${uid}/" -e "s/^PGID=.*/PGID=${gid}/" .env.example >.env
    chmod 600 .env
    printf 'wrote .env for %s:%s\n' "$uid" "$gid"
else
    chmod 600 .env
    printf 'keeping the existing .env\n'
fi

if [ ! -f data/web.json ]; then
    printf '\nSet the admin password for the web UI.\n'
    docker compose run --rm lan-party-web setup --data-dir /data
fi

port="$(sed -n 's/^LANPARTY_WEB_PORT=//p' .env | tail -n 1)"
port="${port:-8080}"

printf '\nReady.\n\n'
printf '  Start:  docker compose up -d\n'
printf '  Open:   http://127.0.0.1:%s\n' "$port"
printf '  Logs:   docker compose logs -f lan-party-web\n'
printf '  Help:   ./webctl show-info\n\n'
printf 'Forward the UDP app port from .env on your router before inviting friends.\n'

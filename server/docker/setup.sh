#!/usr/bin/env bash
# Prepare the web host: create ./data and ./tls, write .env for this user, and
# set the admin password. Run it from this directory. No root needed if your
# user owns the directory; use sudo only when Docker or the files need it.
set -euo pipefail

dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$dir"

check=0
ports_only=0
for arg in "$@"; do
    case "$arg" in
        --check) check=1 ;;
        --ports) ports_only=1 ;;
        -h|--help)
            printf 'Usage: %s [--check|--ports]\n' "$(basename "$0")"
            printf '  --check  print what would happen without changing anything\n'
            printf '  --ports  change the web UI and UDP game ports, then exit\n'
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

current_value() {
    local key=$1 fallback=$2 value
    value="$(sed -n "s/^${key}=//p" .env 2>/dev/null | tail -n 1)"
    printf '%s' "${value:-$fallback}"
}

valid_port() {
    case "$1" in
        ''|*[!0-9]*) return 1 ;;
    esac
    [ "$1" -ge 1 ] && [ "$1" -le 65535 ]
}

web_port="$(current_value LANPARTY_WEB_PORT 8080)"
app_port="$(current_value LANPARTY_APP_PORT 51820)"

if [ "$check" = 1 ]; then
    printf 'data directory: %s/data (0700, owner %s:%s)\n' "$dir" "$uid" "$gid"
    printf 'tls directory:  %s/tls (0700)\n' "$dir"
    printf 'env file:       %s/.env (0600)\n' "$dir"
    printf 'web UI port:    %s/tcp\n' "$web_port"
    printf 'game port:      %s/udp\n' "$app_port"
    if [ -f "$dir/data/web.json" ]; then
        printf 'admin password: already set\n'
    else
        printf 'admin password: would be set with docker compose run\n'
    fi
    exit 0
fi

umask 077
mkdir -p data tls
if ! chmod 700 data tls 2>/dev/null; then
    printf 'note: %s/data or %s/tls is not owned by you; use sudo to change it.\n' "$dir" "$dir" >&2
fi
if [ "$(id -u)" = 0 ] && { [ "$uid" != 0 ] || [ "$gid" != 0 ]; }; then
    chown "$uid:$gid" data tls
fi

new_env=0
if [ ! -f .env ]; then
    sed -e "s/^PUID=.*/PUID=${uid}/" -e "s/^PGID=.*/PGID=${gid}/" .env.example >.env
    chmod 600 .env
    new_env=1
    printf 'wrote .env for %s:%s\n' "$uid" "$gid"
else
    chmod 600 .env 2>/dev/null || true
    printf 'keeping the existing .env\n'
fi

if [ "$ports_only" = 1 ] || [ "$new_env" = 1 ]; then
    if [ -t 0 ]; then
        printf '\nPorts. Press Enter to keep the current value.\n'

        read -r -p "Web UI port [${web_port}]: " answer || true
        if [ -n "${answer:-}" ]; then
            if ! valid_port "$answer"; then
                printf 'That is not a valid port.\n' >&2
                exit 1
            fi
            web_port="$answer"
        fi

        read -r -p "UDP game port [${app_port}]: " answer || true
        if [ -n "${answer:-}" ]; then
            if ! valid_port "$answer"; then
                printf 'That is not a valid port.\n' >&2
                exit 1
            fi
            app_port="$answer"
        fi
    fi
    sed -i -e "s/^LANPARTY_WEB_PORT=.*/LANPARTY_WEB_PORT=${web_port}/" \
        -e "s/^LANPARTY_APP_PORT=.*/LANPARTY_APP_PORT=${app_port}/" .env
fi

if [ "$ports_only" = 1 ]; then
    printf '\nPorts updated: web UI %s/tcp, game %s/udp.\n' "$web_port" "$app_port"
    printf 'Restart to apply: sudo docker compose up -d\n'
    printf 'If you changed the UDP port, update the router forward too.\n'
    exit 0
fi

if [ ! -f data/web.json ]; then
    printf '\nSet the admin password for the web UI.\n'
    docker compose run --rm lan-party-web setup --data-dir /data
fi

printf '\nReady.\n\n'
printf '  Start:  sudo docker compose up -d\n'
printf '  Open:   http://127.0.0.1:%s\n' "$web_port"
printf '  Logs:   sudo docker compose logs -f lan-party-web\n'
printf '  Ports:  sudo ./setup.sh --ports\n'
printf '  Help:   ./webctl show-info\n\n'
printf 'Forward UDP %s on your router before inviting friends.\n' "$app_port"

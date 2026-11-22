#!/usr/bin/env bash
# Run the LAN Party web host from a release zip or a source checkout.
# Everything this script writes stays beside it: ./data and ./web.env.
set -euo pipefail

dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
data="${LANPARTY_WEB_DATA_DIR:-$dir/data}"
envfile="$dir/web.env"

if [ -f "$envfile" ]; then
    set -a
    # shellcheck disable=SC1090
    . "$envfile"
    set +a
fi

listen="${LANPARTY_WEB_LISTEN:-0.0.0.0:8080}"
app_port="${LANPARTY_APP_PORT:-51820}"
cert="${LANPARTY_WEB_TLS_CERT:-}"
key="${LANPARTY_WEB_TLS_KEY:-}"
proxies="${LANPARTY_WEB_TRUSTED_PROXIES:-}"

find_binary() {
    local candidate
    for candidate in "$dir/lan-party-web" "$dir/../lan-party-web" "$(command -v lan-party-web 2>/dev/null || true)"; do
        if [ -n "$candidate" ] && [ -x "$candidate" ]; then
            printf '%s' "$candidate"
            return 0
        fi
    done
    return 1
}

bin="$(find_binary || true)"
if [ -z "$bin" ]; then
    printf 'lan-party-web binary not found. Put it beside run.sh or in PATH.\n' >&2
    exit 1
fi

mkdir -p "$data"
chmod 700 "$data"

write_env() {
    {
        printf 'LANPARTY_WEB_LISTEN=%s\n' "$listen"
        printf 'LANPARTY_APP_PORT=%s\n' "$app_port"
        if [ -n "$cert" ]; then printf 'LANPARTY_WEB_TLS_CERT=%s\n' "$cert"; fi
        if [ -n "$key" ]; then printf 'LANPARTY_WEB_TLS_KEY=%s\n' "$key"; fi
        if [ -n "$proxies" ]; then printf 'LANPARTY_WEB_TRUSTED_PROXIES=%s\n' "$proxies"; fi
    } >"$envfile"
    chmod 600 "$envfile"
}

valid_port() {
    case "$1" in
        ''|*[!0-9]*) return 1 ;;
    esac
    [ "$1" -ge 1 ] && [ "$1" -le 65535 ]
}

first_run() {
    if [ ! -f "$envfile" ]; then
        printf 'First run. Press Enter to keep the defaults.\n\n'
        read -r -p "Web UI listen address [$listen]: " answer || true
        listen="${answer:-$listen}"
        read -r -p "UDP game port [$app_port]: " answer || true
        if [ -n "${answer:-}" ]; then
            if ! valid_port "$answer"; then
                printf 'That is not a valid port.\n' >&2
                exit 1
            fi
            app_port="$answer"
        fi
        write_env
    fi
    if [ ! -f "$data/web.json" ]; then
        printf '\nSet the admin password for the web UI.\n'
        "$bin" setup --data-dir "$data"
        printf '\n'
    fi
}

start_host() {
    local args=(
        run
        --data-dir "$data"
        --listen "$listen"
        --app-port "$app_port"
    )
    if [ -n "$cert" ] && [ -n "$key" ]; then
        args+=(--tls-cert "$cert" --tls-key "$key")
    fi
    if [ -n "$proxies" ]; then
        args+=(--trusted-proxies "$proxies")
    fi
    printf 'Starting LAN Party web host on %s\n' "$listen"
    printf 'Game port: %s/udp. Data: %s\n\n' "$app_port" "$data"
    exec "$bin" "${args[@]}"
}

show_info() {
    "$bin" show-info --data-dir "$data" --listen "$listen" --app-port "$app_port"
}

tail_log() {
    if [ -f "$data/lan-party.log" ]; then
        tail -n 40 "$data/lan-party.log"
    else
        printf 'No log file yet at %s\n' "$data/lan-party.log"
    fi
}

menu() {
    while true; do
        printf '\nLAN Party web host\n'
        printf '  1) Start\n'
        printf '  2) Show info\n'
        printf '  3) Change admin password\n'
        printf '  4) Show log tail\n'
        printf '  5) How to set the public address\n'
        printf '  q) Quit\n\n'
        read -r -p 'Choose: ' choice || exit 0
        case "$choice" in
            1) start_host ;;
            2) show_info ;;
            3) "$bin" passwd --data-dir "$data" ;;
            4) tail_log ;;
            5) printf '\nOpen the web UI, sign in, and set the public address in Settings.\n' ;;
            q|Q) exit 0 ;;
            *) printf 'Unknown choice.\n' ;;
        esac
    done
}

first_run

case "${1:-menu}" in
    menu) menu ;;
    start) start_host ;;
    *)
        # Pass other commands through, always using this data directory.
        exec "$bin" "$@" --data-dir "$data"
        ;;
esac

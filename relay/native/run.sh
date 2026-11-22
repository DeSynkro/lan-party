#!/usr/bin/env bash
# Run the LAN Party relay in a foreground console, without systemd or root.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
bin="${LANPARTY_RELAY_BIN:-}"
if [ -z "$bin" ]; then
    for candidate in \
        "$script_dir/../lan-party-relay" \
        "$script_dir/lan-party-relay" \
        /usr/local/bin/lan-party-relay; do
        if [ -x "$candidate" ]; then
            bin="$candidate"
            break
        fi
    done
fi

data_dir="${LANPARTY_RELAY_DIR:-${XDG_DATA_HOME:-$HOME/.local/share}/lan-party-relay}"
public_host=""
public_port=""
relay_pid=""

usage() {
    cat >&2 <<EOF
Usage: ./run.sh [--dir DIR] [--host HOST] [--port PORT]

Runs the relay in the foreground without systemd. The relay is stopped with
Ctrl+C or the Stop menu item. Data is kept under:
  ${data_dir}

Options:
  --dir DIR    relay data directory (default: LANPARTY_RELAY_DIR or
               ~/.local/share/lan-party-relay)
  --host HOST  public relay host or IP; applied on this start
  --port PORT  public relay port; also the local listen port
  -h, --help   show this help

The relay binary is found next to the release zip, in native/, or at
/usr/local/bin/lan-party-relay. Override it with LANPARTY_RELAY_BIN.
EOF
}

while [ $# -gt 0 ]; do
    case "$1" in
        --dir)
            [ $# -ge 2 ] || { usage; exit 2; }
            data_dir="$2"
            shift 2
            ;;
        --host)
            [ $# -ge 2 ] || { usage; exit 2; }
            public_host="$2"
            shift 2
            ;;
        --port)
            [ $# -ge 2 ] || { usage; exit 2; }
            public_port="$2"
            shift 2
            ;;
        -h | --help)
            usage
            exit 0
            ;;
        *)
            usage
            exit 2
            ;;
    esac
done

if [ -z "$bin" ]; then
    echo "lan-party-relay binary not found. Set LANPARTY_RELAY_BIN or run this from the release zip." >&2
    exit 1
fi

case "$data_dir" in
    "~/"*) data_dir="${HOME}/${data_dir#\~/}" ;;
esac

config="$data_dir/relay.json"
profile="$data_dir/relay-profile.json"
key="$data_dir/relay.key"
log="$data_dir/lan-party-relay.log"

mkdir -p "$data_dir"
chmod 700 "$data_dir" 2>/dev/null || true

profile_value() {
    "$bin" show-info --dir "$data_dir" 2>/dev/null |
        sed -n "s/^$1:[[:space:]]*//p" |
        head -n 1
}

valid_port() {
    [[ "$1" =~ ^[0-9]+$ ]] && [ "$1" -ge 1 ] && [ "$1" -le 65535 ]
}

prompt_host() {
    local default="$1" answer
    while :; do
        if [ -n "$default" ]; then
            read -r -p "Public relay host or IP [${default}]: " answer || return 1
            answer="${answer:-$default}"
        else
            read -r -p "Public relay host or IP: " answer || return 1
        fi
        if [ -n "$answer" ]; then
            public_host="$answer"
            return 0
        fi
        echo "The relay host is required." >&2
    done
}

prompt_port() {
    local default="$1" answer
    while :; do
        read -r -p "Public relay port [${default}]: " answer || return 1
        answer="${answer:-$default}"
        if valid_port "$answer"; then
            public_port="$answer"
            return 0
        fi
        echo "Enter a port between 1 and 65535." >&2
    done
}

stop_relay() {
    if [ -n "$relay_pid" ] && kill -0 "$relay_pid" 2>/dev/null; then
        kill "$relay_pid" 2>/dev/null || true
        wait "$relay_pid" 2>/dev/null || true
    fi
    relay_pid=""
}

cleanup() {
    stop_relay
    echo
    echo "Relay stopped."
}
trap 'cleanup; exit 0' INT TERM
trap stop_relay EXIT

start_relay() {
    stop_relay
    : >>"$log"
    chmod 600 "$log" 2>/dev/null || true
    LANPARTY_RELAY_PUBLIC_HOST="$public_host" \
        LANPARTY_RELAY_PUBLIC_PORT="$public_port" \
        "$bin" run --config "$config" >>"$log" 2>&1 &
    relay_pid=$!
    sleep 0.8
    if ! kill -0 "$relay_pid" 2>/dev/null; then
        echo "The relay failed to start. Recent log:" >&2
        tail -n 30 "$log" >&2 || true
        relay_pid=""
        return 1
    fi
    chmod 600 "$config" "$profile" "$key" 2>/dev/null || true
    echo "Relay running. Log: $log"
    tail -n 8 "$log" 2>/dev/null || true
}

show_info() {
    echo "Host-only relay: do not share the Relay ID or password." >&2
    echo "Keep this terminal private and clear it after copying." >&2
    "$bin" show-info --dir "$data_dir"
}

change_public() {
    local old_host="$public_host" old_port="$public_port"
    echo "Change the public relay host and port."
    prompt_host "$old_host" || { public_host="$old_host"; return; }
    prompt_port "$old_port" || { public_port="$old_port"; return; }
    if [ "$public_host" = "$old_host" ] && [ "$public_port" = "$old_port" ]; then
        echo "No change."
        return
    fi
    if "$bin" set-public --dir "$data_dir" --host "$public_host" --port "$public_port" --listen ":$public_port"; then
        start_relay
    else
        echo "Could not update the relay address. Keeping the old settings." >&2
        public_host="$old_host"
        public_port="$old_port"
    fi
}

change_password() {
    local first second
    printf 'New relay password: ' >&2
    IFS= read -r -s first || true
    printf '\n' >&2
    printf 'Repeat new relay password: ' >&2
    IFS= read -r -s second || true
    printf '\n' >&2
    if [ -z "$first" ]; then
        echo "The password is empty. Nothing changed." >&2
        return
    fi
    if [ "$first" != "$second" ]; then
        echo "The passwords do not match. Nothing changed." >&2
        return
    fi
    if printf '%s\n' "$first" | "$bin" passwd --password-stdin --dir "$data_dir"; then
        unset first second
        start_relay
        show_info
    fi
}

generate_password() {
    local answer
    echo "This replaces the relay password and restarts it." >&2
    read -r -p "Type GENPASS to continue: " answer || return
    if [ "$answer" != "GENPASS" ]; then
        echo "Cancelled."
        return
    fi
    if "$bin" passwd --generate --dir "$data_dir"; then
        start_relay
        show_info
    fi
}

rotate_key() {
    local answer
    echo "This replaces the relay key and Relay ID." >&2
    echo "Old pins and invites stop working. The password does not change." >&2
    read -r -p "Type RESET to continue: " answer || return
    if [ "$answer" != "RESET" ]; then
        echo "Cancelled."
        return
    fi
    stop_relay
    if "$bin" rotate-key --dir "$data_dir"; then
        start_relay
        show_info
    fi
}

show_log() {
    if [ -f "$log" ]; then
        tail -n 40 "$log"
    else
        echo "No log yet."
    fi
}

menu() {
    cat <<EOF

LAN Party relay console
  Data:   $data_dir
  Public: $public_host:$public_port

  1) Show relay info
  2) Change public host/port
  3) Change password
  4) Generate password
  5) Rotate Relay ID
  6) Show recent log
  7) Stop
EOF
}

# First run: create the relay key, config and profile.
if [ ! -f "$config" ]; then
    echo "First run: setting up the relay in $data_dir"
    if [ -z "$public_host" ]; then
        prompt_host "" || { cleanup; exit 1; }
    fi
    if [ -z "$public_port" ]; then
        prompt_port "7443" || { cleanup; exit 1; }
    fi
    if ! valid_port "$public_port"; then
        echo "The relay port must be between 1 and 65535." >&2
        exit 1
    fi
    if [ "$public_port" -lt 1024 ] && [ "$(id -u)" -ne 0 ]; then
        echo "Ports below 1024 need root. Choose a port above 1024 for console mode." >&2
        exit 1
    fi
    "$bin" setup --dir "$data_dir" --host "$public_host" --port "$public_port" --listen ":$public_port"
    chmod 600 "$config" "$profile" "$key" 2>/dev/null || true
    show_info
else
    [ -f "$profile" ] || {
        echo "The relay profile is missing from $data_dir. Move the data directory aside and run again." >&2
        exit 1
    }
    if [ -z "$public_host" ]; then
        public_host="$(profile_value "Relay host")"
    fi
    if [ -z "$public_port" ]; then
        public_port="$(profile_value "Relay port")"
    fi
    if [ -z "$public_host" ] || ! valid_port "$public_port"; then
        echo "The saved relay address is incomplete. Use --host and --port to choose new values." >&2
        exit 1
    fi
    # Keep the saved config and listen port aligned with the console values.
    "$bin" set-public --dir "$data_dir" --host "$public_host" --port "$public_port" --listen ":$public_port" >/dev/null
    chmod 600 "$config" "$profile" "$key" 2>/dev/null || true
fi

if [ "$public_port" -lt 1024 ] && [ "$(id -u)" -ne 0 ]; then
    echo "Ports below 1024 need root. Choose a port above 1024 for console mode." >&2
    exit 1
fi

start_relay || exit 1

while :; do
    menu
    if ! read -r -p "Choose: " choice; then
        cleanup
        exit 0
    fi
    case "$choice" in
        1) show_info ;;
        2) change_public ;;
        3) change_password ;;
        4) generate_password ;;
        5) rotate_key ;;
        6) show_log ;;
        7 | q | quit | stop)
            cleanup
            exit 0
            ;;
        *)
            echo "Unknown choice: $choice"
            ;;
    esac
    if ! kill -0 "$relay_pid" 2>/dev/null; then
        echo "The relay is no longer running. Recent log:" >&2
        tail -n 30 "$log" >&2 || true
        relay_pid=""
        read -r -p "Press Enter to exit. " _ || true
        exit 1
    fi
done

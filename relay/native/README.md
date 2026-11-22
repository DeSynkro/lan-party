# LAN Party relay: native Linux

This is the no-Docker install. It runs the same `lan-party-relay` binary as the
Docker image under a hardened systemd unit.

The relay is an opaque UDP forwarder. It cannot read the encrypted QUIC/TLS
traffic between the host and friends.

## Try it first: console mode, no root or systemd

The release zip includes `native/run.sh`. It runs the same relay binary in a
foreground console and keeps all of its data under one user-owned directory:

```sh
unzip lan-party-relay_1.2.0_linux_amd64.zip
cd lan-party-relay_1.2.0_linux_amd64
./native/run.sh
```

If the zip did not preserve the executable bit:

```sh
chmod +x native/run.sh
./native/run.sh
```

The first run asks for the public host and port, creates the relay key and
password, and starts the relay. It uses
`~/.local/share/lan-party-relay` by default, or `LANPARTY_RELAY_DIR` if set. No
system user, systemd unit or `/etc` files are created.

The console menu can:

- show the host, port, Relay ID and password
- change the public host or port, including after the first run
- change or generate the password
- rotate the Relay ID
- show the recent relay log
- stop the relay

Closing the terminal or pressing Ctrl+C stops the relay. This mode is for a
relay you run while hosting; it is not a service and does not start on boot.

You can also pass the initial values directly:

```sh
./native/run.sh --host relay.example.com --port 7443
```

The router/firewall still needs TCP and UDP on that port.

## Requirements (for the service install)

- Linux amd64 or arm64 with systemd
- a public IPv4 or IPv6 address, or a hostname that resolves to one
- TCP and UDP port `7443` reachable from the internet (or another port you
  choose)
- root or sudo for the install

## Install

Extract the relay zip and enter the directory:

```sh
unzip lan-party-relay_1.2.0_linux_amd64.zip
cd lan-party-relay_1.2.0_linux_amd64
```

Create the service user and install the binary and unit:

```sh
sudo useradd --system --home-dir /nonexistent --shell /usr/sbin/nologin lan-party-relay
sudo install -m 0755 lan-party-relay /usr/local/bin/lan-party-relay
sudo install -m 0755 native/relayctl /usr/local/bin/lan-party-relayctl
sudo install -m 0644 native/lan-party-relay.service /etc/systemd/system/lan-party-relay.service
sudo install -d -m 0700 /etc/lan-party-relay
```

Run setup. It asks for the public host and port. Use the hostname or IP that
friends will connect to:

```sh
sudo lan-party-relay setup --dir /etc/lan-party-relay
```

Examples:

```text
Host: relay.example.com
Port: 7443

Host: 203.0.113.10
Port: 8443
```

The port defaults to `7443`. By default the relay listens on `7443` even when
the public port is another port, such as `8443` behind a firewall redirect. To
bind the public port directly instead, pass `--listen`:

```sh
sudo lan-party-relay setup --dir /etc/lan-party-relay --listen :8443
```

Setup generates the relay identity key, the relay password and these files:

| File | Purpose | Keep private |
| --- | --- | --- |
| `/etc/lan-party-relay/relay.json` | relay config and password hash | yes |
| `/etc/lan-party-relay/relay.key` | relay identity key | yes |
| `/etc/lan-party-relay/relay-profile.json` | relay-side profile with the plaintext password | yes, contains the password |

The host profile is the only file that contains the relay password in plain
text. The relay itself stores only a salted PBKDF2 hash.

Let the service user read the config and key, but not the host profile:

```sh
sudo chown root:lan-party-relay /etc/lan-party-relay
sudo chmod 0750 /etc/lan-party-relay
sudo chown lan-party-relay:lan-party-relay /etc/lan-party-relay/relay.json /etc/lan-party-relay/relay.key
sudo chmod 0600 /etc/lan-party-relay/relay.json /etc/lan-party-relay/relay.key
sudo chmod 0600 /etc/lan-party-relay/relay-profile.json
```

Start the relay:

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now lan-party-relay
sudo systemctl status lan-party-relay
```

The unit reads `/etc/lan-party-relay/relay.json` by default. To override the
config path, copy `native/relay.env.example` to
`/etc/lan-party-relay/relay.env` and edit it.

Follow the logs to see the Relay ID and any errors:

```sh
sudo journalctl -u lan-party-relay -f
```

Open the relay port in the firewall. With ufw:

```sh
sudo ufw allow 7443/tcp
sudo ufw allow 7443/udp
```

Use the same TCP and UDP port. The control channel uses TCP; friend traffic uses
UDP.

## Shortcuts

The install step above puts the wrapper at
`/usr/local/bin/lan-party-relayctl`. It has the same commands as the Docker
wrapper:

```sh
sudo lan-party-relayctl showinfo
sudo lan-party-relayctl setpass
sudo lan-party-relayctl genpass
sudo lan-party-relayctl relayreset
```

You can also run `sudo ./native/relayctl ...` from the extracted release.

The wrapper uses `/etc/lan-party-relay` and the `lan-party-relay` service. The
raw `lan-party-relay` commands below still work for scripts and advanced use.

## Show the values for the host app

The UDP data port is bound only while a host session is active. When the host is
offline, only the TLS control port remains. A UDP port check may report the port
closed while the relay is idle; health checks should use TCP.

The profile stays root-only on the relay. Display the values instead of copying
the file:

```sh
sudo lan-party-relay show-info --dir /etc/lan-party-relay
```

Enter the address, Relay ID and password in LAN Party under
**Settings > Relay**, then use **Test relay**. The app pins the Relay ID.

This relay is host-only infrastructure. Do not post the Relay ID or give out
the password; friends join through invites.

The password is printed to the terminal only. Keep that terminal private and
clear it after copying. If the profile is ever deleted, `passwd` recreates it
with the new password; the password cannot be recovered from `relay.json`.

## Change the public host or port (service install)

The console wrapper has a menu option for this. For a service install, stop the
service, update the config and profile, fix ownership, and start it again:

```sh
sudo systemctl stop lan-party-relay
sudo lan-party-relay set-public --dir /etc/lan-party-relay \
  --host relay.example.net --port 7443 --listen :7443
sudo chown lan-party-relay:lan-party-relay /etc/lan-party-relay/relay.json /etc/lan-party-relay/relay.key
sudo chmod 0600 /etc/lan-party-relay/relay.json /etc/lan-party-relay/relay.key
sudo chmod 0600 /etc/lan-party-relay/relay-profile.json
sudo systemctl start lan-party-relay
sudo lan-party-relay show-info --dir /etc/lan-party-relay
```

`--listen :7443` keeps the local bind port aligned with the public port. If a
firewall redirects a different public port to 7443, omit `--listen`.

## Password and key rotation

Use the shortcuts. They prompt without echoing, restart the service and show the
new values:

```sh
sudo lan-party-relayctl setpass
sudo lan-party-relayctl genpass
sudo lan-party-relayctl relayreset
```

If you did not install the wrapper, run `sudo ./native/relayctl ...` from the
extracted release instead.

The raw commands are for scripts. To choose a password before the first setup,
the setup accepts `--password`, but it appears in shell history. The safer path
is to let setup generate one, then run `relayctl setpass`.

Change the password later with stdin:

```sh
read -r -s -p 'New relay password: ' pw; echo
printf '%s\n' "$pw" | sudo lan-party-relay passwd --password-stdin --dir /etc/lan-party-relay
unset pw
sudo systemctl restart lan-party-relay
sudo lan-party-relay show-info --dir /etc/lan-party-relay
```

Or generate and set a strong random password:

```sh
sudo lan-party-relay passwd --generate --dir /etc/lan-party-relay
sudo systemctl restart lan-party-relay
sudo lan-party-relay show-info --dir /etc/lan-party-relay
```

The Relay ID stays the same. Enter the new password in the app.

The Relay ID is not a secret, but this is private infrastructure. Do not post
it. It is an identifier, not an access grant. If the private `relay.key` is
compromised, or you want a new relay identity, rotate it with the service
stopped:

```sh
sudo systemctl stop lan-party-relay
sudo lan-party-relay rotate-key --dir /etc/lan-party-relay
sudo chown lan-party-relay:lan-party-relay /etc/lan-party-relay/relay.key
sudo chmod 0600 /etc/lan-party-relay/relay.key
sudo systemctl start lan-party-relay
sudo lan-party-relay show-info --dir /etc/lan-party-relay
```

The chown/chmod keeps the new key readable by the service user. The
`native/relayctl relayreset` shortcut does this for you.

This changes the Relay ID. Enter the new Relay ID in the app and issue new
invites; old pins and invites that carry the old Relay ID stop working.

## Updating

The config, key and Relay ID survive an update. Replace the binary and
restart:

```sh
sudo systemctl stop lan-party-relay
sudo install -m 0755 lan-party-relay /usr/local/bin/lan-party-relay
sudo systemctl start lan-party-relay
```

Do not regenerate the key during an update. A new key means the host app must
re-enter the Relay ID, and existing invites stop working.

## Uninstall

```sh
sudo systemctl disable --now lan-party-relay
sudo rm -f /etc/systemd/system/lan-party-relay.service
sudo rm -f /usr/local/bin/lan-party-relay
sudo rm -rf /etc/lan-party-relay
sudo userdel lan-party-relay
```

Removing `/etc/lan-party-relay` deletes the relay identity key and password
hash. This cannot be undone.

## Troubleshooting

**The service exits immediately.** Run the Relay ID command as the service
user to see the same check the unit runs:

```sh
sudo -u lan-party-relay lan-party-relay relay-id --config /etc/lan-party-relay/relay.json
```

Common causes are a missing file, a config or key readable by other users, or
the service user lacking permission to traverse `/etc/lan-party-relay`.

**Address already in use.** Another process owns TCP or UDP 7443. Check with:

```sh
sudo ss -lntup | grep 7443
```

**Friends cannot connect.** Confirm TCP and UDP 7443 are open in the host
firewall and the VPS provider's firewall. The control channel is TCP; friend
traffic is UDP. A TCP-only check is not enough.

**The app says the Relay ID changed.** The relay key changed. Only re-enter it
if you changed it intentionally. Otherwise treat it as a possible
man-in-the-middle and investigate before importing anything.

## Security notes

- Run the relay as the dedicated `lan-party-relay` user, never as root.
- The unit uses a read-only filesystem, no capabilities, no new privileges and
  a restricted system-call set.
- The relay never logs payloads, tokens or passwords.
- The relay password is host-only. Friends receive short-lived invite tokens,
  not the password.
- Back up `relay.key` and `relay.json` if you want to rebuild the relay machine
  without changing its identity.

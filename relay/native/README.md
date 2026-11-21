# LAN Party relay: native Linux

This is the no-Docker install. It runs the same `lan-party-relay` binary as the
Docker image under a hardened systemd unit.

The relay is an opaque UDP forwarder. It cannot read the encrypted QUIC/TLS
traffic between the host and friends.

## Requirements

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
sudo install -m 0644 native/lan-party-relay.service /etc/systemd/system/lan-party-relay.service
sudo install -d -m 0700 /etc/lan-party-relay
```

Run setup. It asks for the public address. Use the hostname or IP that friends
will connect to, optionally with the port:

```sh
sudo lan-party-relay setup --dir /etc/lan-party-relay
```

Examples of the public address:

```text
relay.example.com
relay.example.com:7443
203.0.113.10
203.0.113.10:8443
```

By default the relay listens on `7443` even when the public address uses
another port, such as `relay.example.com:8443` behind a firewall redirect. To
bind the public port directly instead, pass `--listen`:

```sh
sudo lan-party-relay setup --dir /etc/lan-party-relay --listen :8443
```

Setup generates the relay identity key, the relay password and these files:

| File | Purpose | Keep private |
| --- | --- | --- |
| `/etc/lan-party-relay/relay.json` | relay config and password hash | yes |
| `/etc/lan-party-relay/relay.key` | relay identity key | yes |
| `/etc/lan-party-relay/relay-profile.json` | import file for the host app | yes, contains the password |

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

Follow the logs to see the fingerprint and any errors:

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

## Copy the host profile to the desktop

From the desktop that runs LAN Party, pull the profile over SSH:

```sh
scp root@relay.example.com:/etc/lan-party-relay/relay-profile.json ./relay-profile.json
chmod 600 ./relay-profile.json
```

If root SSH is disabled:

```sh
ssh relay.example.com 'sudo cat /etc/lan-party-relay/relay-profile.json' > relay-profile.json
chmod 600 relay-profile.json
```

Import the file in LAN Party under **Settings > Relay**, then use **Test relay**.
The app pins the relay fingerprint from the profile.

After importing, you can remove the profile from the relay. The password cannot
be recovered from `relay.json`; if you delete the profile, keep a copy in your
password manager or create a new relay setup.

```sh
sudo shred -u /etc/lan-party-relay/relay-profile.json
```

## Password and key rotation

To choose your own password before the first setup, pass it explicitly:

```sh
sudo lan-party-relay setup --dir /etc/lan-party-relay --password 'your password'
```

Change the password later without rotating the key:

```sh
sudo LANPARTY_RELAY_PASSWORD='new password' \
  lan-party-relay passwd --dir /etc/lan-party-relay
sudo systemctl restart lan-party-relay
sudo lan-party-relay show-profile --dir /etc/lan-party-relay
```

The fingerprint stays the same. Re-import the profile in the app so it gets the
new password.

The fingerprint is public and safe to share. A leaked fingerprint does not
require a reset. If the private `relay.key` is compromised, or you want a new
relay identity, rotate it with the service stopped:

```sh
sudo systemctl stop lan-party-relay
sudo lan-party-relay rotate-key --dir /etc/lan-party-relay
sudo systemctl start lan-party-relay
sudo lan-party-relay show-profile --dir /etc/lan-party-relay
```

This changes the fingerprint. Re-import the profile and issue new invites; old
pins and invites that carry the old fingerprint stop working.

## Updating

The config, key and fingerprint survive an update. Replace the binary and
restart:

```sh
sudo systemctl stop lan-party-relay
sudo install -m 0755 lan-party-relay /usr/local/bin/lan-party-relay
sudo systemctl start lan-party-relay
```

Do not regenerate the key during an update. A new key means the host app must
re-import the profile and re-pin the fingerprint, and existing invites stop
working.

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

**The service exits immediately.** Run the fingerprint command as the service
user to see the same check the unit runs:

```sh
sudo -u lan-party-relay lan-party-relay fingerprint --config /etc/lan-party-relay/relay.json
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

**The app says the fingerprint changed.** The relay key changed. Only re-pin if
you changed it intentionally. Otherwise treat it as a possible
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

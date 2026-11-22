# Using LAN Party

This is the longer version of the in-app guide. The app shows the same steps
with your current port and local address filled in.

## What you need

- The host: a Linux or Windows PC running the game server, with LAN Party
  installed.
- The friend: a Linux or Windows PC with LAN Party installed. Windows 10 or 11
  on x86-64, or Linux on x86-64 with X11 or Wayland.
- One UDP port forwarded on the host's router, or a self-hosted relay when a
  forward is not possible (for example behind carrier-grade NAT).
- A public address that reaches the host. A hostname is better than an IP
  address because it survives a changing home IP.

The host is the only person who needs a router change or a relay. Friends do
not forward anything.

On Windows, extract the portable zip and run `lan-party.cmd`. The first launch
may show a SmartScreen warning because the build is not signed. Allow LAN Party
on private networks when Windows Firewall asks. Settings live in
`%AppData%\lan-party`. Groups, keys and the log live in
`%LocalAppData%\lan-party`.

## Hosting

### 1. Add the game server as a service

Open the app, create a group, then choose **Add service**.

- **Name:** what the game is called, for example `Minecraft: Java Edition`.
- **Protocol:** TCP, UDP or both. Check the game's own documentation. Most
  survival games use UDP, Minecraft Java and Terraria use TCP.
- **Game server IP:** the IP of the machine running the server. New services
  default to `127.0.0.1`, which is right for a server or container on the same
  PC. `localhost` is accepted as an alias. For a server on another PC, use that
  PC's LAN IP. Names other than `localhost` are not accepted.
- **Game server port(s):** the port or ports the game listens on, for example
  `25565`, `2456-2458` or `2456,2600,2610`.
- **Friend port (optional):** the local port friends use. Leave it blank to use
  the same port(s) as the game server. Fill it in only when friends should use
  a different single port, for example a game on `25566` that friends reach on
  `25565`.
- **Hint:** what to paste the address into, for example
  `Direct Connect in Minecraft`.

Published services can be edited from their details. **Edit** reopens this form
with the current values.

A blank friend port maps every game port to the same number on the friend's PC.
That covers a range such as Valheim's `2456-2458` and a non-adjacent list such
as `2456,2600`. A different friend port only works with a single game port.
Multiple game ports with one friend port are rejected because the mapping would
be ambiguous. A service can publish up to eight ports.

### 2. Forward one UDP port

The app listens on one UDP port, by default `51820`. Forward that port on the
host's router to the host PC's LAN address:

```
Forward UDP 51820 to 192.168.1.20
```

The exact labels depend on the router. Look for **Port Forwarding**, **Virtual
Server** or **NAT**. The rule is:

- protocol: UDP
- external port: the app port (51820 unless you changed it)
- internal address: the host PC's LAN IP
- internal port: the same app port

Only this one port is opened. The game ports themselves are never opened on
the router or the firewall. The host app dials them from inside the LAN after
a friend is approved.

If your router forwards an external port that is different from the internal
one, put the external port in the **Public address** setting, for example
`home.example.net:40000`.

### 3. Set the public address

In **Settings → Network defaults**, enter the address friends should connect to. Usually
that is just a hostname or an IP:

- `desynkro.example.net`
- `203.0.113.10`, though a home IP usually changes

The app adds the app port automatically, so you do not repeat it here. Add a
port only when your router forwards a different outside port:

- `desynkro.example.net:40000` when outside 40000 is forwarded to the app port

The app does not call a “what is my IP” service. Use the address shown by your
router's WAN/status page, or a dynamic DNS name you control. A hostname is the
better choice because it survives an address change. The settings screen shows
the exact endpoint that will go into invites under the fields.

Changing these defaults cancels open invites for groups that use them. Create
new invites after the change. A group with its own connection keeps its invites
unless its effective endpoint also changes.

If the router's WAN address is different from the public address your ISP gives
you, you may be behind carrier-grade NAT. In that case a direct connection will
not work. Use the self-hosted relay described below instead of forwarding a
port.

### Host through a relay

A relay lets friends join when you cannot forward a port. The relay is a small
server you run on a VPS or spare machine. It forwards encrypted UDP packets and
cannot read the game traffic. Your host key and the friend's connection stay end
to end.

1. Follow [relay/README.md](../relay/README.md) to start the relay. Docker and
   native systemd installs both work. For a quick native trial without systemd,
   run `native/run.sh` from the relay zip: it runs in the console and stops when
   you close the terminal. Note the relay host, port (`7443` by default), Relay
   ID and password printed by the relay setup.
2. In **Settings → Network defaults**, tick **Use Relay**. Enter the relay host,
   relay port, Relay ID and password printed by `relayctl showinfo` on Docker
   or `lan-party-relay show-info` on a native install.
3. Choose **Test relay** to check the address, Relay ID pin and password.
4. Start the session and create an invite as usual. The relay needs only TCP and
   UDP on its one public port; the host PC needs no inbound port.
5. After the friend joins once, the host gives that device a reconnect token.
   Later drops reconnect automatically with the saved device key, including
   after the host ends and starts a session again. The friend does not need a
   new invite unless the host removes them or resets their reconnect access.

While no host session is active, the relay closes its UDP data port. Only the
TLS control port stays open so the host can wake it again. Starting a session
opens the data port before the host is accepted.

The relay is private infrastructure. Friends never see the relay password; an
invite carries a short-lived first-join token instead. Do not post the Relay ID.
The app has no **Share relay** action, and lending the relay to another host
means giving them the password, which is not supported in the UI.

If the relay changes its key, its Relay ID changes. Enter the new Relay ID and
create new invites. Old invites and reconnect tokens stop working. Use
`relayctl` on the relay host to change the password or rotate the key; see the
relay README.

### Forget inactive friends

In **Settings → Friends**, set how many days a friend can go without connecting
before their member record and reconnect access are removed. This card only
appears when a host group uses a relay, and only relay groups are cleaned up.
The saved relay reconnect token is long-lived, so the window limits how long a
lost or abandoned device can keep reconnecting.

New installs start at 5 days. An existing settings file starts blank (never) so
an upgrade cannot forget anyone before the host chooses a window. Leave the
field blank to keep friends forever. Friends who are currently connected are
never removed.

A friend who is removed needs a new invite and host approval. The member details
show when they last connected and when they will be forgotten.

### Use a different address or port for one group

Settings holds the defaults for every group. A host group can override them for
itself:

1. Open the group and click **Change** next to the endpoint.
2. Tick **Override Settings**.
3. Choose **Use Relay** or direct mode, then enter the group's own values.
   Leave direct fields blank to use the Settings defaults. A custom relay can
   use its own host, port, Relay ID and password.
4. Choose **Reset** to clear every group connection override and return to
   Settings. Reset is a form action: choose **Save** to commit it.

The card shows **Default** or **Custom** next to the endpoint, plus a **Relay**
tag when the group uses one. The app port is the UDP port on this PC when direct
mode is active. Changing the connection while a session is live restarts it;
everyone disconnects briefly.

Changing the app port while the session is live restarts it. Everyone
disconnects briefly. If the advertised endpoint changes, open invites are
cancelled. Create new invites after the restart. If it stays the same, update
your router forward to the new local port.

### 4. Send an invite

Click **Invite**, enter the friend's name, and copy the code. If the session is
off, the dialog starts it before creating the code. If another group's session
is live, end that session first.

The code is a single-use invite. It expires after 1 hour. Send it privately.
Anyone who has it can try to redeem it, and the approval card is your chance to
deny a stranger.

### 5. Approve the device

When your friend redeems the code, an approval card appears on your screen:

- their name, the name you put on the invite
- the device name their app reported
- the address the host actually sees
- a short device-key fingerprint

Allow it if you recognise the device. If you do not, deny it and create a new
invite. You can remove a device later from the People section.

## Joining

1. Open LAN Party and choose **Join with an invite**.
2. Paste the code. The dialog shows the name the host assigned you and the group
   and host name. Your device name is detected automatically and sent to the
   host. It is read-only here.
3. Wait while the host approves the device. The host has to be online and
   running the session.
4. Open the game and go to the address shown under **Ready to play**, usually
   `127.0.0.1:25565`.
5. Use the host's hint. For Minecraft Java, that is usually **Direct Connect**.

If the local port is already in use, the app tries the next port and shows the
address it actually got. The host's published port does not change.

The group stays saved. If the host goes offline, the screen says so and retries
automatically for up to 30 minutes. **Reconnect** tries immediately and resets
that timer. Turn the retries off in **Settings → Connection**. **Disconnect**
ends the session but keeps the group. **Leave group** removes the saved group
and its device key from your PC.

## Window and tray

By default, closing the window keeps LAN Party and a live session running in
the system tray. The tray menu can show the window, open the guide or settings,
and quit. On Windows the icon lives in the notification area. Check the overflow
menu if it is hidden. On Linux the desktop needs a StatusNotifier host. GNOME
users can install the AppIndicator extension.

If you untick **Close to tray** in Settings, closing the window quits the app
and ends any live session. If your desktop does not report a system tray, that
is what closing does anyway. The setting says so.

## Invites

- Single use. The token is burned after a successful join.
- Expire after 1 hour.
- The host must be online when the friend redeems the code.
- The host stores only a hash, so a code cannot be shown again after the app
  restarts. Create a new invite for the same friend if you need to send it
  again.
- A relay friend gets a separate reconnect token after the first join. It is
  stored in the owner-only secret store and lets them reconnect without another
  invite until the host removes the member, resets their access, or forgets
  them as inactive.
- A denied or timed-out direct join leaves the invite usable until it
  expires. A relay invite binds its token to the friend's first source address,
  so a failed first relay connection needs a new invite.
- **Cancel invite** in the host's group screen stops a code immediately.

## Device changes

The device key is the membership. If a friend reinstalls the app or uses a new
PC, the key changes. The host sees a **new device** request and must allow it.
The old key is replaced. The old connection is closed.

Changing a device name or a public IP is shown as information, not used to
kick anyone. IP addresses change for ordinary reasons. The key is the strong
signal.

## Problems

### Windows shows a SmartScreen or Defender warning

The Windows build is not code-signed. SmartScreen may ask you to confirm it.
Choose **More info** and **Run anyway** only after checking the SHA-256 against
the release's `SHA256SUMS`.

Windows Defender can occasionally false-positive an unsigned new build. Do not
disable Defender. Submit the zip through Microsoft's
[false-positive form](https://www.microsoft.com/en-us/wdsi/filesubmission) and
send the log. Code signing is planned after 1.1.

### A friend cannot reach the host

Check, in this order:

1. The public address in Settings matches what your router shows.
2. The router forwards UDP **app port** to the host PC's current LAN IP.
3. The host PC's firewall allows the app to receive UDP. On Linux, a firewall
   may be blocking the port even when the router is correct. On Windows, allow
   LAN Party on private networks when the firewall prompt appears.
4. A VPN on the host is not hiding the public address. Try excluding LAN Party
   from the VPN if the client supports per-app exclusions.
5. The host is behind carrier-grade NAT. Compare the router's WAN address with
   the address your ISP shows. If they differ, use the self-hosted relay and
   check **Test relay** in Settings.

### The relay went down

The host app retries an unexpectedly lost relay session for about 16 minutes:
every 5 seconds for the first minute, every 10 seconds for the next 5 minutes,
then every 30 seconds for the next 10. If the relay comes back, the session
restarts and member reconnect tokens are registered again. Friends retry on the
normal native schedule and reconnect without a new invite.

If automatic recovery stops, start the session manually once the relay is back.
The app does not silently switch to direct mode.

### The public address changed

Update it in **Settings → Network defaults**, or in the group's own connection
dialog. Open invites that used the old endpoint are cancelled. Create new
invites after the change.

### A friend's port is already in use

The friend app tries the next ten loopback ports and shows the address that
worked. The host's published port is unchanged. If every attempt fails, the
service card shows the error.

### An invite expired or was already used

Create a new invite. Codes work once and expire after 1 hour.

### Sending a log

LAN Party keeps a log in its data directory:

- Linux: `~/.local/share/lan-party/lan-party.log`
- Windows: `%LocalAppData%\lan-party\lan-party.log`

The log rotates at 5 MB and keeps one previous file. It never contains invite
codes, tokens or private keys. Send it when reporting a startup or connection
problem.

### Where are my keys stored?

On Linux the app keeps private keys in `~/.local/share/lan-party/keys/`. On
Windows they live in `%LocalAppData%\lan-party\keys\`. Each key is an owner-only
file in an owner-only directory, so only your user account can read it. There
is no keyring, session bus or unlock prompt.

The keys are not passphrase-protected. Keep the folder private and use full-disk
encryption if the disk could be stolen.

### A group says its key is missing

That group came from an older build that kept keys in the desktop keyring. This
version does not import them and will not silently make a new key. Delete the
group and create it again, then send a new invite.

### The app port is not 51820

That is allowed. Set **App port** in **Settings → Network defaults**, or set a
group-specific app port on the group screen. Forward that UDP port on the
router. If the app port changes while a host session is live, the group dialog
stops the session, saves, and restarts it after confirmation. Everyone
disconnects briefly. Update the router forward and, if the advertised endpoint
changed, send new invites.

## Security notes

- Only the services you publish exist. Friends cannot reach your router, your
  NAS, your printer or each other.
- The host approves each new device. A changed device key asks again.
- Invite codes work once and expire.
- Do not post invite codes in public. Send them privately.
- Removing a member deletes the key and closes the connection. To let them
  back, create a new invite and approve the new device.
- The game server's own security is still your responsibility. Use its password
  or whitelist, especially for an offline-mode server.

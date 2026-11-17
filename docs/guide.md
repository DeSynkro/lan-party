# Using LAN Party

This is the longer version of the in-app guide. The app shows the same steps
with your current port and local address filled in.

## What you need

- The host: a Linux PC running the game server, with LAN Party installed.
- The friend: a Linux PC with LAN Party installed.
- One UDP port forwarded on the host's router.
- A public address that reaches the host. A hostname is better than an IP
  address because it survives a changing home IP.

The host is the only person who needs a router change. Friends do not forward
anything.

## Hosting

### 1. Add the game server as a service

Open the app, create a group, then choose **Add service**.

- **Name:** what the game is called, for example `Minecraft: Java Edition`.
- **Protocol:** TCP, UDP or both. Check the game's own documentation; most
  survival games use UDP, Minecraft Java and Terraria use TCP.
- **On my LAN:** the address the server listens on, for example
  `192.168.1.50:25565`. A name is not accepted; use the IP. If the server runs
  on the same PC, `127.0.0.1:25565` works.
- **Friends open:** the port the friend will use, usually the same number.
  You can publish a different port for a single-port service, for example a
  server on `25566` that friends reach on `25565`.
- **Hint:** what to paste the address into, for example
  `Direct Connect in Minecraft`.

A bare IP with several ports publishes those same port numbers. That is how a
game with a range such as Valheim's `2456-2458` is added. An `IP:port` target
with several published ports is rejected because the mapping would be
ambiguous.

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

In **Settings → Network**, enter the address friends should connect to. Usually
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

If the router's WAN address is different from the public address your ISP gives
you, you may be behind carrier-grade NAT. In that case a direct connection will
not work. This version has no relay, so there is no workaround inside the app.
Try a different connection or a public host.

### 4. Start the session and send an invite

Click **Start session**. Then click **Invite**, enter the friend's name, and
copy the code.

The code is a single-use invite. It expires after 24 hours. Send it privately.
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
2. Paste the code and leave the device name as it is, or change it to something
   the host will recognise.
3. Wait while the host approves the device. The host has to be online and
   running the session.
4. Open the game and go to the address shown under **Ready to play**, usually
   `127.0.0.1:25565`.
5. Use the host's hint. For Minecraft Java, that is usually **Direct Connect**.

If the local port is already in use, the app tries the next port and shows the
address it actually got. The host's published port does not change.

The group stays saved. If the host goes offline, the screen says so and offers
Reconnect. **Disconnect** ends the session but keeps the group. **Leave group**
removes the saved group and its device key from your PC.

## Window and tray

By default, closing the window keeps LAN Party and a live session running in
the system tray. The tray menu can show the window, open the guide or settings,
and quit.

If you untick **Close to tray** in Settings, closing the window quits the app
and ends any live session. If your desktop does not report a system tray, that
is what closing does anyway; the setting says so.

## Invites

- Single use. The token is burned after a successful join.
- Expire after 24 hours.
- The host must be online when the friend redeems the code.
- The host stores only a hash, so a code cannot be shown again after the app
  restarts. Create a new invite for the same friend if you need to send it
  again.
- A denied or timed-out join leaves the invite usable until it expires.
- **Cancel invite** in the host's group screen stops a code immediately.

## Device changes

The device key is the membership. If a friend reinstalls the app or uses a new
PC, the key changes. The host sees a **new device** request and must allow it.
The old key is replaced; the old connection is closed.

Changing a device name or a public IP is shown as information, not used to
kick anyone. IP addresses change for ordinary reasons. The key is the strong
signal.

## Problems

### A friend cannot reach the host

Check, in this order:

1. The public address in Settings matches what your router shows.
2. The router forwards UDP **app port** to the host PC's current LAN IP.
3. The host PC's firewall allows the app to receive UDP. On Linux, a firewall
   may be blocking the port even when the router is correct.
4. A VPN on the host is not hiding the public address. Try excluding LAN Party
   from the VPN if the client supports per-app exclusions.
5. The host is behind carrier-grade NAT. Compare the router's WAN address with
   the address your ISP shows. If they differ, direct hosting will not work
   without a relay, which this version does not have.

### The public address changed

Update it in **Settings → Network**. Old invites still point at the old
address; create new invites after the change.

### A friend's port is already in use

The friend app tries the next ten loopback ports and shows the address that
worked. The host's published port is unchanged. If every attempt fails, the
service card shows the error.

### An invite expired or was already used

Create a new invite. Codes work once and expire after 24 hours.

### The app says the keyring is unavailable

The app keeps private keys in the desktop keyring. On GNOME that is GNOME
Keyring; on KDE it is usually KWallet with the Secret Service interface
enabled. Make sure the keyring is unlocked and the session bus is available.
The app does not fall back to a plaintext key file.

### The app port is not 51820

That is allowed. Set **App port** in Settings and forward that UDP port on the
router. If the app port changes while a host session is live, end the session
first.

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

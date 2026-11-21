package session

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/desynkro/lan-party/internal/group"
	"github.com/desynkro/lan-party/internal/invite"
	"github.com/desynkro/lan-party/internal/relay"
	"github.com/desynkro/lan-party/internal/secure"
	"github.com/desynkro/lan-party/internal/tunnel"
)

type testStore map[string]string

func (m testStore) slot(service, user string) string { return service + "\x00" + user }

func (m testStore) Get(service, user string) (string, error) {
	value, ok := m[m.slot(service, user)]
	if !ok {
		return "", fs.ErrNotExist
	}
	return value, nil
}

func (m testStore) Set(service, user, secret string) error {
	m[m.slot(service, user)] = secret
	return nil
}

func (m testStore) Delete(service, user string) error {
	delete(m, m.slot(service, user))
	return nil
}

func testIdentity(t *testing.T, name string) secure.Identity {
	t.Helper()
	identity, err := secure.NewStore(testStore{}).LoadOrCreate(name)
	if err != nil {
		t.Fatal(err)
	}
	return identity
}

type testAuth func(context.Context, tunnel.AuthRequest) (tunnel.AuthDecision, error)

func (f testAuth) Authenticate(ctx context.Context, req tunnel.AuthRequest) (tunnel.AuthDecision, error) {
	return f(ctx, req)
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func freeUDPPort(t *testing.T) int {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).Port
}

func tcpEcho(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				_, _ = io.Copy(conn, conn)
			}(conn)
		}
	}()
	return ln.Addr().String()
}

func udpEcho(t *testing.T) string {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	go func() {
		buf := make([]byte, 65535)
		for {
			n, addr, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}
			_, _ = conn.WriteTo(buf[:n], addr)
		}
	}()
	return conn.LocalAddr().String()
}

func makeInvite(t *testing.T, hostID secure.Identity, appPort int) (invite.Invite, string) {
	t.Helper()
	token, _, err := invite.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	inv := invite.Invite{
		Version:   invite.CurrentVersion,
		GroupID:   "group-1",
		GroupName: "Friday Night",
		HostName:  "Desynkro",
		Endpoint:  net.JoinHostPort("127.0.0.1", strconv.Itoa(appPort)),
		HostPin:   secure.Pin(hostID.Public),
		Token:     token,
		PeerName:  "Ash",
		Expires:   time.Now().Add(time.Hour),
	}
	return inv, token
}

func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition did not become true in time")
}

func TestHostFriendTCPAndUDP(t *testing.T) {
	tcpTarget := tcpEcho(t)
	udpTarget := udpEcho(t)
	pubTCP := freeTCPPort(t)
	pubUDP := freeUDPPort(t)
	services := []group.Service{
		{ID: "tcp-1", Name: "TCP echo", Proto: "tcp", Target: tcpTarget, Ports: []int{pubTCP}},
		{ID: "udp-1", Name: "UDP echo", Proto: "udp", Target: udpTarget, Ports: []int{pubUDP}},
	}

	hostID := testIdentity(t, "host")
	appPort := freeUDPPort(t)
	auth := testAuth(func(_ context.Context, req tunnel.AuthRequest) (tunnel.AuthDecision, error) {
		if len(req.PublicKey) == 0 || req.Fingerprint == "" {
			t.Error("the host did not receive the device key")
		}
		return tunnel.AuthDecision{Allow: true, MemberID: "member-1", Services: ServiceInfos(services)}, nil
	})

	host, err := StartHost(context.Background(), HostConfig{
		Identity:     hostID,
		GroupID:      "group-1",
		AppPort:      appPort,
		Services:     services,
		Authenticate: auth,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = host.Close() })

	inv, token := makeInvite(t, hostID, appPort)
	friend, err := Join(context.Background(), JoinConfig{
		Identity:   testIdentity(t, "device"),
		Invite:     inv,
		DeviceName: "ash-desktop",
		Token:      token,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = friend.Close() })

	var tcpBound, udpBound Bound
	for _, b := range friend.Bound() {
		if b.Error != "" {
			t.Fatalf("%s: %s", b.Service, b.Error)
		}
		switch b.Proto {
		case "tcp":
			tcpBound = b
		case "udp":
			udpBound = b
		}
	}
	if tcpBound.Address == "" || udpBound.Address == "" {
		t.Fatalf("bounds = %+v", friend.Bound())
	}
	if !friend.Presence().Live {
		t.Fatal("the friend does not see the host as live")
	}
	waitFor(t, 2*time.Second, func() bool {
		p, ok := host.Presence()["member-1"]
		return ok && p.Live
	})

	tcpConn, err := net.Dial("tcp", tcpBound.Address)
	if err != nil {
		t.Fatal(err)
	}
	defer tcpConn.Close()
	_ = tcpConn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := tcpConn.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, 5)
	if _, err := io.ReadFull(tcpConn, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("tcp echo = %q", got)
	}

	udpAddr, err := net.ResolveUDPAddr("udp", udpBound.Address)
	if err != nil {
		t.Fatal(err)
	}
	udpConn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer udpConn.Close()
	_ = udpConn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := udpConn.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 32)
	n, err := udpConn.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf[:n]) != "ping" {
		t.Fatalf("udp echo = %q", buf[:n])
	}
}

func TestRevoke(t *testing.T) {
	target := tcpEcho(t)
	port := freeTCPPort(t)
	services := []group.Service{{ID: "tcp-1", Name: "TCP echo", Proto: "tcp", Target: target, Ports: []int{port}}}
	hostID := testIdentity(t, "host")
	appPort := freeUDPPort(t)
	auth := testAuth(func(context.Context, tunnel.AuthRequest) (tunnel.AuthDecision, error) {
		return tunnel.AuthDecision{Allow: true, MemberID: "member-1", Services: ServiceInfos(services)}, nil
	})

	host, err := StartHost(context.Background(), HostConfig{Identity: hostID, GroupID: "group-1", AppPort: appPort, Services: services, Authenticate: auth})
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()

	inv, token := makeInvite(t, hostID, appPort)
	friend, err := Join(context.Background(), JoinConfig{Identity: testIdentity(t, "device"), Invite: inv, DeviceName: "ash-desktop", Token: token})
	if err != nil {
		t.Fatal(err)
	}
	defer friend.Close()

	waitFor(t, 2*time.Second, func() bool {
		p, ok := host.Presence()["member-1"]
		return ok && p.Live
	})
	if err := host.Revoke("member-1"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 2*time.Second, func() bool {
		return !friend.Presence().Live
	})
	if friend.Note() == "" {
		t.Fatal("the friend has no note explaining the disconnect")
	}
}

func TestServicesUpdate(t *testing.T) {
	tcpTarget := tcpEcho(t)
	udpTarget := udpEcho(t)
	pubTCP := freeTCPPort(t)
	pubUDP := freeUDPPort(t)
	tcpService := group.Service{ID: "tcp-1", Name: "TCP echo", Proto: "tcp", Target: tcpTarget, Ports: []int{pubTCP}}
	udpService := group.Service{ID: "udp-1", Name: "UDP echo", Proto: "udp", Target: udpTarget, Ports: []int{pubUDP}}

	hostID := testIdentity(t, "host")
	appPort := freeUDPPort(t)
	auth := testAuth(func(context.Context, tunnel.AuthRequest) (tunnel.AuthDecision, error) {
		return tunnel.AuthDecision{Allow: true, MemberID: "member-1", Services: ServiceInfos([]group.Service{tcpService, udpService})}, nil
	})
	host, err := StartHost(context.Background(), HostConfig{
		Identity:     hostID,
		GroupID:      "group-1",
		AppPort:      appPort,
		Services:     []group.Service{tcpService, udpService},
		Authenticate: auth,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()

	inv, token := makeInvite(t, hostID, appPort)
	friend, err := Join(context.Background(), JoinConfig{Identity: testIdentity(t, "device"), Invite: inv, DeviceName: "ash-desktop", Token: token})
	if err != nil {
		t.Fatal(err)
	}
	defer friend.Close()

	waitFor(t, 2*time.Second, func() bool {
		p, ok := host.Presence()["member-1"]
		return ok && p.Live
	})
	if err := host.UpdateServices([]group.Service{udpService}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 2*time.Second, func() bool {
		bounds := friend.Bound()
		if len(bounds) != 1 {
			return false
		}
		return bounds[0].Proto == "udp" && bounds[0].Error == ""
	})
}

func TestJoinDenied(t *testing.T) {
	hostID := testIdentity(t, "host")
	appPort := freeUDPPort(t)
	auth := testAuth(func(context.Context, tunnel.AuthRequest) (tunnel.AuthDecision, error) {
		return tunnel.AuthDecision{Allow: false, Reason: "not on the list"}, nil
	})
	host, err := StartHost(context.Background(), HostConfig{Identity: hostID, GroupID: "group-1", AppPort: appPort, Authenticate: auth})
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()

	inv, token := makeInvite(t, hostID, appPort)
	_, err = Join(context.Background(), JoinConfig{Identity: testIdentity(t, "device"), Invite: inv, DeviceName: "ash-desktop", Token: token})
	if !errors.Is(err, tunnel.ErrDenied) {
		t.Fatalf("error = %v", err)
	}
}

func TestJoinExpiredInvite(t *testing.T) {
	hostID := testIdentity(t, "host")
	inv, token := makeInvite(t, hostID, 51820)
	inv.Expires = time.Now().Add(-time.Hour)
	if _, err := Join(context.Background(), JoinConfig{Identity: testIdentity(t, "device"), Invite: inv, DeviceName: "ash-desktop", Token: token}); err == nil {
		t.Fatal("an expired invite was accepted")
	}
}

func TestResume(t *testing.T) {
	target := tcpEcho(t)
	port := freeTCPPort(t)
	services := []group.Service{{ID: "tcp-1", Name: "TCP echo", Proto: "tcp", Target: target, Ports: []int{port}}}
	hostID := testIdentity(t, "host")
	appPort := freeUDPPort(t)
	deviceID := testIdentity(t, "device")

	var reqMu sync.Mutex
	var requests []tunnel.AuthRequest
	auth := testAuth(func(_ context.Context, req tunnel.AuthRequest) (tunnel.AuthDecision, error) {
		reqMu.Lock()
		requests = append(requests, req)
		reqMu.Unlock()
		return tunnel.AuthDecision{Allow: true, MemberID: "member-1", Services: ServiceInfos(services)}, nil
	})
	host, err := StartHost(context.Background(), HostConfig{Identity: hostID, GroupID: "group-1", AppPort: appPort, Services: services, Authenticate: auth})
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()

	inv, token := makeInvite(t, hostID, appPort)
	first, err := Join(context.Background(), JoinConfig{Identity: deviceID, Invite: inv, DeviceName: "ash-desktop", Token: token})
	if err != nil {
		t.Fatal(err)
	}
	firstID := first.MemberID
	if firstID == "" {
		t.Fatal("the first join has no member id")
	}
	_ = first.Close()
	waitFor(t, 2*time.Second, func() bool {
		_, ok := host.Presence()[firstID]
		return !ok
	})

	inv.Expires = time.Now().Add(-time.Hour)
	second, err := Join(context.Background(), JoinConfig{
		Identity:   deviceID,
		Invite:     inv,
		DeviceName: "ash-desktop",
		MemberID:   firstID,
		Token:      "",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if second.MemberID != firstID {
		t.Fatalf("member id changed: %q -> %q", firstID, second.MemberID)
	}
	reqMu.Lock()
	gotRequests := append([]tunnel.AuthRequest(nil), requests...)
	reqMu.Unlock()
	if len(gotRequests) < 2 {
		t.Fatalf("the authenticator saw %d requests", len(gotRequests))
	}
	if gotRequests[1].MemberID != firstID {
		t.Fatalf("resume member id = %q", gotRequests[1].MemberID)
	}
	if !gotRequests[1].PublicKey.Equal(deviceID.Public) {
		t.Fatal("resume presented a different device key")
	}
}

func TestHostPortInUse(t *testing.T) {
	// Hold the same wildcard address the host binds. Windows allows a
	// specific-address socket to coexist with a wildcard bind; Linux does not.
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	port := conn.LocalAddr().(*net.UDPAddr).Port
	hostID := testIdentity(t, "host")
	auth := testAuth(func(context.Context, tunnel.AuthRequest) (tunnel.AuthDecision, error) {
		return tunnel.AuthDecision{Allow: true, MemberID: "member-1"}, nil
	})
	if _, err := StartHost(context.Background(), HostConfig{Identity: hostID, GroupID: "group-1", AppPort: port, Authenticate: auth}); err == nil {
		t.Fatal("a busy app port was accepted")
	}
}

func TestHostClose(t *testing.T) {
	target := tcpEcho(t)
	port := freeTCPPort(t)
	services := []group.Service{{ID: "tcp-1", Name: "TCP echo", Proto: "tcp", Target: target, Ports: []int{port}}}
	hostID := testIdentity(t, "host")
	appPort := freeUDPPort(t)
	auth := testAuth(func(context.Context, tunnel.AuthRequest) (tunnel.AuthDecision, error) {
		return tunnel.AuthDecision{Allow: true, MemberID: "member-1", Services: ServiceInfos(services)}, nil
	})
	host, err := StartHost(context.Background(), HostConfig{Identity: hostID, GroupID: "group-1", AppPort: appPort, Services: services, Authenticate: auth})
	if err != nil {
		t.Fatal(err)
	}
	inv, token := makeInvite(t, hostID, appPort)
	friend, err := Join(context.Background(), JoinConfig{Identity: testIdentity(t, "device"), Invite: inv, DeviceName: "ash-desktop", Token: token})
	if err != nil {
		t.Fatal(err)
	}
	defer friend.Close()

	waitFor(t, 2*time.Second, func() bool { return friend.Presence().Live })
	if err := host.Close(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 2*time.Second, func() bool { return !friend.Presence().Live })
}

func freeRelayPort(t *testing.T) int {
	t.Helper()
	for attempt := 0; attempt < 20; attempt++ {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port := ln.Addr().(*net.TCPAddr).Port
		_ = ln.Close()
		udp, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
		if err == nil {
			_ = udp.Close()
			return port
		}
	}
	t.Fatal("could not find a free relay port")
	return 0
}

func TestHostFriendThroughRelay(t *testing.T) {
	const relayPassword = "relay test password"
	port := freeRelayPort(t)
	dir := t.TempDir()
	profile, err := relay.Setup(relay.SetupOptions{
		Dir:           dir,
		PublicAddress: net.JoinHostPort("127.0.0.1", strconv.Itoa(port)),
		Listen:        net.JoinHostPort("127.0.0.1", strconv.Itoa(port)),
		Password:      relayPassword,
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := relay.LoadConfig(relay.ConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	cert, fingerprint, err := relay.LoadIdentity(relay.KeyPath(relay.ConfigPath(dir), cfg))
	if err != nil {
		t.Fatal(err)
	}
	relayServer, err := relay.NewServer(cfg, cert, fingerprint)
	if err != nil {
		t.Fatal(err)
	}
	if err := relayServer.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = relayServer.Close() })

	hostID := testIdentity(t, "relay-host")
	appPort := freeUDPPort(t)
	auth := testAuth(func(context.Context, tunnel.AuthRequest) (tunnel.AuthDecision, error) {
		return tunnel.AuthDecision{Allow: true, MemberID: "member-1"}, nil
	})
	host, err := StartHost(context.Background(), HostConfig{
		Identity:     hostID,
		GroupID:      "relay-group",
		AppPort:      appPort,
		Authenticate: auth,
		Relay: &RelayHostConfig{
			Address:     profile.Address,
			Fingerprint: profile.Fingerprint,
			Password:    relayPassword,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = host.Close() })

	hostToken, _, err := invite.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	_, relayToken, err := relay.NewFriendToken()
	if err != nil {
		t.Fatal(err)
	}
	expires := time.Now().Add(time.Hour)
	if err := host.AuthorizeRelayToken(context.Background(), relayToken, expires); err != nil {
		t.Fatal(err)
	}
	inv := invite.Invite{
		Version:    invite.RelayVersion,
		GroupID:    "relay-group",
		GroupName:  "Relay Group",
		HostName:   "Host",
		HostPin:    secure.Pin(hostID.Public),
		Token:      hostToken,
		RelayHost:  "127.0.0.1",
		RelayPort:  port,
		RelayID:    profile.Fingerprint,
		RelayToken: relayToken,
		PeerName:   "Ash",
		Expires:    expires,
	}
	friend, err := Join(context.Background(), JoinConfig{
		Identity:   testIdentity(t, "relay-device"),
		Invite:     inv,
		DeviceName: "ash-desktop",
		Token:      hostToken,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer friend.Close()

	if !friend.Presence().Live {
		t.Fatal("the friend does not see the relayed host as live")
	}
	waitFor(t, 3*time.Second, func() bool {
		presence, ok := host.Presence()["member-1"]
		return ok && presence.Live
	})
}

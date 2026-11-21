package relay

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

const testRelayPassword = "relay test password"

type testRelay struct {
	server      *Server
	address     string
	fingerprint string
}

func startTestRelay(t *testing.T, adjust func(*Server)) *testRelay {
	t.Helper()
	dir := t.TempDir()
	cert, fingerprint, err := GenerateIdentity(filepath.Join(dir, KeyFileName))
	if err != nil {
		t.Fatal(err)
	}
	salt, hash, err := HashPassword(testRelayPassword)
	if err != nil {
		t.Fatal(err)
	}
	port := freePort(t)
	cfg := Config{
		Version:       Version,
		Listen:        net.JoinHostPort("127.0.0.1", strconv.Itoa(port)),
		PublicAddress: net.JoinHostPort("relay.test", strconv.Itoa(port)),
		KeyFile:       KeyFileName,
		PasswordSalt:  salt,
		PasswordHash:  hash,
		MaxSessions:   8,
		MaxFriends:    8,
	}
	server, err := NewServer(cfg, cert, fingerprint)
	if err != nil {
		t.Fatal(err)
	}
	if adjust != nil {
		adjust(server)
	}
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	return &testRelay{server: server, address: cfg.Listen, fingerprint: fingerprint}
}

func freePort(t *testing.T) int {
	t.Helper()
	for attempt := 0; attempt < 20; attempt++ {
		tcp, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port := tcp.Addr().(*net.TCPAddr).Port
		_ = tcp.Close()
		udp, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
		if err == nil {
			_ = udp.Close()
			return port
		}
	}
	t.Fatal("could not find a free TCP and UDP port")
	return 0
}

type testHost struct {
	conn      *tls.Conn
	reader    *bufio.Reader
	sessionID string
	dataToken []byte
}

func openTestHost(t *testing.T, relay *testRelay, password string) (*tls.Conn, *bufio.Reader) {
	t.Helper()
	conn, err := tls.Dial("tcp", relay.address, ClientTLS(relay.fingerprint))
	if err != nil {
		t.Fatalf("connect host control: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	reader := bufio.NewReader(conn)
	if err := writeControl(conn, controlMessage{
		Type:     "hello",
		Version:  Version,
		Role:     "host",
		Password: password,
	}); err != nil {
		t.Fatalf("send hello: %v", err)
	}
	return conn, reader
}

func dialTestHost(t *testing.T, relay *testRelay, password string) *testHost {
	t.Helper()
	conn, reader := openTestHost(t, relay, password)
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	msg, err := readControl(reader)
	if err != nil {
		t.Fatalf("read welcome: %v", err)
	}
	if msg.Type != "welcome" {
		t.Fatalf("welcome = %+v", msg)
	}
	token, err := base64.RawURLEncoding.DecodeString(msg.DataToken)
	if err != nil || len(token) != tokenLength {
		t.Fatalf("welcome data token = %q", msg.DataToken)
	}
	_ = conn.SetReadDeadline(time.Time{})
	return &testHost{conn: conn, reader: reader, sessionID: msg.SessionID, dataToken: token}
}

func (h *testHost) send(t *testing.T, msg controlMessage) {
	t.Helper()
	if err := writeControl(h.conn, msg); err != nil {
		t.Fatalf("send control: %v", err)
	}
}

func (h *testHost) read(t *testing.T) controlMessage {
	t.Helper()
	_ = h.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	msg, err := readControl(h.reader)
	if err != nil {
		t.Fatalf("read control: %v", err)
	}
	return msg
}

func (h *testHost) expect(t *testing.T, typ string) controlMessage {
	t.Helper()
	msg := h.read(t)
	if msg.Type != typ {
		t.Fatalf("control type = %q (%+v), want %q", msg.Type, msg, typ)
	}
	return msg
}

func (h *testHost) expectNone(t *testing.T, wait time.Duration) {
	t.Helper()
	_ = h.conn.SetReadDeadline(time.Now().Add(wait))
	_, err := readControl(h.reader)
	if err == nil {
		t.Fatal("relay sent an unexpected control message")
	}
	if !isTimeout(err) {
		t.Fatalf("read control: %v", err)
	}
	_ = h.conn.SetReadDeadline(time.Time{})
}

func (h *testHost) authorize(t *testing.T, token string) {
	t.Helper()
	h.send(t, controlMessage{Type: "authorize", Token: token})
	h.expect(t, "ok")
}

func newTestFriend(t *testing.T, relay *testRelay) *net.UDPConn {
	t.Helper()
	addr, err := net.ResolveUDPAddr("udp", relay.address)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func newTestHostData(t *testing.T) *net.UDPConn {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func relayDataAddr(t *testing.T, relay *testRelay) *net.UDPAddr {
	t.Helper()
	addr, err := net.ResolveUDPAddr("udp", relay.address)
	if err != nil {
		t.Fatal(err)
	}
	return addr
}

func sendFriend(t *testing.T, conn *net.UDPConn, tokenText string, payload []byte) {
	t.Helper()
	token, err := DecodeFriendToken(tokenText)
	if err != nil {
		t.Fatalf("decode friend token: %v", err)
	}
	packet := make([]byte, 0, 1+len(token)+len(payload))
	packet = append(packet, packetFriend)
	packet = append(packet, token...)
	packet = append(packet, payload...)
	if _, err := conn.Write(packet); err != nil {
		t.Fatalf("send friend packet: %v", err)
	}
}

func readFriend(conn *net.UDPConn, wait time.Duration) ([]byte, error) {
	_ = conn.SetReadDeadline(time.Now().Add(wait))
	buf := make([]byte, maxRelayPayload)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), buf[:n]...), nil
}

func expectNoFriendPacket(t *testing.T, conn *net.UDPConn, wait time.Duration) {
	t.Helper()
	if payload, err := readFriend(conn, wait); err == nil {
		t.Fatalf("friend received an unexpected packet %q", payload)
	} else if !isTimeout(err) && !errors.Is(err, syscall.ECONNREFUSED) {
		t.Fatalf("read friend: %v", err)
	}
}

func hostKeepalive(t *testing.T, hostData *net.UDPConn, relay *testRelay, dataToken []byte) {
	t.Helper()
	packet := append([]byte{packetHost}, dataToken...)
	if _, err := hostData.WriteToUDP(packet, relayDataAddr(t, relay)); err != nil {
		t.Fatalf("send host keepalive: %v", err)
	}
}

func hostReply(t *testing.T, hostData *net.UDPConn, relay *testRelay, dataToken []byte, friendID uint16, payload []byte) {
	t.Helper()
	packet := make([]byte, 0, 1+len(dataToken)+friendIDLength+len(payload))
	packet = append(packet, packetHost)
	packet = append(packet, dataToken...)
	var id [friendIDLength]byte
	binary.BigEndian.PutUint16(id[:], friendID)
	packet = append(packet, id[:]...)
	packet = append(packet, payload...)
	if _, err := hostData.WriteToUDP(packet, relayDataAddr(t, relay)); err != nil {
		t.Fatalf("send host reply: %v", err)
	}
}

func readHostData(t *testing.T, hostData *net.UDPConn) (uint16, []byte, *net.UDPAddr) {
	t.Helper()
	_ = hostData.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, maxDatagramSize)
	n, addr, err := hostData.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("read host data: %v", err)
	}
	if n < 1+friendIDLength || buf[0] != packetRelay {
		t.Fatalf("host data missing relay header: %q", buf[:n])
	}
	id := binary.BigEndian.Uint16(buf[1 : 1+friendIDLength])
	payload := append([]byte(nil), buf[1+friendIDLength:n]...)
	return id, payload, addr
}

func joinFriend(t *testing.T, relay *testRelay, host *testHost, hostData *net.UDPConn, friend *net.UDPConn, token string, payload []byte) (uint16, *net.UDPAddr) {
	t.Helper()
	sendFriend(t, friend, token, payload)
	note := host.expect(t, "friend")
	if note.FriendID == 0 {
		t.Fatalf("friend notification has no id: %+v", note)
	}
	hostKeepalive(t, hostData, relay, host.dataToken)
	id, got, from := readHostData(t, hostData)
	if id != note.FriendID {
		t.Fatalf("host saw friend id %d, want %d", id, note.FriendID)
	}
	if string(got) != string(payload) {
		t.Fatalf("host received %q, want %q", got, payload)
	}
	return id, from
}

func TestRelayForwardsOneFriend(t *testing.T) {
	relay := startTestRelay(t, nil)
	host := dialTestHost(t, relay, testRelayPassword)
	_, token, err := NewFriendToken()
	if err != nil {
		t.Fatal(err)
	}
	host.authorize(t, token)

	hostData := newTestHostData(t)
	friend := newTestFriend(t, relay)
	friendID, from := joinFriend(t, relay, host, hostData, friend, token, []byte("friend hello"))
	relayAddr := relayDataAddr(t, relay)
	if from.Port != relayAddr.Port || !from.IP.Equal(relayAddr.IP) {
		t.Fatalf("host saw relay source %s, want %s", from, relayAddr)
	}

	hostReply(t, hostData, relay, host.dataToken, friendID, []byte("host reply"))
	got, err := readFriend(friend, 3*time.Second)
	if err != nil {
		t.Fatalf("friend read: %v", err)
	}
	if string(got) != "host reply" {
		t.Fatalf("friend received %q, want %q", got, "host reply")
	}
}

func TestRelayForwardsTwoFriendsWithSeparateIDs(t *testing.T) {
	relay := startTestRelay(t, nil)
	host := dialTestHost(t, relay, testRelayPassword)
	hostData := newTestHostData(t)

	var tokens [2]string
	for i := range tokens {
		_, token, err := NewFriendToken()
		if err != nil {
			t.Fatal(err)
		}
		tokens[i] = token
		host.authorize(t, token)
	}

	friendA := newTestFriend(t, relay)
	friendB := newTestFriend(t, relay)
	idA, _ := joinFriend(t, relay, host, hostData, friendA, tokens[0], []byte("from A"))
	idB, _ := joinFriend(t, relay, host, hostData, friendB, tokens[1], []byte("from B"))
	if idA == idB {
		t.Fatalf("both friends use id %d", idA)
	}

	hostReply(t, hostData, relay, host.dataToken, idA, []byte("reply A"))
	hostReply(t, hostData, relay, host.dataToken, idB, []byte("reply B"))
	gotA, err := readFriend(friendA, 3*time.Second)
	if err != nil {
		t.Fatalf("friend A read: %v", err)
	}
	gotB, err := readFriend(friendB, 3*time.Second)
	if err != nil {
		t.Fatalf("friend B read: %v", err)
	}
	if string(gotA) != "reply A" || string(gotB) != "reply B" {
		t.Fatalf("replies = %q and %q", gotA, gotB)
	}
}

func TestRelayBuffersFirstFriendPacket(t *testing.T) {
	relay := startTestRelay(t, nil)
	host := dialTestHost(t, relay, testRelayPassword)
	_, token, err := NewFriendToken()
	if err != nil {
		t.Fatal(err)
	}
	host.authorize(t, token)

	hostData := newTestHostData(t)
	friend := newTestFriend(t, relay)
	sendFriend(t, friend, token, []byte("buffered hello"))
	note := host.expect(t, "friend")
	if note.FriendID == 0 {
		t.Fatalf("friend notification = %+v", note)
	}

	hostKeepalive(t, hostData, relay, host.dataToken)
	id, got, _ := readHostData(t, hostData)
	if id != note.FriendID || string(got) != "buffered hello" {
		t.Fatalf("buffered packet = id %d payload %q", id, got)
	}
}

func TestRelayBindsHostDataAddress(t *testing.T) {
	relay := startTestRelay(t, nil)
	host := dialTestHost(t, relay, testRelayPassword)
	_, token, err := NewFriendToken()
	if err != nil {
		t.Fatal(err)
	}
	host.authorize(t, token)

	hostData := newTestHostData(t)
	friend := newTestFriend(t, relay)
	friendID, _ := joinFriend(t, relay, host, hostData, friend, token, []byte("hello"))

	attacker := newTestHostData(t)
	hostKeepalive(t, attacker, relay, host.dataToken)
	hostReply(t, attacker, relay, host.dataToken, friendID, []byte("from another address"))
	expectNoFriendPacket(t, friend, 300*time.Millisecond)

	hostReply(t, hostData, relay, []byte(strings.Repeat("x", tokenLength)), friendID, []byte("wrong token"))
	expectNoFriendPacket(t, friend, 300*time.Millisecond)

	hostReply(t, hostData, relay, host.dataToken, friendID, []byte("real reply"))
	got, err := readFriend(friend, 3*time.Second)
	if err != nil {
		t.Fatalf("friend read: %v", err)
	}
	if string(got) != "real reply" {
		t.Fatalf("friend received %q", got)
	}
}

func TestRelayRebindsHostDataAfterSilence(t *testing.T) {
	relay := startTestRelay(t, func(server *Server) {
		server.hostRebind = 100 * time.Millisecond
	})
	host := dialTestHost(t, relay, testRelayPassword)
	_, token, err := NewFriendToken()
	if err != nil {
		t.Fatal(err)
	}
	host.authorize(t, token)

	hostData := newTestHostData(t)
	friend := newTestFriend(t, relay)
	friendID, _ := joinFriend(t, relay, host, hostData, friend, token, []byte("hello"))

	time.Sleep(200 * time.Millisecond)
	rebound := newTestHostData(t)
	hostKeepalive(t, rebound, relay, host.dataToken)
	hostReply(t, rebound, relay, host.dataToken, friendID, []byte("from a new mapping"))
	got, err := readFriend(friend, 3*time.Second)
	if err != nil {
		t.Fatalf("friend read: %v", err)
	}
	if string(got) != "from a new mapping" {
		t.Fatalf("friend received %q", got)
	}
}

func TestRelayRejectsWrongPassword(t *testing.T) {
	relay := startTestRelay(t, nil)
	conn, reader := openTestHost(t, relay, "not the password")
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	msg, err := readControl(reader)
	if err != nil {
		t.Fatalf("read rejection: %v", err)
	}
	if msg.Type != "error" || !strings.Contains(msg.Message, "password") {
		t.Fatalf("rejection = %+v", msg)
	}

	host := dialTestHost(t, relay, testRelayPassword)
	host.send(t, controlMessage{Type: "ping"})
	host.expect(t, "pong")
}

func TestRelayRejectsWrongFingerprint(t *testing.T) {
	relay := startTestRelay(t, nil)
	wrong := strings.Repeat("0", 64)
	if conn, err := tls.Dial("tcp", relay.address, ClientTLS(wrong)); err == nil {
		_ = conn.Close()
		t.Fatal("relay key mismatch was accepted")
	}
}

func TestRelayRejectsExpiredToken(t *testing.T) {
	relay := startTestRelay(t, func(server *Server) {
		server.tokenTTL = 200 * time.Millisecond
	})
	host := dialTestHost(t, relay, testRelayPassword)

	_, expired, err := NewFriendToken()
	if err != nil {
		t.Fatal(err)
	}
	host.send(t, controlMessage{Type: "authorize", Token: expired, Expires: time.Now().Add(-time.Second)})
	if msg := host.read(t); msg.Type != "error" {
		t.Fatalf("expired authorization = %+v", msg)
	}

	_, token, err := NewFriendToken()
	if err != nil {
		t.Fatal(err)
	}
	host.authorize(t, token)
	time.Sleep(300 * time.Millisecond)

	friend := newTestFriend(t, relay)
	sendFriend(t, friend, token, []byte("too late"))
	host.expectNone(t, 300*time.Millisecond)
	expectNoFriendPacket(t, friend, 300*time.Millisecond)
}

func TestRelayRejectsTokenReplay(t *testing.T) {
	relay := startTestRelay(t, nil)
	host := dialTestHost(t, relay, testRelayPassword)
	_, token, err := NewFriendToken()
	if err != nil {
		t.Fatal(err)
	}
	host.authorize(t, token)

	hostData := newTestHostData(t)
	first := newTestFriend(t, relay)
	friendID, _ := joinFriend(t, relay, host, hostData, first, token, []byte("first"))

	second := newTestFriend(t, relay)
	sendFriend(t, second, token, []byte("second"))
	host.expectNone(t, 300*time.Millisecond)
	expectNoFriendPacket(t, second, 300*time.Millisecond)

	hostReply(t, hostData, relay, host.dataToken, friendID, []byte("still first"))
	got, err := readFriend(first, 3*time.Second)
	if err != nil {
		t.Fatalf("first friend read: %v", err)
	}
	if string(got) != "still first" {
		t.Fatalf("first friend received %q", got)
	}
}

func TestRelayDropsMalformedPackets(t *testing.T) {
	relay := startTestRelay(t, nil)
	host := dialTestHost(t, relay, testRelayPassword)
	_, token, err := NewFriendToken()
	if err != nil {
		t.Fatal(err)
	}
	host.authorize(t, token)

	friend := newTestFriend(t, relay)
	for _, packet := range [][]byte{
		{},
		{packetFriend},
		{packetHost},
		{packetRelay, 0x00},
		{packetFriend, 1, 2, 3},
		[]byte("junk packet"),
	} {
		if _, err := friend.Write(packet); err != nil {
			t.Fatalf("send malformed packet: %v", err)
		}
	}
	host.expectNone(t, 300*time.Millisecond)

	big := make([]byte, maxRelayPayload+100)
	sendFriend(t, friend, token, big)
	host.expectNone(t, 300*time.Millisecond)
	expectNoFriendPacket(t, friend, 300*time.Millisecond)

	hostData := newTestHostData(t)
	friendID, _ := joinFriend(t, relay, host, hostData, friend, token, []byte("valid"))
	hostReply(t, hostData, relay, host.dataToken, friendID, []byte("valid reply"))
	got, err := readFriend(friend, 3*time.Second)
	if err != nil {
		t.Fatalf("friend read: %v", err)
	}
	if string(got) != "valid reply" {
		t.Fatalf("friend received %q", got)
	}
}

func TestRelayCapsPendingHandshakes(t *testing.T) {
	relay := startTestRelay(t, func(server *Server) {
		server.maxHandshakes = 2
	})
	var idle []net.Conn
	for i := 0; i < 2; i++ {
		conn, err := net.Dial("tcp", relay.address)
		if err != nil {
			t.Fatal(err)
		}
		idle = append(idle, conn)
	}
	waitFor(t, 2*time.Second, func() bool {
		return len(relay.server.handshakeSlots) == 2
	})

	extra, err := net.Dial("tcp", relay.address)
	if err != nil {
		t.Fatal(err)
	}
	defer extra.Close()
	_ = extra.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := extra.Read(make([]byte, 1)); err == nil {
		t.Fatal("relay accepted a connection beyond the handshake cap")
	}

	for _, conn := range idle {
		_ = conn.Close()
	}
	waitFor(t, 2*time.Second, func() bool {
		return len(relay.server.handshakeSlots) == 0
	})
	host := dialTestHost(t, relay, testRelayPassword)
	host.send(t, controlMessage{Type: "ping"})
	host.expect(t, "pong")
}

func TestRelayCapsControlConnectionsPerIP(t *testing.T) {
	relay := startTestRelay(t, func(server *Server) {
		server.maxHandshakes = 8
		server.maxPerIP = 2
	})
	var idle []net.Conn
	for i := 0; i < 2; i++ {
		conn, err := net.Dial("tcp", relay.address)
		if err != nil {
			t.Fatal(err)
		}
		idle = append(idle, conn)
	}
	waitFor(t, 2*time.Second, func() bool {
		relay.server.mu.Lock()
		defer relay.server.mu.Unlock()
		return relay.server.connsByIP["127.0.0.1"] == 2
	})

	extra, err := net.Dial("tcp", relay.address)
	if err != nil {
		t.Fatal(err)
	}
	defer extra.Close()
	_ = extra.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := extra.Read(make([]byte, 1)); err == nil {
		t.Fatal("relay accepted a connection beyond the per-IP cap")
	}

	for _, conn := range idle {
		_ = conn.Close()
	}
	waitFor(t, 2*time.Second, func() bool {
		relay.server.mu.Lock()
		defer relay.server.mu.Unlock()
		return len(relay.server.connsByIP) == 0
	})
	host := dialTestHost(t, relay, testRelayPassword)
	host.send(t, controlMessage{Type: "ping"})
	host.expect(t, "pong")
}

func TestRateLimiter(t *testing.T) {
	var limiter rateLimiter
	now := time.Now()
	for i := 0; i < 5; i++ {
		if !limiter.allow(now, 10, 5) {
			t.Fatalf("burst packet %d was denied", i)
		}
	}
	if limiter.allow(now, 10, 5) {
		t.Fatal("the burst cap was exceeded")
	}
	if !limiter.allow(now.Add(100*time.Millisecond), 10, 5) {
		t.Fatal("a refilled packet was denied")
	}
	if limiter.allow(now.Add(100*time.Millisecond), 10, 5) {
		t.Fatal("a packet was allowed without enough refill")
	}
}

func TestRelayCapsOutstandingFriendTokens(t *testing.T) {
	relay := startTestRelay(t, func(server *Server) {
		server.maxTokensPerSession = 1
	})
	host := dialTestHost(t, relay, testRelayPassword)
	_, first, err := NewFriendToken()
	if err != nil {
		t.Fatal(err)
	}
	host.authorize(t, first)

	_, second, err := NewFriendToken()
	if err != nil {
		t.Fatal(err)
	}
	host.send(t, controlMessage{Type: "authorize", Token: second})
	if msg := host.read(t); msg.Type != "error" || !strings.Contains(msg.Message, "outstanding") {
		t.Fatalf("token cap response = %+v", msg)
	}

	// Refreshing an existing token is allowed at the cap.
	host.authorize(t, first)
}

func TestRelaySessionLimit(t *testing.T) {
	relay := startTestRelay(t, func(server *Server) {
		server.cfg.MaxSessions = 1
	})
	first := dialTestHost(t, relay, testRelayPassword)
	if first.sessionID == "" {
		t.Fatal("first host has no session")
	}

	conn, reader := openTestHost(t, relay, testRelayPassword)
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	msg, err := readControl(reader)
	if err != nil {
		t.Fatalf("read session limit: %v", err)
	}
	if msg.Type != "error" || !strings.Contains(msg.Message, "session limit") {
		t.Fatalf("session limit response = %+v", msg)
	}
}

func TestRelayFriendLimit(t *testing.T) {
	relay := startTestRelay(t, func(server *Server) {
		server.cfg.MaxFriends = 1
	})
	host := dialTestHost(t, relay, testRelayPassword)
	hostData := newTestHostData(t)

	var tokens [2]string
	for i := range tokens {
		_, token, err := NewFriendToken()
		if err != nil {
			t.Fatal(err)
		}
		tokens[i] = token
		host.authorize(t, token)
	}
	friendA := newTestFriend(t, relay)
	joinFriend(t, relay, host, hostData, friendA, tokens[0], []byte("A"))

	friendB := newTestFriend(t, relay)
	sendFriend(t, friendB, tokens[1], []byte("B"))
	host.expectNone(t, 300*time.Millisecond)
	expectNoFriendPacket(t, friendB, 300*time.Millisecond)
}

func TestRelayRemovesIdleFriend(t *testing.T) {
	relay := startTestRelay(t, func(server *Server) {
		server.friendIdle = 150 * time.Millisecond
		server.janitorInterval = 50 * time.Millisecond
	})
	host := dialTestHost(t, relay, testRelayPassword)
	_, token, err := NewFriendToken()
	if err != nil {
		t.Fatal(err)
	}
	host.authorize(t, token)

	hostData := newTestHostData(t)
	friend := newTestFriend(t, relay)
	friendID, _ := joinFriend(t, relay, host, hostData, friend, token, []byte("idle"))

	left := host.expect(t, "friendLeft")
	if left.FriendID != friendID {
		t.Fatalf("friend left id = %d, want %d", left.FriendID, friendID)
	}

	// Tokens are single-use, so the same invite cannot rejoin after idle.
	sendFriend(t, friend, token, []byte("again"))
	host.expectNone(t, 300*time.Millisecond)
}

func TestRelayClosesFriendOnSessionEnd(t *testing.T) {
	relay := startTestRelay(t, nil)
	host := dialTestHost(t, relay, testRelayPassword)
	_, token, err := NewFriendToken()
	if err != nil {
		t.Fatal(err)
	}
	host.authorize(t, token)

	hostData := newTestHostData(t)
	friend := newTestFriend(t, relay)
	joinFriend(t, relay, host, hostData, friend, token, []byte("before close"))

	_ = host.conn.Close()
	waitFor(t, 3*time.Second, func() bool {
		relay.server.mu.Lock()
		defer relay.server.mu.Unlock()
		return len(relay.server.sessions) == 0
	})

	sendFriend(t, friend, token, []byte("after close"))
	expectNoFriendPacket(t, friend, 300*time.Millisecond)
}

func TestRelayClosesIdleControl(t *testing.T) {
	relay := startTestRelay(t, func(server *Server) {
		server.controlIdle = 200 * time.Millisecond
	})
	host := dialTestHost(t, relay, testRelayPassword)
	_ = host.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := readControl(host.reader); err == nil {
		t.Fatal("relay kept an idle control session open")
	}
	waitFor(t, 3*time.Second, func() bool {
		relay.server.mu.Lock()
		defer relay.server.mu.Unlock()
		return len(relay.server.sessions) == 0
	})
}

func TestRelayCloseInterruptsHandshakes(t *testing.T) {
	relay := startTestRelay(t, nil)
	conn, err := net.Dial("tcp", relay.address)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	waitFor(t, 2*time.Second, func() bool {
		return len(relay.server.handshakeSlots) == 1
	})

	done := make(chan struct{})
	go func() {
		_ = relay.server.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("relay Close blocked on a pending handshake")
	}
}

func waitFor(t *testing.T, timeout time.Duration, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition was not met before the deadline")
}

func tokenHashText(raw []byte) string {
	hash := TokenHash(raw)
	return hex.EncodeToString(hash[:])
}

func TestRelayReusableTokenRebindsAfterSilence(t *testing.T) {
	relay := startTestRelay(t, func(server *Server) {
		server.rebindGrace = 50 * time.Millisecond
	})
	host := dialTestHost(t, relay, testRelayPassword)
	raw, token, err := NewFriendToken()
	if err != nil {
		t.Fatal(err)
	}
	host.send(t, controlMessage{Type: "authorize", TokenHash: tokenHashText(raw), Reusable: true})
	host.expect(t, "ok")

	hostData := newTestHostData(t)
	friendA := newTestFriend(t, relay)
	sendFriend(t, friendA, token, []byte("from A"))
	noteA := host.expect(t, "friend")
	hostKeepalive(t, hostData, relay, host.dataToken)
	id, payload, _ := readHostData(t, hostData)
	if id != noteA.FriendID || string(payload) != "from A" {
		t.Fatalf("first packet = id %d %q", id, payload)
	}

	time.Sleep(100 * time.Millisecond)
	friendB := newTestFriend(t, relay)
	sendFriend(t, friendB, token, []byte("from B"))

	seen := map[string]uint16{}
	for len(seen) < 2 {
		msg := host.read(t)
		seen[msg.Type] = msg.FriendID
	}
	if seen["friendLeft"] != noteA.FriendID {
		t.Fatalf("friendLeft = %d, want %d", seen["friendLeft"], noteA.FriendID)
	}
	if seen["friend"] == 0 || seen["friend"] == noteA.FriendID {
		t.Fatalf("new friend id = %d", seen["friend"])
	}

	hostKeepalive(t, hostData, relay, host.dataToken)
	id, payload, _ = readHostData(t, hostData)
	if id != seen["friend"] || string(payload) != "from B" {
		t.Fatalf("second packet = id %d %q", id, payload)
	}
}

func TestRelayReusableTokenSurvivesFriendIdle(t *testing.T) {
	relay := startTestRelay(t, func(server *Server) {
		server.friendIdle = 80 * time.Millisecond
		server.janitorInterval = 10 * time.Millisecond
	})
	host := dialTestHost(t, relay, testRelayPassword)
	raw, token, err := NewFriendToken()
	if err != nil {
		t.Fatal(err)
	}
	host.send(t, controlMessage{Type: "authorize", TokenHash: tokenHashText(raw), Reusable: true})
	host.expect(t, "ok")

	friend := newTestFriend(t, relay)
	sendFriend(t, friend, token, []byte("first"))
	first := host.expect(t, "friend")
	if left := host.expect(t, "friendLeft"); left.FriendID != first.FriendID {
		t.Fatalf("friendLeft = %d, want %d", left.FriendID, first.FriendID)
	}

	sendFriend(t, friend, token, []byte("second"))
	second := host.expect(t, "friend")
	if second.FriendID == 0 || second.FriendID == first.FriendID {
		t.Fatalf("second friend id = %d", second.FriendID)
	}
}

func TestRelayRevokeReusableToken(t *testing.T) {
	relay := startTestRelay(t, nil)
	host := dialTestHost(t, relay, testRelayPassword)
	raw, token, err := NewFriendToken()
	if err != nil {
		t.Fatal(err)
	}
	hashText := tokenHashText(raw)
	host.send(t, controlMessage{Type: "authorize", TokenHash: hashText, Reusable: true})
	host.expect(t, "ok")
	host.send(t, controlMessage{Type: "revoke", TokenHash: hashText})
	host.expect(t, "ok")

	hostData := newTestHostData(t)
	friend := newTestFriend(t, relay)
	sendFriend(t, friend, token, []byte("after revoke"))
	expectNoFriendPacket(t, friend, 200*time.Millisecond)
	host.expectNone(t, 200*time.Millisecond)
	_ = hostData
}

func canBindUDP(t *testing.T, address string) bool {
	t.Helper()
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func TestRelayDataPlaneSleepsBetweenSessions(t *testing.T) {
	relay := startTestRelay(t, nil)
	if !canBindUDP(t, relay.address) {
		t.Fatal("UDP was open before any host session")
	}

	host := dialTestHost(t, relay, testRelayPassword)
	if canBindUDP(t, relay.address) {
		t.Fatal("UDP did not open for the host session")
	}
	_ = host.conn.Close()
	waitFor(t, 3*time.Second, func() bool {
		relay.server.mu.Lock()
		defer relay.server.mu.Unlock()
		return len(relay.server.sessions) == 0
	})
	waitFor(t, 3*time.Second, func() bool { return canBindUDP(t, relay.address) })

	second := dialTestHost(t, relay, testRelayPassword)
	if canBindUDP(t, relay.address) {
		t.Fatal("UDP did not reopen for the next host session")
	}
	_ = second.conn.Close()
}

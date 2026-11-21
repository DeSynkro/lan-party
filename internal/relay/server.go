package relay

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

const (
	packetFriend byte = 1
	packetHost   byte = 2
	packetRelay  byte = 3

	friendIDLength = 2

	defaultTokenTTL        = time.Hour
	defaultReusableTTL     = 365 * 24 * time.Hour
	defaultRebindGrace     = 10 * time.Second
	defaultFriendIdle      = 5 * time.Minute
	defaultControlIdle     = 90 * time.Second
	defaultHostRebind      = 90 * time.Second
	defaultHandshakeWindow = 10 * time.Second
	defaultControlWrite    = 10 * time.Second
	defaultDenialDelay     = 500 * time.Millisecond

	maxControlMessage      = 64 << 10
	maxRelayPayload        = 2048
	maxDatagramSize        = 64 << 10
	defaultHandshakeSlots  = 64
	defaultControlPerIP    = 8
	maxFriendPacketsPerSec = 20000
	maxFriendPacketBurst   = 4000
	defaultMaxFriendTokens = 256
)

type controlMessage struct {
	Type      string    `json:"type"`
	Version   int       `json:"version,omitempty"`
	Role      string    `json:"role,omitempty"`
	Password  string    `json:"password,omitempty"`
	SessionID string    `json:"sessionId,omitempty"`
	DataToken string    `json:"dataToken,omitempty"`
	Token     string    `json:"token,omitempty"`
	TokenHash string    `json:"tokenHash,omitempty"`
	Reusable  bool      `json:"reusable,omitempty"`
	FriendID  uint16    `json:"friendId,omitempty"`
	Expires   time.Time `json:"expires,omitempty"`
	Message   string    `json:"message,omitempty"`
}

type tokenEntry struct {
	session  *session
	expires  time.Time
	reusable bool
	friend   *friendState
}

type friendState struct {
	id         uint16
	token      [32]byte
	addr       *net.UDPAddr
	pending    []byte
	lastSeen   time.Time
	lastNotify time.Time
	limiter    rateLimiter
	closed     bool
}

type rateLimiter struct {
	tokens float64
	last   time.Time
}

func (r *rateLimiter) allow(now time.Time, rate, burst float64) bool {
	if r.last.IsZero() {
		r.last = now
		r.tokens = burst
	}
	if elapsed := now.Sub(r.last).Seconds(); elapsed > 0 {
		r.tokens += elapsed * rate
		if r.tokens > burst {
			r.tokens = burst
		}
		r.last = now
	}
	if r.tokens < 1 {
		return false
	}
	r.tokens--
	return true
}

type session struct {
	id           string
	control      net.Conn
	writeMu      sync.Mutex
	dataToken    [32]byte
	hostAddr     *net.UDPAddr
	hostSeen     time.Time
	friends      map[uint16]*friendState
	nextFriendID uint16
	closed       bool
}

type pendingFriendPacket struct {
	friendID uint16
	payload  []byte
}

// Server is the headless relay daemon.
type Server struct {
	cfg            Config
	cert           tls.Certificate
	fingerprint    string
	tlsConfig      *tls.Config
	tcp            net.Listener
	udp            *net.UDPConn
	ctx            context.Context
	cancel         context.CancelFunc
	closeOnce      sync.Once
	wg             sync.WaitGroup
	mu             sync.Mutex
	closing        bool
	sessions       map[string]*session
	hostSessions   map[[32]byte]*session
	tokens         map[[32]byte]*tokenEntry
	pending        map[net.Conn]struct{}
	connsByIP      map[string]int
	handshakeSlots chan struct{}

	tokenTTL            time.Duration
	reusableTTL         time.Duration
	rebindGrace         time.Duration
	friendIdle          time.Duration
	controlIdle         time.Duration
	hostRebind          time.Duration
	handshakeWindow     time.Duration
	janitorInterval     time.Duration
	maxHandshakes       int
	maxPerIP            int
	maxTokensPerSession int
}

// NewServer prepares a relay server with the given config and certificate.
func NewServer(cfg Config, cert tls.Certificate, fingerprint string) (*Server, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if cfg.MaxSessions == 0 {
		cfg.MaxSessions = 16
	}
	if cfg.MaxFriends == 0 {
		cfg.MaxFriends = 64
	}
	return &Server{
		cfg:         cfg,
		cert:        cert,
		fingerprint: fingerprint,
		tlsConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS13,
		},
		sessions:            map[string]*session{},
		hostSessions:        map[[32]byte]*session{},
		tokens:              map[[32]byte]*tokenEntry{},
		pending:             map[net.Conn]struct{}{},
		connsByIP:           map[string]int{},
		tokenTTL:            defaultTokenTTL,
		reusableTTL:         defaultReusableTTL,
		rebindGrace:         defaultRebindGrace,
		friendIdle:          defaultFriendIdle,
		controlIdle:         defaultControlIdle,
		hostRebind:          defaultHostRebind,
		handshakeWindow:     defaultHandshakeWindow,
		janitorInterval:     30 * time.Second,
		maxHandshakes:       defaultHandshakeSlots,
		maxPerIP:            defaultControlPerIP,
		maxTokensPerSession: defaultMaxFriendTokens,
	}, nil
}

// Fingerprint returns the relay public-key fingerprint.
func (s *Server) Fingerprint() string { return s.fingerprint }

// Start opens the control listener and starts serving. The UDP data plane is
// opened only while at least one host session is active.
func (s *Server) Start() error {
	tcp, err := net.Listen("tcp", s.cfg.Listen)
	if err != nil {
		return fmt.Errorf("listen control: %w", err)
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.tcp = tcp
	slots := s.maxHandshakes
	if slots <= 0 {
		slots = defaultHandshakeSlots
	}
	s.handshakeSlots = make(chan struct{}, slots)

	s.wg.Add(1)
	go s.acceptLoop()
	s.wg.Add(1)
	go s.janitorLoop()
	return nil
}

// Close stops the relay and all sessions.
func (s *Server) Close() error {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closing = true
		sessions := make([]*session, 0, len(s.sessions))
		for _, session := range s.sessions {
			sessions = append(sessions, session)
		}
		pending := make([]net.Conn, 0, len(s.pending))
		for conn := range s.pending {
			pending = append(pending, conn)
		}
		udp := s.udp
		s.udp = nil
		s.mu.Unlock()

		if s.cancel != nil {
			s.cancel()
		}
		if s.tcp != nil {
			_ = s.tcp.Close()
		}
		if udp != nil {
			_ = udp.Close()
		}
		for _, conn := range pending {
			_ = conn.Close()
		}
		for _, session := range sessions {
			s.closeSession(session)
		}
		s.wg.Wait()
	})
	return nil
}

// admitConn records a new control connection unless shutdown has begun or the
// peer already has too many connections. It also reserves a handshake slot.
func (s *Server) admitConn(conn net.Conn) bool {
	ip := remoteIP(conn.RemoteAddr())
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		return false
	}
	if ip != "" && s.maxPerIP > 0 && s.connsByIP[ip] >= s.maxPerIP {
		return false
	}
	select {
	case s.handshakeSlots <- struct{}{}:
	default:
		return false
	}
	if ip != "" {
		s.connsByIP[ip]++
	}
	s.pending[conn] = struct{}{}
	s.wg.Add(1)
	return true
}

func (s *Server) releaseIP(ip string) {
	if ip == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if n := s.connsByIP[ip]; n <= 1 {
		delete(s.connsByIP, ip)
	} else {
		s.connsByIP[ip] = n - 1
	}
}

func remoteIP(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.tcp.Accept()
		if err != nil {
			return
		}
		if !s.admitConn(conn) {
			_ = conn.Close()
			continue
		}
		go s.handleControl(conn)
	}
}

func (s *Server) handleControl(conn net.Conn) {
	ip := remoteIP(conn.RemoteAddr())
	defer s.wg.Done()
	defer conn.Close()
	defer s.releaseIP(ip)
	defer func() {
		s.mu.Lock()
		delete(s.pending, conn)
		s.mu.Unlock()
	}()

	slotHeld := true
	releaseSlot := func() {
		if slotHeld {
			<-s.handshakeSlots
			slotHeld = false
		}
	}
	defer releaseSlot()

	tlsConn := tls.Server(conn, s.tlsConfig)
	_ = tlsConn.SetDeadline(time.Now().Add(s.handshakeWindow))
	if err := tlsConn.Handshake(); err != nil {
		log.Printf("relay: control handshake from %s: %v", conn.RemoteAddr(), err)
		return
	}
	reader := bufio.NewReader(tlsConn)
	hello, err := readControl(reader)
	if err != nil {
		log.Printf("relay: control hello from %s: %v", conn.RemoteAddr(), err)
		return
	}
	if hello.Type != "hello" || hello.Version != Version || hello.Role != "host" {
		_ = writeControl(tlsConn, controlMessage{Type: "error", Message: "unsupported relay hello"})
		return
	}
	if len(hello.Password) > 512 {
		_ = writeControl(tlsConn, controlMessage{Type: "error", Message: "relay hello is too large"})
		return
	}
	if !VerifyPassword(hello.Password, s.cfg.PasswordSalt, s.cfg.PasswordHash) {
		time.Sleep(defaultDenialDelay)
		_ = writeControl(tlsConn, controlMessage{Type: "error", Message: "relay password rejected"})
		log.Printf("relay: rejected host password from %s", conn.RemoteAddr())
		return
	}
	_ = tlsConn.SetDeadline(time.Time{})
	releaseSlot()

	sessionID, err := newSessionID()
	if err != nil {
		log.Printf("relay: %v", err)
		return
	}
	dataToken, err := newDataToken()
	if err != nil {
		log.Printf("relay: %v", err)
		return
	}
	session := &session{
		id:        sessionID,
		control:   tlsConn,
		dataToken: TokenHash(dataToken),
		friends:   map[uint16]*friendState{},
	}

	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		return
	}
	if s.cfg.MaxSessions > 0 && len(s.sessions) >= s.cfg.MaxSessions {
		s.mu.Unlock()
		_ = writeControl(tlsConn, controlMessage{Type: "error", Message: "relay is at its session limit"})
		return
	}
	if err := s.ensureDataLocked(); err != nil {
		s.mu.Unlock()
		_ = writeControl(tlsConn, controlMessage{Type: "error", Message: "relay data plane is unavailable"})
		log.Printf("relay: start data plane: %v", err)
		return
	}
	s.sessions[sessionID] = session
	s.hostSessions[session.dataToken] = session
	s.mu.Unlock()

	log.Printf("relay: host session %s from %s", sessionID, conn.RemoteAddr())
	if err := session.send(controlMessage{
		Type:      "welcome",
		SessionID: sessionID,
		DataToken: base64.RawURLEncoding.EncodeToString(dataToken),
	}); err != nil {
		s.closeSession(session)
		return
	}
	defer s.closeSession(session)

	for {
		_ = tlsConn.SetReadDeadline(time.Now().Add(s.controlIdle))
		message, err := readControl(reader)
		if err != nil {
			if !errors.Is(err, io.EOF) && !isTimeout(err) {
				log.Printf("relay: session %s control: %v", sessionID, err)
			}
			return
		}
		switch message.Type {
		case "authorize":
			s.authorizeFriend(session, message)
		case "revoke":
			s.revokeFriend(session, message)
		case "ping":
			_ = session.send(controlMessage{Type: "pong"})
		case "bye":
			return
		default:
			_ = session.send(controlMessage{Type: "error", Message: "unknown relay control message"})
		}
	}
}

func (s *Server) authorizeFriend(session *session, message controlMessage) {
	var hash [32]byte
	switch {
	case message.TokenHash != "":
		if message.Token != "" {
			_ = session.send(controlMessage{Type: "error", Message: "send a friend token or a token hash, not both"})
			return
		}
		decoded, err := hex.DecodeString(message.TokenHash)
		if err != nil || len(decoded) != len(hash) {
			_ = session.send(controlMessage{Type: "error", Message: "the friend token hash is invalid"})
			return
		}
		copy(hash[:], decoded)
	case message.Token != "":
		if len(message.Token) > 512 {
			_ = session.send(controlMessage{Type: "error", Message: "friend token is too large"})
			return
		}
		raw, err := DecodeFriendToken(message.Token)
		if err != nil {
			_ = session.send(controlMessage{Type: "error", Message: err.Error()})
			return
		}
		hash = TokenHash(raw)
	default:
		_ = session.send(controlMessage{Type: "error", Message: "the friend token is empty"})
		return
	}

	now := time.Now()
	ttl := s.tokenTTL
	if message.Reusable {
		ttl = s.reusableTTL
	}
	expires := message.Expires
	latest := now.Add(ttl)
	if expires.IsZero() || expires.After(latest) {
		expires = latest
	}
	if !expires.After(now) {
		_ = session.send(controlMessage{Type: "error", Message: "friend token already expired"})
		return
	}

	s.mu.Lock()
	if session.closed || s.closing {
		s.mu.Unlock()
		return
	}
	if entry, exists := s.tokens[hash]; exists {
		if entry.session != session || entry.session.closed {
			s.mu.Unlock()
			_ = session.send(controlMessage{Type: "error", Message: "friend token is already in use"})
			return
		}
		if !entry.reusable && entry.friend != nil {
			s.mu.Unlock()
			_ = session.send(controlMessage{Type: "error", Message: "friend token is already in use"})
			return
		}
		entry.expires = expires
		entry.reusable = message.Reusable
		s.mu.Unlock()
		_ = session.send(controlMessage{Type: "ok"})
		return
	}
	if s.maxTokensPerSession > 0 {
		count := 0
		for _, entry := range s.tokens {
			if entry.session == session {
				count++
			}
		}
		if count >= s.maxTokensPerSession {
			s.mu.Unlock()
			_ = session.send(controlMessage{Type: "error", Message: "too many outstanding friend tokens"})
			return
		}
	}
	s.tokens[hash] = &tokenEntry{session: session, expires: expires, reusable: message.Reusable}
	s.mu.Unlock()
	_ = session.send(controlMessage{Type: "ok"})
}

func (s *Server) revokeFriend(session *session, message controlMessage) {
	decoded, err := hex.DecodeString(message.TokenHash)
	if err != nil || len(decoded) != 32 {
		_ = session.send(controlMessage{Type: "error", Message: "the friend token hash is invalid"})
		return
	}
	var hash [32]byte
	copy(hash[:], decoded)

	s.mu.Lock()
	entry := s.tokens[hash]
	if entry == nil || entry.session != session {
		s.mu.Unlock()
		_ = session.send(controlMessage{Type: "ok"})
		return
	}
	delete(s.tokens, hash)
	var left *friendState
	if entry.friend != nil {
		left = entry.friend
		left.closed = true
		delete(session.friends, left.id)
	}
	s.mu.Unlock()

	if left != nil {
		_ = session.send(controlMessage{Type: "friendLeft", FriendID: left.id})
	}
	_ = session.send(controlMessage{Type: "ok"})
}

func (s *Server) ensureDataLocked() error {
	if s.udp != nil {
		return nil
	}
	addr, err := net.ResolveUDPAddr("udp", s.cfg.Listen)
	if err != nil {
		return fmt.Errorf("resolve data address: %w", err)
	}
	udp, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("listen data: %w", err)
	}
	s.udp = udp
	s.wg.Add(1)
	go s.dataLoop(udp)
	log.Printf("relay: data plane active on %s", s.cfg.Listen)
	return nil
}

func (s *Server) stopDataLocked() {
	if s.udp == nil {
		return
	}
	_ = s.udp.Close()
	s.udp = nil
	log.Printf("relay: data plane asleep")
}

func (s *Server) dataLoop(conn *net.UDPConn) {
	defer s.wg.Done()
	buf := make([]byte, maxDatagramSize)
	for {
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		if n < 1 {
			continue
		}
		switch buf[0] {
		case packetFriend:
			s.handleFriendPacket(buf[1:n], addr)
		case packetHost:
			s.handleHostPacket(buf[1:n], addr)
		}
	}
}

func (s *Server) handleFriendPacket(packet []byte, addr *net.UDPAddr) {
	if len(packet) < tokenLength {
		return
	}
	hash := TokenHash(packet[:tokenLength])
	payload := packet[tokenLength:]
	if len(payload) > maxRelayPayload {
		return
	}
	now := time.Now()

	s.mu.Lock()
	entry := s.tokens[hash]
	if entry == nil || entry.session.closed {
		s.mu.Unlock()
		return
	}
	session := entry.session
	friend := entry.friend
	newFriend := false
	var left *friendState
	if friend != nil && (friend.closed || !sameUDPAddr(friend.addr, addr)) {
		if !entry.reusable || now.Sub(friend.lastSeen) < s.rebindGrace {
			s.mu.Unlock()
			return
		}
		delete(session.friends, friend.id)
		friend.closed = true
		left = friend
		entry.friend = nil
		friend = nil
	}
	if friend == nil {
		if !entry.expires.After(now) {
			delete(s.tokens, hash)
			s.mu.Unlock()
			return
		}
		if s.cfg.MaxFriends > 0 && len(session.friends) >= s.cfg.MaxFriends {
			s.mu.Unlock()
			return
		}
		id, ok := allocateFriendID(session)
		if !ok {
			s.mu.Unlock()
			log.Printf("relay: no friend ids available in session %s", session.id)
			return
		}
		friend = &friendState{
			id:         id,
			token:      hash,
			addr:       cloneUDPAddr(addr),
			lastSeen:   now,
			lastNotify: now,
		}
		session.friends[id] = friend
		entry.friend = friend
		newFriend = true
	}
	friend.lastSeen = now
	if !friend.limiter.allow(now, maxFriendPacketsPerSec, maxFriendPacketBurst) {
		s.mu.Unlock()
		return
	}
	host := cloneUDPAddr(session.hostAddr)
	if host == nil && len(payload) > 0 && len(friend.pending) == 0 {
		friend.pending = append([]byte(nil), payload...)
	}
	notify := newFriend
	if !newFriend && host == nil && now.Sub(friend.lastNotify) >= time.Second {
		friend.lastNotify = now
		notify = true
	}
	friendID := friend.id
	s.mu.Unlock()

	if left != nil {
		if err := session.send(controlMessage{Type: "friendLeft", FriendID: left.id}); err != nil {
			s.closeSession(session)
			return
		}
	}
	if notify {
		if err := session.send(controlMessage{Type: "friend", FriendID: friendID}); err != nil {
			s.closeSession(session)
			return
		}
		if newFriend {
			log.Printf("relay: friend %d joined session %s from %s", friendID, session.id, addr)
		}
	}
	if host == nil || len(payload) == 0 {
		return
	}
	s.sendToHost(host, friendID, payload)
}

func (s *Server) handleHostPacket(packet []byte, addr *net.UDPAddr) {
	if len(packet) < tokenLength {
		return
	}
	hash := TokenHash(packet[:tokenLength])
	now := time.Now()

	s.mu.Lock()
	session := s.hostSessions[hash]
	if session == nil || session.closed {
		s.mu.Unlock()
		return
	}
	first := session.hostAddr == nil
	if first || now.Sub(session.hostSeen) > s.hostRebind {
		session.hostAddr = cloneUDPAddr(addr)
	} else if !sameUDPAddr(session.hostAddr, addr) {
		s.mu.Unlock()
		return
	}
	session.hostSeen = now
	host := cloneUDPAddr(session.hostAddr)

	var pending []pendingFriendPacket
	if first {
		for id, friend := range session.friends {
			if len(friend.pending) > 0 {
				pending = append(pending, pendingFriendPacket{friendID: id, payload: friend.pending})
				friend.pending = nil
			}
		}
	}

	var friendID uint16
	var payload []byte
	if len(packet) >= tokenLength+friendIDLength {
		friendID = binary.BigEndian.Uint16(packet[tokenLength : tokenLength+friendIDLength])
		payload = packet[tokenLength+friendIDLength:]
		if len(payload) > maxRelayPayload {
			payload = nil
		}
	}

	var dest *net.UDPAddr
	if len(payload) > 0 {
		friend := session.friends[friendID]
		if friend != nil && !friend.closed && friend.limiter.allow(now, maxFriendPacketsPerSec, maxFriendPacketBurst) {
			dest = cloneUDPAddr(friend.addr)
		}
	}
	udp := s.udp
	s.mu.Unlock()

	s.flushToHost(host, pending)
	if udp == nil || dest == nil || len(payload) == 0 {
		return
	}
	if _, err := udp.WriteToUDP(payload, dest); err != nil {
		log.Printf("relay: forward to friend: %v", err)
	}
}

func (s *Server) sendToHost(host *net.UDPAddr, friendID uint16, payload []byte) {
	if host == nil || len(payload) == 0 {
		return
	}
	packet := make([]byte, 0, 1+friendIDLength+len(payload))
	packet = append(packet, packetRelay)
	var id [friendIDLength]byte
	binary.BigEndian.PutUint16(id[:], friendID)
	packet = append(packet, id[:]...)
	packet = append(packet, payload...)
	s.mu.Lock()
	udp := s.udp
	s.mu.Unlock()
	if udp == nil {
		return
	}
	if _, err := udp.WriteToUDP(packet, host); err != nil {
		log.Printf("relay: forward to host: %v", err)
	}
}

func (s *Server) flushToHost(host *net.UDPAddr, pending []pendingFriendPacket) {
	for _, item := range pending {
		s.sendToHost(host, item.friendID, item.payload)
	}
}

func allocateFriendID(session *session) (uint16, bool) {
	for attempt := 0; attempt < 1<<16; attempt++ {
		session.nextFriendID++
		if session.nextFriendID == 0 {
			continue
		}
		if _, exists := session.friends[session.nextFriendID]; !exists {
			return session.nextFriendID, true
		}
	}
	return 0, false
}

func (s *Server) janitorLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(s.janitorInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case now := <-ticker.C:
			type expiredFriend struct {
				session *session
				friend  *friendState
			}
			var expired []expiredFriend

			s.mu.Lock()
			for hash, entry := range s.tokens {
				if entry.friend == nil && !entry.expires.After(now) {
					delete(s.tokens, hash)
				}
			}
			for _, session := range s.sessions {
				for id, friend := range session.friends {
					if now.Sub(friend.lastSeen) > s.friendIdle {
						delete(session.friends, id)
						if entry := s.tokens[friend.token]; entry != nil {
							if entry.reusable {
								entry.friend = nil
							} else {
								delete(s.tokens, friend.token)
							}
						}
						friend.closed = true
						expired = append(expired, expiredFriend{session: session, friend: friend})
					}
				}
			}
			s.mu.Unlock()

			for _, item := range expired {
				if err := item.session.send(controlMessage{Type: "friendLeft", FriendID: item.friend.id}); err != nil {
					s.closeSession(item.session)
				}
				log.Printf("relay: friend %d left session %s after idle timeout", item.friend.id, item.session.id)
			}
		}
	}
}

func (s *Server) closeSession(session *session) {
	s.mu.Lock()
	if session.closed {
		s.mu.Unlock()
		return
	}
	session.closed = true
	delete(s.sessions, session.id)
	if s.hostSessions[session.dataToken] == session {
		delete(s.hostSessions, session.dataToken)
	}
	for hash, entry := range s.tokens {
		if entry.session == session {
			delete(s.tokens, hash)
		}
	}
	for id, friend := range session.friends {
		delete(session.friends, id)
		friend.closed = true
	}
	if !s.closing && len(s.sessions) == 0 {
		s.stopDataLocked()
	}
	s.mu.Unlock()

	if session.control != nil {
		_ = session.control.Close()
	}
	log.Printf("relay: session %s ended", session.id)
}

func (s *session) send(message controlMessage) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if s.control == nil {
		return errors.New("relay session has no control connection")
	}
	_ = s.control.SetWriteDeadline(time.Now().Add(defaultControlWrite))
	err := writeControl(s.control, message)
	_ = s.control.SetWriteDeadline(time.Time{})
	if err != nil {
		// A partial control message leaves the stream unusable.
		_ = s.control.Close()
	}
	return err
}

func readControl(reader *bufio.Reader) (controlMessage, error) {
	var size [4]byte
	if _, err := io.ReadFull(reader, size[:]); err != nil {
		return controlMessage{}, err
	}
	n := binary.BigEndian.Uint32(size[:])
	if n == 0 || n > maxControlMessage {
		return controlMessage{}, errors.New("relay control message is too large")
	}
	data := make([]byte, n)
	if _, err := io.ReadFull(reader, data); err != nil {
		return controlMessage{}, err
	}
	var message controlMessage
	if err := json.Unmarshal(data, &message); err != nil {
		return controlMessage{}, fmt.Errorf("parse relay control message: %w", err)
	}
	return message, nil
}

func writeControl(writer io.Writer, message controlMessage) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	if len(data) > maxControlMessage {
		return errors.New("relay control message is too large")
	}
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(data)))
	if _, err := writer.Write(size[:]); err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
}

func cloneUDPAddr(addr *net.UDPAddr) *net.UDPAddr {
	if addr == nil {
		return nil
	}
	return &net.UDPAddr{IP: append(net.IP(nil), addr.IP...), Port: addr.Port, Zone: addr.Zone}
}

func sameUDPAddr(a, b *net.UDPAddr) bool {
	return a != nil && b != nil && a.Port == b.Port && a.IP.Equal(b.IP) && a.Zone == b.Zone
}

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

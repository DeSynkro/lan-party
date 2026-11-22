package webui

import (
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// AuditEntry is one recorded administrative action. Detail never contains
// invite codes, passwords, keys or session IDs.
type AuditEntry struct {
	At     time.Time
	Action string
	IP     string
	Detail string
}

// AuditLog keeps a bounded recent history and writes the same lines to the
// application log.
type AuditLog struct {
	mu      sync.Mutex
	max     int
	entries []AuditEntry
	now     func() time.Time
}

// NewAuditLog creates a log holding at most max entries.
func NewAuditLog(max int) *AuditLog {
	if max < 1 {
		max = 1
	}
	return &AuditLog{max: max, now: time.Now}
}

// Record adds an entry, newest first.
func (a *AuditLog) Record(action, ip, detail string) {
	entry := AuditEntry{
		At:     a.now().UTC(),
		Action: sanitizeLog(action),
		IP:     sanitizeLog(ip),
		Detail: sanitizeLog(detail),
	}
	a.mu.Lock()
	a.entries = append([]AuditEntry{entry}, a.entries...)
	if len(a.entries) > a.max {
		a.entries = a.entries[:a.max]
	}
	a.mu.Unlock()
	log.Printf("audit: %s ip=%s %s", entry.Action, entry.IP, entry.Detail)
}

// sanitizeLog keeps one entry on one line and bounds its length.
func sanitizeLog(value string) string {
	value = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		return r
	}, value)
	if len(value) > 200 {
		value = value[:200]
	}
	return value
}

// Recent returns the newest entries first.
func (a *AuditLog) Recent() []AuditEntry {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]AuditEntry, len(a.entries))
	copy(out, a.entries)
	return out
}

func (s *Server) record(r *http.Request, action, detail string) {
	if s.audit == nil {
		return
	}
	s.audit.Record(action, s.clientIP(r), detail)
}

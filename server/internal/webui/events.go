package webui

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"
)

// Snapshot is the compact live state sent over SSE.
type Snapshot struct {
	Groups    []SnapshotGroup `json:"groups"`
	Live      string          `json:"live"`
	Approval  string          `json:"approval"`
	HostNote  string          `json:"note"`
	Connected int             `json:"connected"`
}

// SnapshotGroup is one group's live counts.
type SnapshotGroup struct {
	ID        string           `json:"id"`
	Live      bool             `json:"live"`
	Connected int              `json:"connected"`
	Members   []SnapshotMember `json:"members,omitempty"`
}

// SnapshotMember is one member's live state.
type SnapshotMember struct {
	ID   string `json:"id"`
	Live bool   `json:"live"`
}

// NewSnapshot builds a snapshot from the engine.
func NewSnapshot(a App) Snapshot {
	liveID := a.LiveGroupID()
	presence := a.HostPresence()
	snap := Snapshot{Live: liveID, HostNote: a.HostNote()}
	for _, p := range presence {
		if p.Live {
			snap.Connected++
		}
	}
	if approval := a.Approval(); approval != nil {
		snap.Approval = approval.ID
	}
	for _, g := range hostGroups(a.Groups()) {
		group := SnapshotGroup{ID: g.ID, Live: liveID == g.ID}
		for _, m := range g.Members {
			p := presence[m.ID]
			group.Members = append(group.Members, SnapshotMember{ID: m.ID, Live: p.Live})
			if p.Live {
				group.Connected++
			}
		}
		snap.Groups = append(snap.Groups, group)
	}
	return snap
}

// Hash is a stable digest of the snapshot's meaningful state.
func (s Snapshot) Hash() string {
	data, err := json.Marshal(s)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request, session *Session) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.renderError(w, r, http.StatusInternalServerError, "Live updates are not available.")
		return
	}
	// SSE outlives the server's normal write deadline.
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")

	last := ""
	send := func() bool {
		snap := NewSnapshot(s.app)
		hash := snap.Hash()
		if hash == last {
			return true
		}
		data, err := json.Marshal(snap)
		if err != nil {
			return false
		}
		if _, err := w.Write([]byte("data: " + string(data) + "\n\n")); err != nil {
			return false
		}
		flusher.Flush()
		last = hash
		return true
	}

	if !send() {
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if !send() {
				return
			}
		}
	}
}

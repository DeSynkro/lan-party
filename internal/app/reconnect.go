package app

import (
	"context"
	"errors"
	"time"

	"github.com/desynkro/lan-party/internal/group"
	"github.com/desynkro/lan-party/internal/session"
	"github.com/desynkro/lan-party/internal/tunnel"
)

const (
	reconnectFastUntil      = 10 * time.Minute
	reconnectStopAfter      = 30 * time.Minute
	reconnectAttemptTimeout = 10 * time.Second
)

var reconnectInitialDelays = []time.Duration{
	time.Second,
	2 * time.Second,
	5 * time.Second,
	10 * time.Second,
	15 * time.Second,
	30 * time.Second,
}

// reconnectDelay returns the wait before the next attempt. Attempt zero is the
// first retry, one second after the drop.
func reconnectDelay(attempt int, elapsed time.Duration) (time.Duration, bool) {
	if elapsed >= reconnectStopAfter {
		return 0, false
	}
	var delay time.Duration
	switch {
	case attempt < len(reconnectInitialDelays):
		delay = reconnectInitialDelays[attempt]
	case elapsed < reconnectFastUntil:
		delay = 30 * time.Second
	default:
		delay = time.Minute
	}
	if remaining := reconnectStopAfter - elapsed; delay > remaining {
		delay = remaining
	}
	return delay, true
}

// ReconnectStatus is what the friend screen needs to show the retry state.
type ReconnectStatus struct {
	Active  bool
	Stopped bool
	Next    time.Time
	Reason  string
}

type reconnectState struct {
	groupID string
	started time.Time
	next    time.Time
	attempt int
	stopped bool
	reason  string
	cancel  context.CancelFunc
}

type permanentReconnectError struct {
	err error
}

func (e *permanentReconnectError) Error() string { return e.err.Error() }
func (e *permanentReconnectError) Unwrap() error { return e.err }

func permanentReconnect(err error) error {
	return &permanentReconnectError{err: err}
}

func isPermanentReconnect(err error) bool {
	var target *permanentReconnectError
	return errors.As(err, &target) || errors.Is(err, tunnel.ErrDenied)
}

// ReconnectStatus returns the retry state for one friend group.
func (a *App) ReconnectStatus(groupID string) ReconnectStatus {
	a.mu.Lock()
	defer a.mu.Unlock()

	state := a.autoReconnect
	if state == nil || state.groupID != groupID {
		return ReconnectStatus{}
	}
	return ReconnectStatus{
		Active:  state.cancel != nil,
		Stopped: state.stopped,
		Next:    state.next,
		Reason:  state.reason,
	}
}

// stopAutoReconnect cancels the retry loop and clears its status.
func (a *App) stopAutoReconnect() {
	a.mu.Lock()
	a.stopAutoReconnectLocked()
	a.mu.Unlock()
}

func (a *App) stopAutoReconnectLocked() {
	if a.autoReconnect == nil {
		return
	}
	if a.autoReconnect.cancel != nil {
		a.autoReconnect.cancel()
	}
	a.autoReconnect = nil
}

// startAutoReconnect starts a fresh retry schedule for an offline friend.
func (a *App) startAutoReconnect(groupID string, started time.Time) {
	a.mu.Lock()
	a.startAutoReconnectLocked(groupID, started)
	a.mu.Unlock()
}

func (a *App) startAutoReconnectLocked(groupID string, started time.Time) {
	if !a.settings.AutoReconnect || a.host != nil {
		return
	}
	idx := a.groupIndexLocked(groupID)
	if idx < 0 || a.groups[idx].Role != group.RoleFriend {
		return
	}
	if a.friend != nil {
		if a.friend.Presence().Live || a.friend.Revoked() {
			return
		}
	}
	if started.IsZero() {
		started = time.Now()
	}

	a.stopAutoReconnectLocked()
	ctx, cancel := context.WithCancel(context.Background())
	state := &reconnectState{
		groupID: groupID,
		started: started,
		cancel:  cancel,
	}
	a.autoReconnect = state
	go a.runAutoReconnect(ctx, state)
}

func (a *App) runAutoReconnect(ctx context.Context, state *reconnectState) {
	for {
		a.mu.Lock()
		if a.autoReconnect != state {
			a.mu.Unlock()
			return
		}
		delay, ok := reconnectDelay(state.attempt, time.Since(state.started))
		if !ok {
			a.finishAutoReconnectLocked(state, "Auto-reconnect stopped after 30 minutes.")
			a.mu.Unlock()
			return
		}
		state.next = time.Now().Add(delay)
		a.mu.Unlock()

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		a.mu.Lock()
		if a.autoReconnect != state {
			a.mu.Unlock()
			return
		}
		state.next = time.Time{}
		a.mu.Unlock()

		attemptCtx, cancel := context.WithTimeout(ctx, reconnectAttemptTimeout)
		a.reconnectMu.Lock()
		err := a.reconnect(attemptCtx, state.groupID)
		a.reconnectMu.Unlock()
		cancel()

		a.mu.Lock()
		if a.autoReconnect != state {
			a.mu.Unlock()
			return
		}
		state.attempt++
		if err == nil {
			a.autoReconnect = nil
			a.mu.Unlock()
			return
		}
		if isPermanentReconnect(err) || ctx.Err() != nil {
			reason := ""
			switch {
			case isPermanentReconnect(err):
				reason = "Auto-reconnect stopped: " + err.Error()
			case ctx.Err() != nil:
				reason = "Auto-reconnect stopped."
			}
			a.finishAutoReconnectLocked(state, reason)
			a.mu.Unlock()
			return
		}
		a.mu.Unlock()
	}
}

func (a *App) finishAutoReconnectLocked(state *reconnectState, reason string) {
	state.stopped = true
	state.reason = reason
	state.next = time.Time{}
	if state.cancel != nil {
		state.cancel()
		state.cancel = nil
	}
}

// watchFriend starts the retry loop if this friend drops unexpectedly.
func (a *App) watchFriend(friend *session.Friend, groupID string) {
	<-friend.Done()

	a.mu.Lock()
	if a.friend != friend || a.liveGroupID != groupID {
		a.mu.Unlock()
		return
	}
	if friend.Revoked() || !a.settings.AutoReconnect {
		a.mu.Unlock()
		return
	}
	a.startAutoReconnectLocked(groupID, time.Now())
	a.mu.Unlock()
}

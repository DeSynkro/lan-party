package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestApprovalQueueIsCapped(t *testing.T) {
	a := newTestApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	var wg sync.WaitGroup
	for i := 0; i < maxPendingApprovals; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = a.requestApproval(ctx, Approval{PeerName: fmt.Sprintf("peer-%d", i)})
		}(i)
	}

	waitFor(t, 2*time.Second, func() bool {
		a.mu.Lock()
		defer a.mu.Unlock()
		return len(a.pending) == maxPendingApprovals
	})

	if _, err := a.requestApproval(context.Background(), Approval{PeerName: "extra"}); !errors.Is(err, errTooManyApprovals) {
		t.Fatalf("extra approval error = %v, want %v", err, errTooManyApprovals)
	}

	for {
		a.mu.Lock()
		if len(a.pending) == 0 {
			a.mu.Unlock()
			break
		}
		id := a.pending[0].ID
		a.mu.Unlock()
		if !a.ResolveApproval(id, false) {
			t.Fatal("the approval could not be resolved")
		}
	}
	wg.Wait()
}

func TestAttemptLimiterCapsSourceMap(t *testing.T) {
	l := newAttemptLimiter()
	now := time.Now()
	for i := 0; i < maxAttemptIPs*2; i++ {
		ip := fmt.Sprintf("198.51.%d.%d", (i/256)%256, i%256)
		l.Fail(ip, now)
	}

	l.mu.Lock()
	count := len(l.byIP)
	l.mu.Unlock()
	if count > maxAttemptIPs {
		t.Fatalf("limiter tracks %d sources, want at most %d", count, maxAttemptIPs)
	}
}

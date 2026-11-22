package webui

import (
	"strconv"
	"testing"
	"time"
)

func TestLoginLimiter(t *testing.T) {
	l := NewLoginLimiter()
	now := time.Now()
	for i := 0; i < 5; i++ {
		l.Fail("1.2.3.4", now)
	}
	ok, wait := l.Allow("1.2.3.4", now)
	if ok {
		t.Fatal("limiter did not block")
	}
	if wait <= 0 {
		t.Fatal("no retry delay")
	}
	l.Success("1.2.3.4")
	if ok, _ := l.Allow("1.2.3.4", now); !ok {
		t.Fatal("success did not reset")
	}
}

func TestLoginLimiterExpires(t *testing.T) {
	l := NewLoginLimiter()
	now := time.Now()
	for i := 0; i < 5; i++ {
		l.Fail("1.2.3.4", now)
	}
	if ok, _ := l.Allow("1.2.3.4", now.Add(time.Hour)); !ok {
		t.Fatal("block did not expire")
	}
}

func TestLoginLimiterIsolatesAddresses(t *testing.T) {
	l := NewLoginLimiter()
	now := time.Now()
	for i := 0; i < 5; i++ {
		l.Fail("1.2.3.4", now)
	}
	if ok, _ := l.Allow("5.6.7.8", now); !ok {
		t.Fatal("one address blocked another")
	}
}

func TestLoginLimiterGlobalCap(t *testing.T) {
	l := NewLoginLimiter()
	now := time.Now()
	for i := 0; i < 100; i++ {
		l.Fail("10.0.0."+strconv.Itoa(i), now)
	}
	if ok, _ := l.Allow("192.0.2.1", now); ok {
		t.Fatal("global cap not applied")
	}
}

func TestLoginLimiterBackoffGrows(t *testing.T) {
	l := NewLoginLimiter()
	now := time.Now()
	for i := 0; i < 5; i++ {
		l.Fail("1.2.3.4", now)
	}
	_, first := l.Allow("1.2.3.4", now)
	l.Fail("1.2.3.4", now)
	_, second := l.Allow("1.2.3.4", now)
	if second <= first {
		t.Fatalf("backoff did not grow: %v then %v", first, second)
	}
}

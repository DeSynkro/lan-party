package webui

import (
	"strings"
	"testing"
)

func TestAuditRingIsBoundedAndRedacts(t *testing.T) {
	a := NewAuditLog(2)
	a.Record("login", "1.2.3.4", "ok")
	a.Record("invite", "1.2.3.4", "created")
	a.Record("settings", "1.2.3.4", "saved")
	if len(a.Recent()) != 2 {
		t.Fatal("ring not bounded")
	}
	for _, entry := range a.Recent() {
		if strings.Contains(entry.Detail, "invite-code") {
			t.Fatal("secret logged")
		}
	}
	if a.Recent()[0].Action != "settings" {
		t.Fatal("entries are not newest first")
	}
}

func TestAuditCopiesEntries(t *testing.T) {
	a := NewAuditLog(4)
	a.Record("login", "1.2.3.4", "ok")
	got := a.Recent()
	if len(got) != 1 {
		t.Fatal("entry missing")
	}
	got[0].Action = "changed"
	if a.Recent()[0].Action != "login" {
		t.Fatal("Recent exposes the internal slice")
	}
}

func TestGuideRenders(t *testing.T) {
	srv := newTestServer(t)
	rr := srv.get("/guide")
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "Forward UDP") {
		t.Fatalf("guide missing: %d", rr.Code)
	}
}

func TestSettingsShowsRecentActivity(t *testing.T) {
	srv := newTestServer(t)
	srv.audit.Record("login", "203.0.113.7", "ok")
	rr := srv.get("/settings")
	body := rr.Body.String()
	if !strings.Contains(body, "Recent activity") {
		t.Fatal("activity section missing")
	}
	if !strings.Contains(body, "login") {
		t.Fatal("audit entry not rendered")
	}
	if !strings.Contains(body, "203.0.113.7") {
		t.Fatal("audit IP not rendered")
	}
}

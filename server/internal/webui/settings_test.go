package webui

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/desynkro/lan-party/internal/group"
)

func TestSavePublicAddress(t *testing.T) {
	srv := newTestServer(t)
	rr := srv.post("/settings", url.Values{"publicAddress": {"192.168.1.20"}, "approveNewDevices": {"on"}, "forgetDays": {"5"}})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rr.Code)
	}
	if srv.fake.settings.PublicAddress != "192.168.1.20" {
		t.Fatal("address not saved")
	}
	if !srv.fake.settings.ApproveNewDevices || srv.fake.settings.ForgetFriendsDays != 5 {
		t.Fatalf("settings: %+v", srv.fake.settings)
	}
}

func TestAppPortIsReadOnly(t *testing.T) {
	srv := newTestServer(t)
	rr := srv.get("/settings")
	body := rr.Body.String()
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	if !strings.Contains(body, "managed by this deployment") {
		t.Fatal("missing deployment note")
	}
	if strings.Contains(body, `name="appPort"`) {
		t.Fatal("app port is editable")
	}
}

func TestSettingsRejectInvalidForgetDays(t *testing.T) {
	srv := newTestServer(t)
	rr := srv.post("/settings", url.Values{"publicAddress": {""}, "forgetDays": {"999"}})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestSettingsBlankForgetDaysMeansNever(t *testing.T) {
	srv := newTestServer(t)
	rr := srv.post("/settings", url.Values{"publicAddress": {""}, "forgetDays": {""}})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rr.Code)
	}
	if srv.fake.settings.ForgetFriendsDays != 0 {
		t.Fatalf("days = %d", srv.fake.settings.ForgetFriendsDays)
	}
}

func TestAddressChangeWithOpenInvitesNeedsConfirmation(t *testing.T) {
	srv := newTestServerWithGroups(t, group.Group{ID: "g1", Name: "Friday", Role: group.RoleHost,
		Invites: []group.Invite{{ID: "i1", Name: "Ash", TokenHash: "hash", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}},
	})
	srv.fake.settings.PublicAddress = "old.example.com"
	rr := srv.post("/settings", url.Values{"publicAddress": {"new.example.com"}, "forgetDays": {"5"}})
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "Change the public address") {
		t.Fatalf("confirmation missing: %d", rr.Code)
	}
	if srv.fake.settings.PublicAddress != "old.example.com" {
		t.Fatal("address changed without confirmation")
	}
	rr = srv.post("/settings", url.Values{
		"publicAddress": {"new.example.com"}, "forgetDays": {"5"}, "confirmAddress": {"1"},
	})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("confirm status = %d", rr.Code)
	}
	if srv.fake.settings.PublicAddress != "new.example.com" {
		t.Fatal("confirmed address not saved")
	}
}

func TestPasswordChangeInvalidatesSessions(t *testing.T) {
	srv := newTestServer(t)
	rr := srv.post("/settings/password", url.Values{
		"current": {testPassword}, "new": {"a new long password"}, "confirm": {"a new long password"},
	})
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rr.Code)
	}
	if !srv.sessions.Empty() {
		t.Fatal("sessions survived password change")
	}
	if !srv.creds.Verify("a new long password") {
		t.Fatal("new password not stored")
	}
}

func TestPasswordChangeRejectsWrongCurrent(t *testing.T) {
	srv := newTestServer(t)
	rr := srv.post("/settings/password", url.Values{
		"current": {"wrong password"}, "new": {"a new long password"}, "confirm": {"a new long password"},
	})
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rr.Code)
	}
	if !srv.creds.Verify(testPassword) {
		t.Fatal("password changed without the current password")
	}
}

func TestPasswordChangeRejectsShortPassword(t *testing.T) {
	srv := newTestServer(t)
	rr := srv.post("/settings/password", url.Values{
		"current": {testPassword}, "new": {"short"}, "confirm": {"short"},
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
	if !srv.creds.Verify(testPassword) {
		t.Fatal("password changed to a short one")
	}
}

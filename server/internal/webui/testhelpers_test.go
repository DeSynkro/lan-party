package webui

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/desynkro/lan-party/internal/app"
	"github.com/desynkro/lan-party/internal/config"
	"github.com/desynkro/lan-party/internal/group"
	"github.com/desynkro/lan-party/internal/session"
)

const testPassword = "test-password-123"

type fakeApp struct {
	groups     []group.Group
	settings   config.Settings
	live       string
	presence   map[string]session.Presence
	note       string
	approval   *app.Approval
	resolved   bool
	allowed    bool
	lastInvite string
	started    bool
	dataDir    string
	deviceName string
	groupErr   error
	sessionErr error
	inviteErr  error
	memberErr  error
}

func newFakeApp() *fakeApp {
	return &fakeApp{
		settings:   config.DefaultSettings(),
		presence:   map[string]session.Presence{},
		deviceName: "Server",
	}
}

func (f *fakeApp) Groups() []group.Group { return f.groups }

func (f *fakeApp) Group(id string) (group.Group, bool) {
	for _, g := range f.groups {
		if g.ID == id {
			return g, true
		}
	}
	return group.Group{}, false
}

func (f *fakeApp) Settings() config.Settings { return f.settings }

func (f *fakeApp) SaveSettings(s config.Settings) error {
	f.settings = s
	return nil
}

func (f *fakeApp) SetPublicAddress(value string) error {
	f.settings.PublicAddress = value
	return nil
}

func (f *fakeApp) LiveGroupID() string { return f.live }

func (f *fakeApp) HostPresence() map[string]session.Presence { return f.presence }

func (f *fakeApp) HostNote() string { return f.note }

func (f *fakeApp) Approval() *app.Approval { return f.approval }

func (f *fakeApp) ResolveApproval(id string, allow bool) bool {
	f.resolved = true
	f.allowed = allow
	f.approval = nil
	return true
}

func (f *fakeApp) CreateGroup(name, hostName string) (group.Group, error) {
	if f.groupErr != nil {
		return group.Group{}, f.groupErr
	}
	g := group.CreateHostGroup(name, hostName)
	f.groups = append(f.groups, g)
	return g, nil
}

func (f *fakeApp) DeleteGroup(id string) error {
	for i, g := range f.groups {
		if g.ID == id {
			f.groups = append(f.groups[:i], f.groups[i+1:]...)
			return nil
		}
	}
	return nil
}

func (f *fakeApp) AddService(groupID string, s group.Service) error {
	for i := range f.groups {
		if f.groups[i].ID == groupID {
			f.groups[i].Services = append(f.groups[i].Services, s)
			return nil
		}
	}
	return nil
}

func (f *fakeApp) UpdateService(groupID string, s group.Service) error {
	for i := range f.groups {
		if f.groups[i].ID != groupID {
			continue
		}
		for j := range f.groups[i].Services {
			if f.groups[i].Services[j].ID == s.ID {
				f.groups[i].Services[j] = s
				return nil
			}
		}
	}
	return errors.New("service not found")
}

func (f *fakeApp) RemoveService(groupID, serviceID string) error {
	for i := range f.groups {
		if f.groups[i].ID != groupID {
			continue
		}
		for j, s := range f.groups[i].Services {
			if s.ID == serviceID {
				f.groups[i].Services = append(f.groups[i].Services[:j], f.groups[i].Services[j+1:]...)
				return nil
			}
		}
		return errors.New("service not found")
	}
	return errors.New("group not found")
}

func (f *fakeApp) CreateInvite(groupID, peerName string) (string, error) {
	if f.inviteErr != nil {
		return "", f.inviteErr
	}
	f.lastInvite = "invite-code-1"
	for i := range f.groups {
		if f.groups[i].ID == groupID {
			f.groups[i].Invites = append(f.groups[i].Invites, group.Invite{
				ID: "invite-1", Name: peerName,
				CreatedAt: time.Now(),
				ExpiresAt: time.Now().Add(time.Hour),
			})
			f.live = groupID
			f.started = true
		}
	}
	return f.lastInvite, nil
}

func (f *fakeApp) InviteCode(inviteID string) string { return f.lastInvite }

func (f *fakeApp) RemoveInvite(groupID, inviteID string) error {
	for i := range f.groups {
		if f.groups[i].ID != groupID {
			continue
		}
		kept := f.groups[i].Invites[:0]
		for _, inv := range f.groups[i].Invites {
			if inv.ID != inviteID {
				kept = append(kept, inv)
			}
		}
		f.groups[i].Invites = kept
	}
	return nil
}

func (f *fakeApp) StartSession(groupID string) error {
	if f.sessionErr != nil {
		return f.sessionErr
	}
	f.started = true
	f.live = groupID
	return nil
}

func (f *fakeApp) StopSession() error {
	f.started = false
	f.live = ""
	return nil
}

func (f *fakeApp) RemoveMember(groupID, memberID string) error {
	if f.memberErr != nil {
		return f.memberErr
	}
	for i := range f.groups {
		if f.groups[i].ID != groupID {
			continue
		}
		kept := f.groups[i].Members[:0]
		for _, m := range f.groups[i].Members {
			if m.ID != memberID {
				kept = append(kept, m)
			}
		}
		f.groups[i].Members = kept
	}
	return nil
}

func (f *fakeApp) GroupEndpoint(groupID string) (string, error) {
	return "192.168.1.20:51820", nil
}

func (f *fakeApp) GroupAppPort(groupID string) (int, error) { return 51820, nil }

func (f *fakeApp) DeviceName() string { return f.deviceName }

func (f *fakeApp) DataDir() string { return f.dataDir }

func (f *fakeApp) ConfigDir() string { return f.dataDir }

func (f *fakeApp) Close() error { return nil }

type testServer struct {
	*Server
	fake    *fakeApp
	session *Session
	t       *testing.T
}

func newTestServer(t *testing.T) *testServer {
	return newTestServerWithGroups(t)
}

func newTestServerWithGroups(t *testing.T, groups ...group.Group) *testServer {
	t.Helper()
	dataDir := t.TempDir()
	if err := SetPassword(CredentialsPath(dataDir), testPassword); err != nil {
		t.Fatal(err)
	}
	fake := newFakeApp()
	fake.groups = groups
	fake.dataDir = dataDir
	srv, err := New(fake, Config{DataDir: dataDir, Listen: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	srv.loginDelay = 0
	session, err := srv.sessions.Create()
	if err != nil {
		t.Fatal(err)
	}
	return &testServer{Server: srv, fake: fake, session: session, t: t}
}

func (ts *testServer) do(req *http.Request) *httptest.ResponseRecorder {
	ts.t.Helper()
	rr := httptest.NewRecorder()
	ts.Handler().ServeHTTP(rr, req)
	return rr
}

func (ts *testServer) get(path string) *httptest.ResponseRecorder {
	ts.t.Helper()
	req := httptest.NewRequest("GET", "http://lan.local"+path, nil)
	req.Host = "lan.local"
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: ts.session.ID})
	return ts.do(req)
}

func (ts *testServer) post(path string, values url.Values) *httptest.ResponseRecorder {
	ts.t.Helper()
	if values == nil {
		values = url.Values{}
	}
	if values.Get("csrf") == "" {
		values.Set("csrf", ts.session.CSRF)
	}
	req := httptest.NewRequest("POST", "http://lan.local"+path, strings.NewReader(values.Encode()))
	req.Host = "lan.local"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://lan.local")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: ts.session.ID})
	return ts.do(req)
}

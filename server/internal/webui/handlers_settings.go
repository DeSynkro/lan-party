package webui

import (
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/desynkro/lan-party/internal/buildinfo"
)

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request, session *Session) {
	s.render(w, r, http.StatusOK, "settings.html", s.settingsView(r))
}

func (s *Server) settingsView(r *http.Request) SettingsView {
	settings := s.app.Settings()
	return SettingsView{
		BaseView:           s.base(r, "Settings"),
		PublicAddress:      settings.PublicAddress,
		AppPort:            settings.AppPort,
		ApproveNewDevices:  settings.ApproveNewDevices,
		ForgetDays:         settings.ForgetFriendsDays,
		DataDir:            s.cfg.DataDir,
		LogPath:            filepath.Join(s.cfg.DataDir, "lan-party.log"),
		Version:            buildinfo.Version,
		AddressSuggestions: lanAddresses(),
		Activity:           s.audit.Recent(),
	}
}

func (s *Server) handleSettingsSave(w http.ResponseWriter, r *http.Request, session *Session) {
	if err := r.ParseForm(); err != nil {
		s.renderError(w, r, http.StatusBadRequest, "The form could not be read.")
		return
	}
	days, err := parseForgetDays(r.PostFormValue("forgetDays"))
	if err != nil {
		view := s.settingsView(r)
		view.Error = err.Error()
		s.render(w, r, http.StatusBadRequest, "settings.html", view)
		return
	}
	settings := s.app.Settings()
	publicAddress := strings.TrimSpace(r.PostFormValue("publicAddress"))
	approve := r.PostFormValue("approveNewDevices") != ""

	if r.PostFormValue("confirmAddress") == "" && publicAddress != settings.PublicAddress && s.openInviteCount() > 0 {
		view := s.settingsView(r)
		view.PublicAddress = publicAddress
		view.ApproveNewDevices = approve
		view.ForgetDays = days
		base := view.BaseView
		base.Title = "Change public address"
		view.BaseView = base
		s.render(w, r, http.StatusOK, "confirm.html", confirmView{
			BaseView: view.BaseView,
			Eyebrow:  "Open invites",
			Heading:  "Change the public address?",
			Message:  "There are open invites. They point at the old address and will stop working. New invites will use the new address.",
			Action:   "/settings",
			Confirm:  "Change address",
			Cancel:   "/settings",
			Hidden: map[string]string{
				"publicAddress":     publicAddress,
				"confirmAddress":    "1",
				"forgetDays":        strconv.Itoa(days),
				"approveNewDevices": approveValue(approve),
			},
		})
		return
	}

	settings.PublicAddress = publicAddress
	settings.ApproveNewDevices = approve
	settings.ForgetFriendsDays = days
	if err := s.app.SaveSettings(settings); err != nil {
		view := s.settingsView(r)
		view.PublicAddress = publicAddress
		view.ApproveNewDevices = approve
		view.ForgetDays = days
		base := view.BaseView
		base.Error = err.Error()
		view.BaseView = base
		s.render(w, r, http.StatusBadRequest, "settings.html", view)
		return
	}
	s.record(r, "settings", "saved")
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (s *Server) handlePasswordForm(w http.ResponseWriter, r *http.Request, session *Session) {
	s.render(w, r, http.StatusOK, "password.html", s.base(r, "Change password"))
}

func (s *Server) handlePasswordChange(w http.ResponseWriter, r *http.Request, session *Session) {
	if err := r.ParseForm(); err != nil {
		s.renderError(w, r, http.StatusBadRequest, "The form could not be read.")
		return
	}
	current := r.PostFormValue("current")
	next := r.PostFormValue("new")
	confirm := r.PostFormValue("confirm")

	fail := func(message string, status int) {
		base := s.base(r, "Change password")
		base.Error = message
		s.render(w, r, status, "password.html", base)
	}
	if !s.creds.Verify(current) {
		fail("The current password is not correct.", http.StatusUnauthorized)
		return
	}
	if next != confirm {
		fail("The new passwords do not match.", http.StatusBadRequest)
		return
	}
	path := CredentialsPath(s.cfg.DataDir)
	if err := SetPassword(path, next); err != nil {
		fail(err.Error(), http.StatusBadRequest)
		return
	}
	creds, err := LoadCredentials(path)
	if err != nil {
		s.renderError(w, r, http.StatusInternalServerError, "The password was changed but could not be reloaded.")
		return
	}
	s.creds = creds
	s.sessions.DestroyAll()
	s.record(r, "password", "changed")
	ClearSessionCookie(w, s.secureRequest(r))
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func parseForgetDays(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	days, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("enter a number of days, or leave it blank to keep everyone")
	}
	if days < 0 || days > 365 {
		return 0, fmt.Errorf("the inactive friend limit must be between 0 and 365 days")
	}
	return days, nil
}

func (s *Server) openInviteCount() int {
	now := time.Now()
	n := 0
	for _, g := range s.app.Groups() {
		for _, inv := range g.Invites {
			if inv.Open(now) {
				n++
			}
		}
	}
	return n
}

// lanAddresses lists private IPv4 addresses as suggestions for the public
// address field.
func lanAddresses() []string {
	var out []string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		ip := ipNet.IP.To4()
		if ip == nil || !ip.IsPrivate() {
			continue
		}
		out = append(out, ip.String())
	}
	return out
}

func approveValue(approve bool) string {
	if approve {
		return "on"
	}
	return ""
}

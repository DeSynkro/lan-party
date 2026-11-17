package gtkui

import (
	"context"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	apppkg "github.com/desynkro/lan-party/internal/app"
)

func (u *probeUI) showCreateGroupDialog() {
	win, content := u.dialog("Create a group")
	content.Append(textLabel("Group name", "form-label"))
	groupEntry := entry("LAN Party")
	content.Append(groupEntry)
	content.Append(gapBox(10))
	content.Append(textLabel("Your name", "form-label"))
	hostEntry := entry("User")
	content.Append(hostEntry)
	content.Append(gapBox(10))
	errLabel := textLabel("", "error")
	errLabel.SetVisible(false)
	content.Append(errLabel)
	content.Append(gapBox(10))

	buttons := gtk.NewBox(gtk.OrientationHorizontal, 8)
	cancel := gtk.NewButtonWithLabel("Cancel")
	cancel.AddCSSClass("quiet")
	cancel.ConnectClicked(func() { win.Close() })
	create := gtk.NewButtonWithLabel("Create group")
	create.AddCSSClass("primary")
	create.ConnectClicked(func() {
		name := strings.TrimSpace(groupEntry.Text())
		host := strings.TrimSpace(hostEntry.Text())
		if host == "" {
			host = "User"
		}
		if name == "" {
			errLabel.SetText("Give the group a name.")
			errLabel.SetVisible(true)
			return
		}
		g, err := u.state.CreateGroup(name, host)
		if err != nil {
			errLabel.SetText(err.Error())
			errLabel.SetVisible(true)
			return
		}
		win.Close()
		u.showHome()
		u.showGroup(g.ID)
	})
	buttons.Append(cancel)
	buttons.Append(create)
	buttons.SetHAlign(gtk.AlignEnd)
	content.Append(buttons)
	groupEntry.GrabFocus()
	win.Present()
}

func (u *probeUI) showJoinDialog() {
	win, content := u.dialog("Join with an invite")
	content.Append(textLabel("Invite code", "form-label"))
	codeEntry := entry("lp1_…")
	content.Append(codeEntry)
	content.Append(gapBox(10))
	content.Append(textLabel("This device", "form-label"))
	deviceEntry := entry(u.state.DeviceName())
	content.Append(deviceEntry)
	content.Append(gapBox(10))
	status := textLabel("", "desc")
	status.SetVisible(false)
	errLabel := textLabel("", "error")
	errLabel.SetVisible(false)
	content.Append(status)
	content.Append(errLabel)
	content.Append(gapBox(10))

	buttons := gtk.NewBox(gtk.OrientationHorizontal, 8)
	cancel := gtk.NewButtonWithLabel("Cancel")
	cancel.AddCSSClass("quiet")
	join := gtk.NewButtonWithLabel("Join")
	join.AddCSSClass("primary")
	cancel.ConnectClicked(func() { win.Close() })
	join.ConnectClicked(func() {
		code := strings.TrimSpace(codeEntry.Text())
		device := strings.TrimSpace(deviceEntry.Text())
		if code == "" {
			errLabel.SetText("Paste the invite code first.")
			errLabel.SetVisible(true)
			return
		}
		if device == "" {
			device = u.state.DeviceName()
		}
		errLabel.SetVisible(false)
		status.SetText("Waiting for the host to approve this device…")
		status.SetVisible(true)
		join.SetSensitive(false)
		cancel.SetSensitive(false)
		go func() {
			g, err := u.state.JoinInvite(context.Background(), code, device)
			glib.IdleAdd(func() {
				if err != nil {
					status.SetVisible(false)
					errLabel.SetText(err.Error())
					errLabel.SetVisible(true)
					join.SetSensitive(true)
					cancel.SetSensitive(true)
					return
				}
				win.Close()
				u.showHome()
				u.showGroup(g.ID)
			})
		}()
	})
	buttons.Append(cancel)
	buttons.Append(join)
	buttons.SetHAlign(gtk.AlignEnd)
	content.Append(buttons)
	codeEntry.GrabFocus()
	win.Present()
}

func (u *probeUI) showApprovalDialog(approval apppkg.Approval) {
	u.reveal()
	win, content := u.dialog("Approve new device")
	headline := approval.PeerName + " wants to join"
	if approval.Kind == "new device" {
		headline = approval.PeerName + " has a new device"
	}
	content.Append(textLabel(headline, "card-title"))
	content.Append(gapBox(10))
	content.Append(textLabel("Device  "+approval.DeviceName, "desc"))
	content.Append(textLabel("Address  "+approval.Address, "desc"))
	content.Append(textLabel("Code  "+approval.Fingerprint, "desc"))
	content.Append(gapBox(10))
	content.Append(textLabel("Allow only if you recognise the device. The address is what the host sees, not proof of identity.", "desc"))
	content.Append(gapBox(12))

	buttons := gtk.NewBox(gtk.OrientationHorizontal, 8)
	deny := gtk.NewButtonWithLabel("Deny")
	deny.AddCSSClass("secondary")
	allow := gtk.NewButtonWithLabel("Allow")
	allow.AddCSSClass("primary")
	deny.ConnectClicked(func() {
		u.state.ResolveApproval(approval.ID, false)
		win.Close()
	})
	allow.ConnectClicked(func() {
		u.state.ResolveApproval(approval.ID, true)
		win.Close()
	})
	buttons.Append(deny)
	buttons.Append(allow)
	buttons.SetHAlign(gtk.AlignEnd)
	content.Append(buttons)
	win.Present()
}

func (u *probeUI) dialog(title string) (*gtk.Window, *gtk.Box) {
	win := gtk.NewWindow()
	win.SetTitle(title)
	win.SetTransientFor(&u.win.Window)
	win.SetModal(true)
	win.SetResizable(false)
	win.SetDefaultSize(360, 320)

	content := gtk.NewBox(gtk.OrientationVertical, 0)
	content.SetMarginTop(20)
	content.SetMarginBottom(20)
	content.SetMarginStart(20)
	content.SetMarginEnd(20)
	content.Append(textLabel(title, "card-title"))
	content.Append(gapBox(12))
	win.SetChild(content)
	return win, content
}

func entry(placeholder string) *gtk.Entry {
	e := gtk.NewEntry()
	e.SetPlaceholderText(placeholder)
	e.AddCSSClass("field")
	e.SetHExpand(true)
	return e
}

func (u *probeUI) watchApprovals() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		approval := u.state.Approval()
		if approval == nil {
			u.approvalShown = ""
			continue
		}
		if approval.ID == u.approvalShown {
			continue
		}
		u.approvalShown = approval.ID
		copied := *approval
		glib.IdleAdd(func() {
			u.showApprovalDialog(copied)
		})
	}
}

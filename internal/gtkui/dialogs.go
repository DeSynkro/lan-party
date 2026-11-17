package gtkui

import (
	"context"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	apppkg "github.com/desynkro/lan-party/internal/app"
	"github.com/desynkro/lan-party/internal/group"
)

func (u *probeUI) showCreateGroupDialog() {
	win, content := u.dialog("Create a group")
	content.Append(textLabel("Group name", "form-label"))
	groupEntry := entry("LAN Party")
	content.Append(groupEntry)
	content.Append(gapBox(10))
	content.Append(textLabel("Your name", "form-label"))
	hostEntry := entry("User")
	hostEntry.SetText(apppkg.DefaultHostName())
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

func (u *probeUI) showInviteDialog(g group.Group) {
	win, content := u.dialog("Invite a friend")
	content.Append(textLabel("Friend's name", "form-label"))
	nameEntry := entry("Ash")
	content.Append(nameEntry)
	content.Append(gapBox(10))
	errLabel := textLabel("", "error")
	errLabel.SetVisible(false)
	content.Append(errLabel)
	content.Append(gapBox(10))

	buttons := gtk.NewBox(gtk.OrientationHorizontal, 8)
	cancel := gtk.NewButtonWithLabel("Cancel")
	cancel.AddCSSClass("quiet")
	cancel.ConnectClicked(func() { win.Close() })
	create := gtk.NewButtonWithLabel("Create invite")
	create.AddCSSClass("primary")
	create.ConnectClicked(func() {
		name := strings.TrimSpace(nameEntry.Text())
		if name == "" {
			errLabel.SetText("Give the invite a name.")
			errLabel.SetVisible(true)
			return
		}
		code, err := u.state.CreateInvite(g.ID, name)
		if err != nil {
			errLabel.SetText(err.Error())
			errLabel.SetVisible(true)
			return
		}
		win.Close()
		u.showInviteCodeDialog(g, name, code)
	})
	buttons.Append(cancel)
	buttons.Append(create)
	buttons.SetHAlign(gtk.AlignEnd)
	content.Append(buttons)
	nameEntry.GrabFocus()
	win.Present()
}

func (u *probeUI) showInviteCodeDialog(g group.Group, name, code string) {
	win, content := u.dialog("Invite for " + name)
	codeLabel := textLabel(code, "mono")
	codeLabel.SetSizeRequest(320, -1)
	content.Append(codeLabel)
	content.Append(gapBox(10))
	content.Append(textLabel("Send this code privately. It works once and expires after 24 hours.", "desc"))
	content.Append(gapBox(12))

	buttons := gtk.NewBox(gtk.OrientationHorizontal, 8)
	copy := gtk.NewButtonWithLabel("Copy")
	copy.AddCSSClass("secondary")
	copy.ConnectClicked(func() {
		u.win.Widget.Clipboard().SetText(code)
		copy.SetLabel("Copied")
	})
	done := gtk.NewButtonWithLabel("Done")
	done.AddCSSClass("primary")
	done.ConnectClicked(func() {
		win.Close()
		u.showGroup(g.ID)
	})
	buttons.Append(copy)
	buttons.Append(done)
	buttons.SetHAlign(gtk.AlignEnd)
	content.Append(buttons)
	win.Present()
}

func (u *probeUI) showAddServiceDialog(g group.Group) {
	win, content := u.dialog("Publish a service")
	win.SetDefaultSize(360, 420)
	content.Append(textLabel("Service name", "form-label"))
	nameEntry := entry("Minecraft: Java Edition")
	content.Append(nameEntry)
	content.Append(gapBox(8))
	content.Append(textLabel("Protocol", "form-label"))
	protocolRow := gtk.NewBox(gtk.OrientationHorizontal, 6)
	protocolRow.SetHomogeneous(true)
	protocolButtons := make([]*gtk.Button, 0, 3)
	selectedProtocol := 0
	setProtocol := func(selected int) {
		selectedProtocol = selected
		for i, button := range protocolButtons {
			button.RemoveCSSClass("primary")
			button.RemoveCSSClass("secondary")
			if i == selected {
				button.AddCSSClass("primary")
			} else {
				button.AddCSSClass("secondary")
			}
		}
	}
	for i, label := range []string{"TCP", "UDP", "TCP + UDP"} {
		index := i
		button := gtk.NewButtonWithLabel(label)
		button.SetHExpand(true)
		button.ConnectClicked(func() { setProtocol(index) })
		protocolButtons = append(protocolButtons, button)
		protocolRow.Append(button)
	}
	setProtocol(0)
	content.Append(protocolRow)
	content.Append(gapBox(8))
	content.Append(textLabel("LAN target", "form-label"))
	targetEntry := entry("192.168.0.10:25565")
	content.Append(targetEntry)
	content.Append(gapBox(8))
	content.Append(textLabel("Friend ports", "form-label"))
	portsEntry := entry("25565")
	content.Append(portsEntry)
	content.Append(gapBox(8))
	content.Append(textLabel("Hint", "form-label"))
	hintEntry := entry("Direct Connect in Minecraft")
	content.Append(hintEntry)
	content.Append(gapBox(8))
	errLabel := textLabel("", "error")
	errLabel.SetVisible(false)
	content.Append(errLabel)
	content.Append(gapBox(10))

	buttons := gtk.NewBox(gtk.OrientationHorizontal, 8)
	cancel := gtk.NewButtonWithLabel("Cancel")
	cancel.AddCSSClass("quiet")
	cancel.ConnectClicked(func() { win.Close() })
	add := gtk.NewButtonWithLabel("Publish")
	add.AddCSSClass("primary")
	add.ConnectClicked(func() {
		ports, err := group.ParsePorts(portsEntry.Text())
		if err != nil {
			errLabel.SetText(err.Error())
			errLabel.SetVisible(true)
			return
		}
		s := group.Service{
			ID:     group.NewID(),
			Name:   strings.TrimSpace(nameEntry.Text()),
			Proto:  protoValue(selectedProtocol),
			Target: strings.TrimSpace(targetEntry.Text()),
			Ports:  ports,
			Hint:   strings.TrimSpace(hintEntry.Text()),
		}
		if err := u.state.AddService(g.ID, s); err != nil {
			errLabel.SetText(err.Error())
			errLabel.SetVisible(true)
			return
		}
		win.Close()
		u.showGroup(g.ID)
	})
	buttons.Append(cancel)
	buttons.Append(add)
	buttons.SetHAlign(gtk.AlignEnd)
	content.Append(buttons)
	nameEntry.GrabFocus()
	win.Present()
}

func (u *probeUI) confirm(title, message, confirmLabel string, onConfirm func()) {
	win, content := u.dialog(title)
	content.Append(cardText(message, "desc"))
	content.Append(gapBox(14))

	buttons := gtk.NewBox(gtk.OrientationHorizontal, 8)
	cancel := gtk.NewButtonWithLabel("Cancel")
	cancel.AddCSSClass("quiet")
	cancel.ConnectClicked(func() { win.Close() })
	confirm := gtk.NewButtonWithLabel(confirmLabel)
	confirm.AddCSSClass("danger")
	confirm.ConnectClicked(func() {
		win.Close()
		onConfirm()
	})
	buttons.Append(cancel)
	buttons.Append(confirm)
	buttons.SetHAlign(gtk.AlignEnd)
	content.Append(buttons)
	win.Present()
}

func (u *probeUI) alert(title, message string) {
	win, content := u.dialog(title)
	content.Append(cardText(message, "desc"))
	content.Append(gapBox(14))
	done := gtk.NewButtonWithLabel("Close")
	done.AddCSSClass("primary")
	done.ConnectClicked(func() { win.Close() })
	done.SetHAlign(gtk.AlignEnd)
	content.Append(done)
	win.Present()
}

func (u *probeUI) confirmDeleteGroup(g group.Group) {
	u.confirm("Delete "+g.Name+"?",
		"This ends any running session and revokes every member. Friends keep a saved entry until they remove it.",
		"Delete group",
		func() {
			if err := u.state.DeleteGroup(g.ID); err != nil {
				u.alert("Could not delete group", err.Error())
				return
			}
			u.showHome()
		})
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

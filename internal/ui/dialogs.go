package ui

import (
	"context"
	"errors"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	apppkg "github.com/desynkro/lan-party/internal/app"
	"github.com/desynkro/lan-party/internal/group"
	"github.com/desynkro/lan-party/internal/invite"
	"github.com/desynkro/lan-party/internal/preset"
)

func (u *UI) formDialog(title, confirm string, content fyne.CanvasObject, onConfirm func() error) {
	errLabel := widget.NewLabel("")
	errLabel.Importance = widget.DangerImportance
	body := container.NewVBox(content, gap(6), errLabel)

	d := dialog.NewCustomWithoutButtons(title, body, u.win)
	d.SetButtons([]fyne.CanvasObject{
		newButton("Cancel", buttonQuiet, d.Hide),
		newButton(confirm, buttonPrimary, func() {
			errLabel.SetText("")
			if err := onConfirm(); err != nil {
				errLabel.SetText(err.Error())
				return
			}
			d.Hide()
		}),
	})
	d.Show()
}

func (u *UI) newGroupDialog() {
	name := widget.NewEntry()
	name.SetText("Friday Night")
	host := widget.NewEntry()
	host.SetText(apppkg.DefaultHostName())

	form := widget.NewForm(
		widget.NewFormItem("Group name", name),
		widget.NewFormItem("Your name", host),
	)
	u.formDialog("Start a group", "Create", form, func() error {
		g, err := u.app.CreateGroup(name.Text, host.Text)
		if err != nil {
			return err
		}
		u.showGroup(g.ID)
		return nil
	})
}

func (u *UI) joinDialog() {
	code := widget.NewMultiLineEntry()
	code.SetPlaceHolder("Paste the invite code")
	code.Wrapping = fyne.TextWrapBreak
	code.TextStyle = fyne.TextStyle{Monospace: true}
	code.SetMinRowsVisible(3)

	device := widget.NewEntry()
	device.SetText(u.app.DeviceName())

	form := widget.NewForm(
		widget.NewFormItem("Invite code", code),
		widget.NewFormItem("This device", device),
	)
	content := container.NewVBox(
		form,
		mutedLabel("The host has to approve this device. The code works once and should stay private."),
	)

	u.formDialog("Join with an invite", "Join", content, func() error {
		raw := strings.TrimSpace(code.Text)
		if raw == "" {
			return errors.New("paste the invite code")
		}
		if _, err := invite.Decode(raw); err != nil {
			return err
		}

		var joinedID string
		u.runWaiting("Waiting for the host", "The host has to approve this device before the group opens.",
			func(ctx context.Context) error {
				g, err := u.app.JoinInvite(ctx, raw, device.Text)
				joinedID = g.ID
				return err
			},
			func(err error) {
				if ignoreCancel(err) {
					return
				}
				if err != nil {
					u.showError(err)
					return
				}
				u.showGroup(joinedID)
			})
		return nil
	})
}

func (u *UI) inviteDialog(g group.Group) {
	name := widget.NewEntry()
	name.SetPlaceHolder("Ash")
	form := widget.NewForm(widget.NewFormItem("Friend's name", name))
	u.formDialog("Invite a friend", "Create invite", form, func() error {
		code, err := u.app.CreateInvite(g.ID, name.Text)
		if err != nil {
			return err
		}
		u.showInviteCode(name.Text, code)
		u.showGroup(g.ID)
		return nil
	})
}

func (u *UI) showInviteCode(name, code string) {
	label := widget.NewLabel(code)
	label.Wrapping = fyne.TextWrapBreak
	label.TextStyle = fyne.TextStyle{Monospace: true}
	box := container.NewGridWrap(fyne.NewSize(320, 120), label)

	var copyButton *button
	copyButton = smallButton("Copy code", buttonSecondary, func() {
		u.copyText(code, copyButton)
	})

	content := container.NewVBox(
		bodyLabel("Send this to "+name+". It works once and expires in 24 hours."),
		gap(8),
		box,
		gap(8),
		container.NewCenter(copyButton),
	)
	dialog.NewCustom("Invite for "+name, "Done", content, u.win).Show()
}

func (u *UI) addServiceDialog(g group.Group) {
	presets := preset.All()
	names := []string{"Custom"}
	byName := make(map[string]preset.Preset, len(presets))
	for _, p := range presets {
		names = append(names, p.Name)
		byName[p.Name] = p
	}

	presetSelect := widget.NewSelect(names, nil)
	presetSelect.SetSelected("Custom")

	name := widget.NewEntry()
	name.SetPlaceHolder("Minecraft")
	proto := widget.NewSelect([]string{"TCP", "UDP", "TCP and UDP"}, nil)
	proto.SetSelected("TCP")
	target := widget.NewEntry()
	target.SetPlaceHolder("192.168.1.50:25565")
	ports := widget.NewEntry()
	ports.SetPlaceHolder("25565")
	hint := widget.NewEntry()
	hint.SetPlaceHolder("Direct Connect in Minecraft")

	presetSelect.OnChanged = func(value string) {
		p, ok := byName[value]
		if !ok {
			return
		}
		name.SetText(p.Name)
		proto.SetSelected(protoLabel(p.Proto))
		ports.SetText(group.FormatPorts(p.Ports))
		hint.SetText(p.Hint)
	}

	form := widget.NewForm(
		widget.NewFormItem("Preset", presetSelect),
		widget.NewFormItem("Name", name),
		widget.NewFormItem("Protocol", proto),
		widget.NewFormItem("On my LAN", target),
		widget.NewFormItem("Friends open", ports),
		widget.NewFormItem("Hint", hint),
	)
	help := mutedLabel("Use the IP and port the game server listens on. A bare IP publishes the same ports; an address with a port remaps one port.")

	u.formDialog("Publish a service", "Add service", container.NewVBox(form, help), func() error {
		parsedPorts, err := group.ParsePorts(ports.Text)
		if err != nil {
			return err
		}
		parsedTarget, _, err := group.ParseTarget(target.Text)
		if err != nil {
			return err
		}
		service := group.Service{
			Name:   name.Text,
			Proto:  protoValue(proto.Selected),
			Target: parsedTarget,
			Ports:  parsedPorts,
			Hint:   hint.Text,
		}
		if err := group.ValidateService(service); err != nil {
			return err
		}
		if err := u.app.AddService(g.ID, service); err != nil {
			return err
		}
		u.showGroup(g.ID)
		return nil
	})
}

func (u *UI) showServiceDetails(g group.Group, s group.Service) {
	content := container.NewVBox(
		nameText(s.Name),
		metaText(protoName(s.Proto)),
		gap(10),
		container.NewHBox(metaText("Friends open"), gapW(6), addressText("127.0.0.1:"+group.FormatPorts(s.Ports))),
		container.NewHBox(metaText("On your LAN"), gapW(6), addressMuted(s.Target)),
	)
	if s.Hint != "" {
		content.Add(gap(6))
		content.Add(mutedLabel(s.Hint))
	}
	for _, warning := range group.ServiceWarnings(s) {
		content.Add(warningLabel(warning))
	}

	var d *dialog.CustomDialog
	remove := newButton("Remove service", buttonDanger, func() {
		dialog.NewConfirm("Remove "+s.Name+"?",
			"Friends will no longer see this service.",
			func(ok bool) {
				if !ok {
					return
				}
				if err := u.app.RemoveService(g.ID, s.ID); err != nil {
					u.showError(err)
					return
				}
				if d != nil {
					d.Hide()
				}
				u.showGroup(g.ID)
			}, u.win).Show()
	})
	content.Add(gap(12))
	content.Add(remove)

	d = dialog.NewCustom("Service", "Done", content, u.win)
	d.Show()
}

func (u *UI) showMemberDetails(g group.Group, m group.Member) {
	status := "idle"
	if m.State != group.MemberApproved {
		status = "waiting for approval"
	} else if p, ok := u.app.HostPresence()[m.ID]; ok && p.Live {
		status = "live · " + p.RTT.Round(time.Millisecond).String()
	}

	content := container.NewVBox(
		nameText(m.Name),
		metaText(status),
		gap(10),
		metaText("Device  "+fallback(m.DeviceName, "not reported")),
		metaText("Device code  "+fallback(m.Fingerprint, "not recorded")),
		metaText("Last address  "+fallback(m.LastAddress, "not recorded")),
		metaText("First seen  "+when(m.FirstSeen)),
		metaText("Last seen  "+when(m.LastSeen)),
	)

	var d *dialog.CustomDialog
	remove := newButton("Remove device", buttonDanger, func() {
		dialog.NewConfirm("Remove "+m.Name+"?",
			"Their device key is deleted and their connection is closed. They will need a new invite to return.",
			func(ok bool) {
				if !ok {
					return
				}
				if err := u.app.RemoveMember(g.ID, m.ID); err != nil {
					u.showError(err)
					return
				}
				if d != nil {
					d.Hide()
				}
				u.showGroup(g.ID)
			}, u.win).Show()
	})
	content.Add(gap(12))
	content.Add(remove)

	d = dialog.NewCustom("Member", "Done", content, u.win)
	d.Show()
}

func (u *UI) showInviteDetails(g group.Group, inv group.Invite, code string) {
	content := container.NewVBox(
		nameText(inv.Name),
		metaText(expiresIn(inv.ExpiresAt)),
	)
	if code != "" {
		label := widget.NewLabel(code)
		label.Wrapping = fyne.TextWrapBreak
		label.TextStyle = fyne.TextStyle{Monospace: true}
		box := container.NewGridWrap(fyne.NewSize(320, 110), label)
		var copyButton *button
		copyButton = smallButton("Copy code", buttonSecondary, func() {
			u.copyText(code, copyButton)
		})
		content.Add(gap(8))
		content.Add(box)
		content.Add(container.NewCenter(copyButton))
	} else {
		content.Add(gap(8))
		content.Add(mutedLabel("The code was only shown when it was created. Create a new invite if you need to send it again."))
	}

	var d *dialog.CustomDialog
	remove := newButton("Cancel invite", buttonDanger, func() {
		dialog.NewConfirm("Cancel this invite?",
			"The code stops working immediately.",
			func(ok bool) {
				if !ok {
					return
				}
				if err := u.app.RemoveInvite(g.ID, inv.ID); err != nil {
					u.showError(err)
					return
				}
				if d != nil {
					d.Hide()
				}
				u.showGroup(g.ID)
			}, u.win).Show()
	})
	content.Add(gap(12))
	content.Add(remove)

	d = dialog.NewCustom("Invite", "Done", content, u.win)
	d.Show()
}

func fallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

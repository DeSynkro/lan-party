package gtkui

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	apppkg "github.com/desynkro/lan-party/internal/app"
	"github.com/desynkro/lan-party/internal/group"
	invitepkg "github.com/desynkro/lan-party/internal/invite"
	"github.com/desynkro/lan-party/internal/preset"
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

func (u *probeUI) showGroupConnectionDialog(g group.Group, showError func(error)) {
	settings := u.state.Settings()
	current, err := u.state.EffectiveConnection(g.ID)
	if err != nil {
		showError(err)
		return
	}
	settingsConnection, err := u.state.PreviewGroupConnection(g.ID, apppkg.GroupConnectionInput{})
	if err != nil {
		showError(err)
		return
	}
	hasPassword := u.state.GroupRelayPasswordSet(g.ID)
	live := func() bool { return u.state.LiveGroupID() == g.ID }

	win, content := u.dialog("Connection for " + g.Name)
	win.SetDefaultSize(400, 620)

	content.Append(textLabel("This changes only this group. The defaults live in Settings.", "help"))
	content.Append(gapBox(10))

	overrideCheck := gtk.NewCheckButtonWithLabel("Override Settings")
	overrideCheck.SetActive(g.OverrideSettings)
	content.Append(overrideCheck)
	content.Append(gapBox(8))

	summaryBox := gtk.NewBox(gtk.OrientationVertical, 0)
	summaryMode := textLabel("", "card-title")
	summaryRoute := textLabel("", "mono")
	summaryDetail := textLabel("", "desc")
	summaryBox.Append(summaryMode)
	summaryBox.Append(gapBox(4))
	summaryBox.Append(summaryRoute)
	summaryBox.Append(gapBox(4))
	summaryBox.Append(summaryDetail)
	content.Append(summaryBox)
	content.Append(gapBox(8))

	overrideBox := gtk.NewBox(gtk.OrientationVertical, 0)
	useRelayCheck := gtk.NewCheckButtonWithLabel("Use Relay")
	useRelayCheck.SetActive(g.UseRelay)
	overrideBox.Append(useRelayCheck)
	overrideBox.Append(gapBox(8))

	// Direct fields.
	publicPlaceholder := settings.PublicAddress
	if publicPlaceholder == "" {
		publicPlaceholder = "home.example.net"
	}
	publicEntry := entry(publicPlaceholder)
	publicEntry.SetText(g.PublicAddress)
	portPlaceholder := strconv.Itoa(settings.AppPort)
	portEntry := entry(portPlaceholder)
	if g.AppPort != 0 {
		portEntry.SetText(strconv.Itoa(g.AppPort))
	}
	directBox := gtk.NewBox(gtk.OrientationVertical, 0)
	directBox.Append(formRow("Public address", publicEntry))
	directBox.Append(gapBox(8))
	directBox.Append(formRow("App port (UDP)", portEntry))
	directBox.Append(gapBox(6))
	directBox.Append(textLabel("Blank fields use the Settings defaults.", "help"))

	// Relay fields.
	customCheck := gtk.NewCheckButtonWithLabel("Custom relay for this group")
	customCheck.SetActive(g.RelayCustom)
	relayHostEntry := entry(settings.RelayHost)
	relayHostEntry.SetText(g.RelayHost)
	relayPortEntry := entry(strconv.Itoa(settings.RelayPort))
	if g.RelayPort != 0 {
		relayPortEntry.SetText(strconv.Itoa(g.RelayPort))
	}
	relayIDEntry := entry(settings.RelayID)
	relayIDEntry.SetText(g.RelayID)
	passwordEntry := gtk.NewEntry()
	passwordEntry.SetVisibility(false)
	passwordEntry.SetInputPurpose(gtk.InputPurposePassword)
	passwordEntry.AddCSSClass("field")
	passwordEntry.SetHExpand(true)
	passwordStatus := textLabel("", "desc")

	customFields := gtk.NewBox(gtk.OrientationVertical, 0)
	customFields.Append(gapBox(8))
	customFields.Append(textLabel("Relay host", "form-label"))
	customFields.Append(relayHostEntry)
	customFields.Append(gapBox(8))
	customFields.Append(textLabel("Relay port", "form-label"))
	customFields.Append(relayPortEntry)
	customFields.Append(gapBox(8))
	customFields.Append(textLabel("Relay ID", "form-label"))
	customFields.Append(relayIDEntry)
	customFields.Append(gapBox(8))
	customFields.Append(textLabel("Relay password", "form-label"))
	customFields.Append(passwordEntry)
	customFields.Append(gapBox(4))
	customFields.Append(passwordStatus)

	inheritedNote := textLabel("", "desc")
	relayBox := gtk.NewBox(gtk.OrientationVertical, 0)
	relayBox.Append(customCheck)
	relayBox.Append(inheritedNote)
	relayBox.Append(customFields)

	overrideBox.Append(directBox)
	overrideBox.Append(relayBox)
	content.Append(overrideBox)
	content.Append(gapBox(10))

	preview := textLabel("", "mono")
	listen := textLabel("", "desc")
	warning := textLabel("", "warning")
	warning.SetVisible(false)
	endpointNote := textLabel("", "warning")
	endpointNote.SetVisible(false)
	errLabel := textLabel("", "error")
	errLabel.SetVisible(false)
	content.Append(preview)
	content.Append(gapBox(4))
	content.Append(listen)
	content.Append(gapBox(8))
	content.Append(warning)
	content.Append(gapBox(6))
	content.Append(endpointNote)
	content.Append(gapBox(6))
	content.Append(errLabel)
	content.Append(gapBox(10))

	resetting := false
	pendingReset := false
	passwordClearedOnSave := false

	updatePasswordStatus := func() {
		switch {
		case passwordClearedOnSave:
			passwordStatus.SetText("The saved password will be cleared when you save.")
			passwordEntry.SetPlaceholderText("Cleared on save")
		case hasPassword:
			passwordStatus.SetText("A custom relay password is saved. Leave this blank to keep it.")
			passwordEntry.SetPlaceholderText("Saved password")
		default:
			passwordStatus.SetText("No custom relay password is saved yet.")
			passwordEntry.SetPlaceholderText("Relay password")
		}
	}

	formInput := func() (apppkg.GroupConnectionInput, error) {
		in := apppkg.GroupConnectionInput{
			Override:      overrideCheck.Active(),
			UseRelay:      useRelayCheck.Active(),
			PublicAddress: strings.TrimSpace(publicEntry.Text()),
			RelayCustom:   customCheck.Active(),
			RelayHost:     strings.TrimSpace(relayHostEntry.Text()),
			RelayID:       strings.TrimSpace(relayIDEntry.Text()),
		}
		if text := strings.TrimSpace(portEntry.Text()); text != "" {
			port, err := strconv.Atoi(text)
			if err != nil || port < 1 || port > 65535 {
				return in, errors.New("the app port must be between 1 and 65535")
			}
			in.AppPort = port
		}
		if text := strings.TrimSpace(relayPortEntry.Text()); text != "" {
			port, err := strconv.Atoi(text)
			if err != nil || port < 1 || port > 65535 {
				return in, errors.New("the relay port must be between 1 and 65535")
			}
			in.RelayPort = port
		}
		return in, nil
	}

	formatEndpoint := func(connection apppkg.GroupConnection) string {
		if connection.UseRelay {
			if connection.RelayHost == "" {
				return ""
			}
			return net.JoinHostPort(connection.RelayHost, strconv.Itoa(connection.RelayPort))
		}
		endpoint, err := apppkg.Endpoint(connection.PublicAddress, connection.AppPort)
		if err != nil {
			return ""
		}
		return endpoint
	}

	if settingsConnection.UseRelay {
		summaryMode.SetText("Using Settings: Relay")
		summaryRoute.SetText("Relay: " + relayRoute(settingsConnection.RelayHost, settingsConnection.RelayPort, settingsConnection.RelayID))
		summaryDetail.SetText("Friends connect through the relay. No port forward is needed.")
	} else {
		summaryMode.SetText("Using Settings: Direct")
		endpoint := formatEndpoint(settingsConnection)
		if endpoint == "" {
			endpoint = "not set"
		}
		summaryRoute.SetText("Public address: " + endpoint)
		summaryDetail.SetText("Friends connect to the address in Settings.")
	}
	if settingsConnection.UseRelay {
		inheritedNote.SetText("Relay: from Settings · " + relayRoute(settingsConnection.RelayHost, settingsConnection.RelayPort, settingsConnection.RelayID))
	} else {
		inheritedNote.SetText("Relay: the Settings relay is not configured.")
	}

	var saveButton *gtk.Button
	var reset *gtk.Button
	updatePreview := func() {
		errLabel.SetVisible(false)
		in, parseErr := formInput()
		if parseErr != nil {
			preview.SetText("Check the port values.")
			listen.SetText("")
			warning.SetVisible(false)
			endpointNote.SetVisible(false)
			return
		}
		next, previewErr := u.state.PreviewGroupConnection(g.ID, in)
		if previewErr != nil {
			preview.SetText("Connection preview unavailable.")
			listen.SetText("")
			return
		}
		if !in.Override {
			preview.SetText("")
			listen.SetText("")
			warning.SetVisible(false)
			endpointNote.SetVisible(false)
			return
		}
		relayChanges := current.UseRelay || next.UseRelay
		if relayChanges {
			relayChanges = current.UseRelay != next.UseRelay ||
				current.RelayHost != next.RelayHost ||
				current.RelayPort != next.RelayPort ||
				current.RelayID != next.RelayID
		}
		oldEndpoint := formatEndpoint(current)
		newEndpoint := formatEndpoint(next)
		restartNeeded := live() && liveConnectionChange(current, next)
		endpointChanges := !current.UseRelay && !next.UseRelay && oldEndpoint != newEndpoint
		portChanges := !current.UseRelay && !next.UseRelay && current.AppPort != next.AppPort

		if next.UseRelay {
			preview.SetText("Relay: " + relayRoute(next.RelayHost, next.RelayPort, next.RelayID))
			if next.RelayHost != "" {
				listen.SetText("Friends connect through " + next.RelayHost + " on port " + strconv.Itoa(next.RelayPort) + ". No port forward is needed.")
			} else {
				listen.SetText("Set the relay host before saving.")
			}
		} else {
			endpoint := newEndpoint
			if endpoint == "" {
				endpoint = "not set"
			}
			preview.SetText("Friends connect to: " + endpoint)
			listen.SetText("Host listens on UDP " + strconv.Itoa(next.AppPort))
		}

		actionLabel := "Save"
		warning.SetVisible(false)
		if restartNeeded {
			actionLabel = "Stop session and apply"
			warning.SetText("Changing the connection restarts the session. Everyone will disconnect briefly.")
			warning.SetVisible(true)
		}
		if saveButton != nil {
			saveButton.SetLabel(actionLabel)
		}

		open := len(g.OpenInvites(time.Now()))
		if note := connectionChangeNote(relayChanges, endpointChanges, portChanges, open, restartNeeded); note != "" {
			endpointNote.SetText(note)
			endpointNote.SetVisible(true)
		} else {
			endpointNote.SetVisible(false)
		}
	}

	syncVisibility := func() {
		override := overrideCheck.Active()
		relay := override && useRelayCheck.Active()
		custom := relay && customCheck.Active()
		summaryBox.SetVisible(!override)
		overrideBox.SetVisible(override)
		directBox.SetVisible(override && !relay)
		relayBox.SetVisible(relay)
		customFields.SetVisible(custom)
		inheritedNote.SetVisible(relay && !custom)
		if reset != nil {
			reset.SetVisible(override)
		}
		updatePasswordStatus()
		updatePreview()
	}

	markEdited := func() {
		if resetting {
			return
		}
		pendingReset = false
		passwordClearedOnSave = false
		updatePasswordStatus()
	}

	overrideCheck.ConnectToggled(func() { markEdited(); syncVisibility() })
	useRelayCheck.ConnectToggled(func() { markEdited(); syncVisibility() })
	customCheck.ConnectToggled(func() { markEdited(); syncVisibility() })
	for _, entrance := range []*gtk.Entry{publicEntry, portEntry, relayHostEntry, relayPortEntry, relayIDEntry, passwordEntry} {
		entrance.ConnectChanged(func() { markEdited(); syncVisibility() })
	}

	buttons := gtk.NewBox(gtk.OrientationHorizontal, 8)
	reset = gtk.NewButtonWithLabel("Reset")
	reset.AddCSSClass("quiet")
	cancel := gtk.NewButtonWithLabel("Cancel")
	cancel.AddCSSClass("quiet")
	saveButton = gtk.NewButtonWithLabel("Save")
	saveButton.AddCSSClass("primary")

	reset.ConnectClicked(func() {
		resetting = true
		pendingReset = true
		passwordClearedOnSave = true
		overrideCheck.SetActive(false)
		useRelayCheck.SetActive(false)
		customCheck.SetActive(false)
		publicEntry.SetText("")
		portEntry.SetText("")
		relayHostEntry.SetText("")
		relayPortEntry.SetText("")
		relayIDEntry.SetText("")
		passwordEntry.SetText("")
		resetting = false
		syncVisibility()
	})
	cancel.ConnectClicked(func() { win.Close() })
	saveButton.ConnectClicked(func() {
		errLabel.SetVisible(false)
		in, err := formInput()
		if err != nil {
			errLabel.SetText(err.Error())
			errLabel.SetVisible(true)
			return
		}
		password := strings.TrimSpace(passwordEntry.Text())
		if in.Override && in.UseRelay && in.RelayCustom && password == "" && !hasPassword && !pendingReset {
			errLabel.SetText("Enter the password for this group's relay.")
			errLabel.SetVisible(true)
			return
		}
		next, err := u.state.PreviewGroupConnection(g.ID, in)
		if err != nil {
			errLabel.SetText(err.Error())
			errLabel.SetVisible(true)
			return
		}
		restartNeeded := live() && liveConnectionChange(current, next)
		if restartNeeded {
			if err := u.state.StopSession(); err != nil {
				errLabel.SetText(err.Error())
				errLabel.SetVisible(true)
				return
			}
		}

		if pendingReset {
			err = u.state.ResetGroupConnection(g.ID)
		} else {
			err = u.state.SaveGroupConnectionOptions(g.ID, in, password)
		}
		if err != nil {
			if restartNeeded {
				_ = u.state.StartSession(g.ID)
			}
			errLabel.SetText(err.Error())
			errLabel.SetVisible(true)
			return
		}
		if restartNeeded {
			if err := u.state.StartSession(g.ID); err != nil {
				updatePreview()
				errLabel.SetText("Connection saved, but the session could not restart: " + err.Error() + ". Start the session again.")
				errLabel.SetVisible(true)
				return
			}
		}
		win.Close()
		u.showGroup(g.ID)
	})

	buttons.Append(reset)
	buttons.Append(cancel)
	buttons.Append(saveButton)
	buttons.SetHAlign(gtk.AlignEnd)
	content.Append(buttons)

	syncVisibility()
	publicEntry.GrabFocus()
	win.Present()
}

func (u *probeUI) showJoinDialog() {
	win, content := u.dialog("Join with an invite")
	content.Append(textLabel("Invite code", "form-label"))
	codeEntry := entry("lp1_…")
	content.Append(codeEntry)
	content.Append(gapBox(8))

	preview := gtk.NewBox(gtk.OrientationVertical, 0)
	namePreview := textLabel("", "body")
	groupPreview := textLabel("", "desc")
	preview.Append(namePreview)
	preview.Append(gapBox(2))
	preview.Append(groupPreview)
	preview.SetVisible(false)
	content.Append(preview)
	content.Append(gapBox(10))

	content.Append(textLabel("Device Name", "form-label"))
	content.Append(textLabel(u.state.DeviceName(), "mono"))
	content.Append(gapBox(10))
	status := textLabel("", "desc")
	status.SetVisible(false)
	errLabel := textLabel("", "error")
	errLabel.SetVisible(false)
	content.Append(status)
	content.Append(errLabel)
	content.Append(gapBox(10))

	updatePreview := func() {
		nameLine, groupLine := joinInvitePreview(codeEntry.Text())
		if nameLine == "" {
			preview.SetVisible(false)
			return
		}
		namePreview.SetText(nameLine)
		groupPreview.SetText(groupLine)
		preview.SetVisible(true)
	}
	codeEntry.ConnectChanged(func() {
		errLabel.SetVisible(false)
		updatePreview()
	})
	updatePreview()

	buttons := gtk.NewBox(gtk.OrientationHorizontal, 8)
	cancel := gtk.NewButtonWithLabel("Cancel")
	cancel.AddCSSClass("quiet")
	join := gtk.NewButtonWithLabel("Join")
	join.AddCSSClass("primary")
	cancel.ConnectClicked(func() { win.Close() })
	join.ConnectClicked(func() {
		code := strings.TrimSpace(codeEntry.Text())
		if code == "" {
			errLabel.SetText("Paste the invite code first.")
			errLabel.SetVisible(true)
			return
		}
		errLabel.SetVisible(false)
		status.SetText("Waiting for the host to approve this device…")
		status.SetVisible(true)
		join.SetSensitive(false)
		cancel.SetSensitive(false)
		go func() {
			g, err := u.state.JoinInvite(context.Background(), code, "")
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

	liveID := u.state.LiveGroupID()
	liveName := ""
	if liveID != "" && liveID != g.ID {
		if other, ok := u.state.Group(liveID); ok {
			liveName = other.Name
		}
	}
	sessionState := inviteDialogStateFor(liveID, g.ID, liveName)
	if sessionState.Warning != "" {
		content.Append(textLabel(sessionState.Warning, "warning"))
		content.Append(gapBox(8))
	}

	content.Append(textLabel("Friend's name", "form-label"))
	nameEntry := entry("Ash")
	content.Append(nameEntry)
	if connection, err := u.state.EffectiveConnection(g.ID); err == nil {
		content.Append(gapBox(8))
		if connection.UseRelay {
			content.Append(textLabel("Relay: "+relayRoute(connection.RelayHost, connection.RelayPort, connection.RelayID), "desc"))
			content.Append(textLabel("The invite carries a short-lived relay token. Friends need no port forward.", "desc"))
		} else if endpoint, err := u.state.GroupEndpoint(g.ID); err == nil {
			content.Append(textLabel("Invite endpoint: "+endpoint, "desc"))
		}
	}
	content.Append(gapBox(10))
	errLabel := textLabel("", "error")
	errLabel.SetVisible(false)
	content.Append(errLabel)
	content.Append(gapBox(10))

	buttons := gtk.NewBox(gtk.OrientationHorizontal, 8)
	cancel := gtk.NewButtonWithLabel("Cancel")
	cancel.AddCSSClass("quiet")
	cancel.ConnectClicked(func() { win.Close() })
	create := gtk.NewButtonWithLabel(sessionState.Action)
	create.AddCSSClass("primary")
	if sessionState.Blocked {
		create.SetSensitive(false)
	}
	create.ConnectClicked(func() {
		name := strings.TrimSpace(nameEntry.Text())
		if name == "" {
			errLabel.SetText("Give the invite a name.")
			errLabel.SetVisible(true)
			return
		}
		code, err := u.createInvite(g, name)
		if err != nil {
			errLabel.SetText(err.Error())
			errLabel.SetVisible(true)
			return
		}
		win.Close()
		u.showGroup(g.ID)
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
	view := gtk.NewTextView()
	view.SetEditable(false)
	view.SetCursorVisible(false)
	view.SetMonospace(true)
	view.SetWrapMode(gtk.WrapChar)
	view.SetLeftMargin(8)
	view.SetRightMargin(8)
	view.SetTopMargin(8)
	view.SetBottomMargin(8)
	view.AddCSSClass("code-view")
	view.Buffer().SetText(code)
	codeScroll := gtk.NewScrolledWindow()
	codeScroll.AddCSSClass("code-box")
	codeScroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	codeScroll.SetChild(view)
	codeScroll.SetSizeRequest(320, 110)
	content.Append(codeScroll)
	if inv, err := invitepkg.Decode(code); err == nil {
		content.Append(gapBox(8))
		if inv.Version == invitepkg.RelayVersion {
			content.Append(textLabel("Relay: "+relayRoute(inv.RelayHost, inv.RelayPort, inv.RelayID), "desc"))
			content.Append(textLabel("Friends connect through the relay; no port forward is needed.", "desc"))
		} else {
			content.Append(textLabel("Code endpoint: "+inv.Endpoint, "desc"))
		}
	}
	content.Append(gapBox(10))
	content.Append(textLabel("Send this code privately. It works once and expires after 1 hour.", "desc"))
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
	u.showServiceForm(g, nil)
}

func (u *probeUI) showEditServiceDialog(g group.Group, s group.Service) {
	u.showServiceForm(g, &s)
}

func (u *probeUI) showServiceForm(g group.Group, existing *group.Service) {
	title := "Publish a service"
	actionLabel := "Publish"
	if existing != nil {
		title = "Edit service"
		actionLabel = "Save changes"
	}
	win, content := u.dialog(title)
	win.SetDefaultSize(360, 540)
	content.Append(textLabel("Game preset", "form-label"))
	presetArea := gtk.NewBox(gtk.OrientationVertical, 0)
	content.Append(presetArea)
	content.Append(gapBox(8))

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

	content.Append(textLabel("Game server IP", "form-label"))
	ipEntry := entry("127.0.0.1")
	ipEntry.SetText("127.0.0.1")
	ipEntry.SetTooltipText("The IP of the PC running the game. Use 127.0.0.1 for this PC, including containers.")
	content.Append(ipEntry)
	content.Append(gapBox(4))
	content.Append(textLabel(serviceTargetHelp, "help"))
	content.Append(gapBox(8))

	portRow := gtk.NewBox(gtk.OrientationHorizontal, 8)
	gameCol := gtk.NewBox(gtk.OrientationVertical, 0)
	gameCol.SetHExpand(true)
	gameCol.Append(lineLabel("Game server port(s)", "form-label"))
	gamePortsEntry := entry("25565 or 2456-2458")
	gamePortsEntry.SetTooltipText("Port(s) the game server listens on, e.g. 25565, 2456-2458 or 2456,2600.")
	gameCol.Append(gamePortsEntry)
	portRow.Append(gameCol)

	friendCol := gtk.NewBox(gtk.OrientationVertical, 0)
	friendCol.SetHExpand(true)
	friendCol.Append(lineLabel("Friend port (optional)", "form-label"))
	friendPortsEntry := entry("Same as game port(s)")
	friendPortsEntry.SetTooltipText("The local port your friends use. Leave blank to use the game server port(s).")
	friendCol.Append(friendPortsEntry)
	portRow.Append(friendCol)
	content.Append(portRow)
	content.Append(gapBox(8))

	content.Append(textLabel("Hint", "form-label"))
	hintEntry := entry("Direct Connect in Minecraft")
	content.Append(hintEntry)
	content.Append(gapBox(8))

	preview := textLabel("", "desc")
	content.Append(preview)
	content.Append(gapBox(4))

	warning := textLabel("", "warning")
	warning.SetVisible(false)
	content.Append(warning)
	content.Append(gapBox(4))

	errLabel := textLabel("", "error")
	errLabel.SetVisible(false)
	content.Append(errLabel)
	content.Append(gapBox(10))

	updatePreview := func() {
		preview.SetText(serviceRoutePreview(ipEntry.Text(), gamePortsEntry.Text(), friendPortsEntry.Text()))
		warnings := serviceWarningText(ipEntry.Text(), gamePortsEntry.Text(), friendPortsEntry.Text(), protoValue(selectedProtocol))
		warning.SetText(warnings)
		warning.SetVisible(warnings != "")
		errLabel.SetVisible(false)
	}
	ipEntry.ConnectChanged(func() { updatePreview() })
	gamePortsEntry.ConnectChanged(func() { updatePreview() })
	friendPortsEntry.ConnectChanged(func() { updatePreview() })

	presetList := gtk.NewBox(gtk.OrientationVertical, 0)
	var presetMenu *gtk.MenuButton
	var presetPopover *gtk.Popover
	for _, item := range preset.All() {
		item := item
		button := gtk.NewButtonWithLabel(item.Name)
		button.AddCSSClass("quiet")
		button.SetHAlign(gtk.AlignStart)
		button.ConnectClicked(func() {
			nameEntry.SetText(item.Name)
			gamePortsEntry.SetText(group.FormatPorts(item.Ports))
			friendPortsEntry.SetText("")
			hintEntry.SetText(item.Hint)
			setProtocol(protoIndex(item.Proto))
			presetMenu.SetLabel(item.Name)
			presetPopover.Popdown()
			updatePreview()
		})
		presetList.Append(button)
	}
	presetScroll := gtk.NewScrolledWindow()
	presetScroll.AddCSSClass("preset-scroll")
	presetScroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	presetScroll.SetChild(presetList)
	presetScroll.SetSizeRequest(300, 260)
	presetList.AddCSSClass("preset-list")
	presetPopover = gtk.NewPopover()
	presetPopover.AddCSSClass("preset-popover")
	presetPopover.SetHasArrow(false)
	presetPopover.SetAutohide(true)
	presetSurface := gtk.NewBox(gtk.OrientationVertical, 0)
	presetSurface.AddCSSClass("preset-surface")
	presetSurface.Append(presetScroll)
	presetPopover.SetChild(presetSurface)
	presetMenu = gtk.NewMenuButton()
	presetMenu.SetLabel("Choose a preset")
	presetMenu.AddCSSClass("preset-menu")
	presetMenu.SetPopover(presetPopover)
	presetArea.Append(presetMenu)

	var prefillErr error
	if existing != nil {
		nameEntry.SetText(existing.Name)
		ip, gamePorts, friendPort, hint, err := serviceFormValues(*existing)
		if err != nil {
			prefillErr = err
		} else {
			ipEntry.SetText(ip)
			gamePortsEntry.SetText(gamePorts)
			friendPortsEntry.SetText(friendPort)
			hintEntry.SetText(hint)
			setProtocol(protoIndex(existing.Proto))
		}
	}

	updatePreview()
	if prefillErr != nil {
		errLabel.SetText(prefillErr.Error())
		errLabel.SetVisible(true)
	}

	buttons := gtk.NewBox(gtk.OrientationHorizontal, 8)
	cancel := gtk.NewButtonWithLabel("Cancel")
	cancel.AddCSSClass("quiet")
	cancel.ConnectClicked(func() { win.Close() })
	add := gtk.NewButtonWithLabel(actionLabel)
	add.AddCSSClass("primary")
	add.ConnectClicked(func() {
		gamePorts, err := group.ParsePorts(gamePortsEntry.Text())
		if err != nil {
			errLabel.SetText("Game server port(s): " + err.Error())
			errLabel.SetVisible(true)
			return
		}
		var friendPorts []int
		if strings.TrimSpace(friendPortsEntry.Text()) != "" {
			friendPorts, err = group.ParsePorts(friendPortsEntry.Text())
			if err != nil {
				errLabel.SetText("Friend port: " + err.Error())
				errLabel.SetVisible(true)
				return
			}
		}
		target, ports, err := group.ComposeServiceTarget(ipEntry.Text(), gamePorts, friendPorts)
		if err != nil {
			errLabel.SetText(err.Error())
			errLabel.SetVisible(true)
			return
		}
		service := group.Service{
			Name:   strings.TrimSpace(nameEntry.Text()),
			Proto:  protoValue(selectedProtocol),
			Target: target,
			Ports:  ports,
			Hint:   strings.TrimSpace(hintEntry.Text()),
		}
		if existing != nil {
			service.ID = existing.ID
			err = u.state.UpdateService(g.ID, service)
		} else {
			service.ID = group.NewID()
			err = u.state.AddService(g.ID, service)
		}
		if err != nil {
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
	win.SetDefaultWidget(add)
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
	confirm.AddCSSClass("secondary")
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

func (u *probeUI) showMemberDialog(g group.Group, m group.Member) {
	win, content := u.dialog("Member")
	content.Append(textLabel(m.Name, "card-title"))
	content.Append(gapBox(10))
	content.Append(cardText("Device: "+orUnknown(m.DeviceName), "desc"))
	content.Append(cardText("Address: "+orUnknown(m.LastAddress), "desc"))
	content.Append(cardText("Fingerprint: "+orUnknown(m.Fingerprint), "desc"))
	content.Append(cardText("First seen: "+when(m.FirstSeen), "desc"))
	content.Append(cardText("Last seen: "+when(m.LastSeen), "desc"))
	status := "approved"
	if !m.IsApproved() {
		status = "waiting for approval"
	}
	content.Append(cardText("Status: "+status, "desc"))
	content.Append(gapBox(14))

	buttons := gtk.NewBox(gtk.OrientationHorizontal, 8)
	cancel := gtk.NewButtonWithLabel("Close")
	cancel.AddCSSClass("quiet")
	cancel.ConnectClicked(func() { win.Close() })
	remove := gtk.NewButtonWithLabel("Remove member")
	remove.AddCSSClass("secondary")
	remove.ConnectClicked(func() {
		if err := u.state.RemoveMember(g.ID, m.ID); err != nil {
			u.alert("Could not remove member", err.Error())
			return
		}
		win.Close()
		u.showGroup(g.ID)
	})
	buttons.Append(cancel)
	buttons.Append(remove)
	buttons.SetHAlign(gtk.AlignEnd)
	content.Append(buttons)
	win.Present()
}

func (u *probeUI) showServiceDialog(g group.Group, s group.Service) {
	win, content := u.dialog("Service")
	content.Append(textLabel(s.Name, "card-title"))
	content.Append(gapBox(10))
	content.Append(cardText("Protocol: "+protoName(s.Proto), "desc"))
	game := s.Target
	if address, err := s.GameAddress(); err == nil {
		game = address
	}
	content.Append(cardText("Game server: "+game, "desc"))
	if gamePorts, err := s.GamePorts(); err == nil {
		content.Append(cardText("Game port(s): "+group.FormatPorts(gamePorts), "desc"))
	}
	content.Append(cardText("Friend port(s): "+group.FormatPorts(s.Ports), "desc"))
	if s.Hint != "" {
		content.Append(cardText("Hint: "+s.Hint, "desc"))
	}
	content.Append(gapBox(14))

	buttons := gtk.NewBox(gtk.OrientationHorizontal, 8)
	cancel := gtk.NewButtonWithLabel("Close")
	cancel.AddCSSClass("quiet")
	cancel.ConnectClicked(func() { win.Close() })
	remove := gtk.NewButtonWithLabel("Remove service")
	remove.AddCSSClass("secondary")
	remove.ConnectClicked(func() {
		if err := u.state.RemoveService(g.ID, s.ID); err != nil {
			u.alert("Could not remove service", err.Error())
			return
		}
		win.Close()
		u.showGroup(g.ID)
	})
	edit := gtk.NewButtonWithLabel("Edit")
	edit.AddCSSClass("primary")
	edit.ConnectClicked(func() {
		win.Close()
		u.showEditServiceDialog(g, s)
	})
	buttons.Append(cancel)
	buttons.Append(remove)
	buttons.Append(edit)
	buttons.SetHAlign(gtk.AlignEnd)
	content.Append(buttons)
	win.Present()
}

func (u *probeUI) confirmEndSession(g group.Group) {
	u.confirm("End the session?", "Connected friends will be disconnected. The group and its invites stay saved.", "End session", func() {
		_ = u.state.StopSession()
		u.showGroup(g.ID)
	})
}

func (u *probeUI) confirmDisconnect(g group.Group) {
	u.confirm("Disconnect?", "The group stays saved and you can reconnect later.", "Disconnect", func() {
		_ = u.state.StopSession()
		u.showGroup(g.ID)
	})
}

func (u *probeUI) confirmLeaveGroup(g group.Group) {
	u.confirm("Leave "+g.Name+"?", "This removes the saved group and its device key from this PC.", "Leave group", func() {
		if err := u.state.DeleteGroup(g.ID); err != nil {
			u.alert("Could not leave group", err.Error())
			return
		}
		u.showHome()
	})
}

func (u *probeUI) confirmCancelInvite(g group.Group, inv group.Invite) {
	u.confirm("Cancel invite for "+inv.Name+"?", "The code will stop working immediately.", "Cancel invite", func() {
		if err := u.state.RemoveInvite(g.ID, inv.ID); err != nil {
			u.alert("Could not cancel invite", err.Error())
			return
		}
		u.showGroup(g.ID)
	})
}

func addClick(widget *gtk.Box, onTap func()) {
	gesture := gtk.NewGestureClick()
	gesture.SetButton(1)
	gesture.ConnectReleased(func(nPress int, x, y float64) {
		if nPress == 1 {
			onTap()
		}
	})
	gesture.SetPropagationPhase(gtk.PhaseBubble)
	widget.AddController(gesture)
}

func orUnknown(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

func when(at time.Time) string {
	if at.IsZero() {
		return "never"
	}
	return at.Local().Format("2 Jan 15:04")
}

func (u *probeUI) dialog(title string) (*gtk.Window, *gtk.Box) {
	win := gtk.NewWindow()
	win.SetTitle(title)
	win.SetTransientFor(&u.win.Window)
	win.SetModal(true)
	win.SetResizable(false)
	win.SetDefaultSize(360, 320)
	win.SetDecorated(false)

	outer := gtk.NewBox(gtk.OrientationVertical, 0)

	topDrag := gtk.NewWindowHandle()
	topDrag.SetHExpand(true)
	topDrag.SetSizeRequest(-1, 24)
	u.enableWindowDrag(topDrag, win)
	outer.Append(topDrag)

	content := gtk.NewBox(gtk.OrientationVertical, 0)
	content.SetMarginTop(0)
	content.SetMarginBottom(20)
	content.SetMarginStart(20)
	content.SetMarginEnd(20)

	header := gtk.NewBox(gtk.OrientationHorizontal, 8)
	titleLabel := textLabel(title, "card-title")
	titleLabel.SetHExpand(true)
	titleHandle := gtk.NewWindowHandle()
	titleHandle.SetHExpand(true)
	titleHandle.SetChild(titleLabel)
	u.enableWindowDrag(titleHandle, win)
	header.Append(titleHandle)
	close := gtk.NewButtonWithLabel("×")
	close.AddCSSClass("quiet")
	close.ConnectClicked(func() { win.Close() })
	header.Append(close)
	content.Append(header)
	content.Append(gapBox(12))
	outer.Append(content)
	win.SetChild(outer)
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

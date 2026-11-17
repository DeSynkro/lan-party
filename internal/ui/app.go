package ui

import (
	"context"
	"errors"
	"time"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"

	apppkg "github.com/desynkro/lan-party/internal/app"
	"github.com/desynkro/lan-party/internal/config"
)

const (
	windowWidth  = 400
	windowHeight = 640
)

// UI owns the window and the screen currently on it.
type UI struct {
	fyneApp    fyne.App
	win        fyne.Window
	app        *apppkg.App
	back       func()
	current    func()
	screenName string

	cancelRefresh context.CancelFunc
	approvalShown string
	hasTray       bool
}

// Run opens the application.
func Run() {
	fyneApp := fyneapp.NewWithID("com.desynkro.lanparty")
	fyneApp.Settings().SetTheme(NewTheme())

	win := fyneApp.NewWindow("LAN Party")
	win.SetIcon(appIcon)
	win.Resize(fyne.NewSize(windowWidth, windowHeight))
	win.SetFixedSize(true)
	win.CenterOnScreen()

	state, err := apppkg.New(config.Resolve())
	if err != nil {
		win.SetContent(startupError(fyneApp, err))
		win.ShowAndRun()
		return
	}

	u := &UI{fyneApp: fyneApp, win: win, app: state}
	u.showHome()
	u.hasTray = u.setupTray()
	win.SetCloseIntercept(u.closeWindow)
	go u.watchApprovals()

	if u.hasTray && state.Settings().StartMinimised {
		fyneApp.Run()
		_ = state.Close()
		return
	}

	win.ShowAndRun()
	_ = state.Close()
}

func (u *UI) reveal() {
	u.win.Show()
	u.win.RequestFocus()
}

func (u *UI) setupTray() bool {
	desk, ok := u.fyneApp.(desktop.App)
	if !ok {
		return false
	}
	desk.SetSystemTrayIcon(appIcon)
	desk.SetSystemTrayMenu(fyne.NewMenu("LAN Party",
		fyne.NewMenuItem("Show", func() {
			fyne.Do(u.reveal)
		}),
		fyne.NewMenuItem("How to Use", func() {
			fyne.Do(func() {
				u.reveal()
				u.showGuide(u.currentScreen())
			})
		}),
		fyne.NewMenuItem("Settings", func() {
			fyne.Do(func() {
				u.reveal()
				u.showSettings(u.currentScreen())
			})
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Quit", func() {
			fyne.Do(func() {
				_ = u.app.StopSession()
				u.fyneApp.Quit()
			})
		}),
	))
	return true
}

func (u *UI) closeWindow() {
	settings := u.app.Settings()
	if u.hasTray && settings.CloseToTray {
		u.win.Hide()
		return
	}
	if u.app.LiveGroupID() != "" {
		dialog.NewConfirm("Quit LAN Party?",
			"A session is live. Closing the app ends it for everyone.",
			func(ok bool) {
				if !ok {
					return
				}
				_ = u.app.StopSession()
				u.fyneApp.Quit()
			}, u.win).Show()
		return
	}
	_ = u.app.StopSession()
	// Closing the window is not enough when a tray icon exists: the tray keeps
	// the Fyne process alive. Quit explicitly.
	u.fyneApp.Quit()
}

func startupError(fyneApp fyne.App, err error) fyne.CanvasObject {
	content := container.NewVBox(
		titleText("LAN Party could not start"),
		gap(8),
		bodyLabel(err.Error()),
		gap(16),
		newButton("Quit", buttonPrimary, fyneApp.Quit),
	)
	return pad(24, 20, content)
}

func (u *UI) setScreen(content fyne.CanvasObject, refresh func()) {
	if u.cancelRefresh != nil {
		u.cancelRefresh()
		u.cancelRefresh = nil
	}
	u.win.SetContent(content)
	u.win.Canvas().Unfocus()

	if refresh == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	u.cancelRefresh = cancel
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				fyne.Do(refresh)
			}
		}
	}()
}

func (u *UI) watchApprovals() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		approval := u.app.Approval()
		if approval == nil {
			u.approvalShown = ""
			continue
		}
		if approval.ID == u.approvalShown {
			continue
		}
		u.approvalShown = approval.ID
		copied := *approval
		fyne.Do(func() { u.showApprovalDialog(copied) })
	}
}

func (u *UI) showApprovalDialog(approval apppkg.Approval) {
	u.reveal()
	title := "Approve new device"
	headline := approval.PeerName + " wants to join"
	if approval.Kind == "new device" {
		headline = approval.PeerName + " has a new device"
	}
	if g, ok := u.app.Group(approval.GroupID); ok {
		headline += " " + g.Name
	}

	content := container.NewVBox(
		bodyLabel(headline),
		gap(8),
		metaText("Device  "+approval.DeviceName),
		metaText("Address  "+approval.Address),
		metaText("Code  "+approval.Fingerprint),
		divider(),
		mutedLabel("Allow only if you recognise the device. The address is what the host sees, not a proof of identity."),
	)

	d := dialog.NewCustomConfirm(title, "Allow", "Deny", content, func(allow bool) {
		if !u.app.ResolveApproval(approval.ID, allow) {
			dialog.ShowInformation("Request expired", "That device is no longer waiting.", u.win)
		}
	}, u.win)
	d.Show()
}

func (u *UI) runWaiting(title, message string, work func(context.Context) error, done func(error)) {
	ctx, cancel := context.WithCancel(context.Background())
	content := container.NewVBox(
		bodyLabel(message),
		gap(8),
		mutedLabel("This can take a moment."),
	)
	d := dialog.NewCustom(title, "Cancel", content, u.win)
	d.SetOnClosed(cancel)
	d.Show()

	go func() {
		err := work(ctx)
		fyne.Do(func() {
			d.Hide()
			done(err)
		})
	}()
}

func (u *UI) showError(err error) {
	if err == nil {
		return
	}
	dialog.ShowError(err, u.win)
}

func (u *UI) showInfo(title, message string) {
	dialog.ShowInformation(title, message, u.win)
}

func (u *UI) appBar(title string, home bool) fyne.CanvasObject {
	var left fyne.CanvasObject
	if home {
		mark := canvas.NewImageFromResource(appIcon)
		mark.FillMode = canvas.ImageFillContain
		mark.SetMinSize(fyne.NewSize(22, 22))
		left = container.NewHBox(mark, gapW(8), appBarTitle("LAN Party"))
	} else {
		if u.back != nil {
			left = container.NewHBox(
				smallButton("Back", buttonQuiet, u.goBack),
				gapW(2),
				appBarTitle(title),
			)
		} else {
			left = container.NewHBox(appBarTitle(title))
		}
	}

	right := container.NewHBox()
	if u.screenName != "guide" {
		right.Add(smallButton("?", buttonQuiet, func() { u.showGuide(u.currentScreen()) }))
	}
	if u.screenName != "settings" {
		if len(right.Objects) > 0 {
			right.Add(gapW(2))
		}
		right.Add(smallButton("Settings", buttonQuiet, func() { u.showSettings(u.currentScreen()) }))
	}

	row := container.NewBorder(nil, nil, container.NewCenter(left), container.NewCenter(right), layout.NewSpacer())
	bar := canvas.NewRectangle(colourBackground)
	bar.SetMinSize(fyne.NewSize(0, 48))
	stack := container.NewStack(bar, pad(0, 12, row))
	return container.NewVBox(stack, hairline())
}

func (u *UI) goBack() {
	if u.back != nil {
		u.back()
	}
}

func (u *UI) screen(bar fyne.CanvasObject, content fyne.CanvasObject, refresh func()) {
	body := container.NewVScroll(pad(12, 16, content))
	u.setScreen(container.NewBorder(bar, nil, nil, nil, body), refresh)
}

func (u *UI) currentScreen() func() {
	if u.current != nil {
		return u.current
	}
	return u.showHome
}

func (u *UI) showGroup(id string) {
	g, ok := u.app.Group(id)
	if !ok {
		u.showHome()
		return
	}
	if g.IsHost() {
		u.showHostGroup(g)
		return
	}
	u.showFriend(g)
}

var errCancelled = errors.New("cancelled")

func ignoreCancel(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, errCancelled)
}

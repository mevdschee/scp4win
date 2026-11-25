//go:build windows

package ui

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mevdschee/scp4win/internal/app"
	"github.com/mevdschee/scp4win/internal/logging"
	"github.com/mevdschee/scp4win/internal/update"
	"github.com/rodrigocfd/windigo/co"
	"github.com/rodrigocfd/windigo/ui"
)

// checkForUpdates runs when the user clicks Update: it asks GitHub for the
// latest release and offers to install it when it is newer. Nothing is
// checked automatically.
func (me *Explorer) checkForUpdates() {
	if me.busy {
		return
	}
	if !update.IsRelease(me.version) {
		ui.MsgOk(me.wnd, "Update", "Development build",
			"This is a development build ("+me.version+"). Updates are only for installed releases.")
		return
	}
	me.busy = true
	me.setStatus("Checking for updates...")
	me.updateButtons()
	go func() {
		var r update.Release
		var ok bool
		err := errors.New("internal error, see the log")
		func() {
			defer logging.Recover("update check")
			r, ok, err = update.Latest(me.ctx, me.version)
		}()
		me.wnd.UiThread(func() {
			me.busy = false
			me.setStatus("Ready")
			me.updateButtons()
			switch {
			case err != nil:
				slog.Error("update check failed", "err", err)
				ui.MsgError(me.wnd, "Update", "Could not check for updates", err.Error())
			case !ok || !update.Newer(r.Version, me.version):
				slog.Info("no update", "latest", r.Version, "current", me.version)
				ui.MsgOk(me.wnd, "Update", "scp4win is up to date", "You have the latest version ("+me.version+").")
			default:
				slog.Info("update available", "latest", r.Version, "current", me.version)
				me.offerUpdate(r)
			}
		})
	}()
}

// installed reports whether this is the copy the MSI installed, in
// %LOCALAPPDATA%\Programs\scp4win, and not the portable exe. A portable copy
// cannot update itself: the MSI would install a second copy next to it.
func installed() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	dir := filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "scp4win")
	return strings.EqualFold(filepath.Clean(filepath.Dir(exe)), filepath.Clean(dir))
}

func (me *Explorer) offerUpdate(r update.Release) {
	if !installed() {
		if ui.MsgOkCancel(me.wnd, "Update", "scp4win "+r.Version+" is available",
			fmt.Sprintf("You have %s. This copy is not installed, so it cannot update itself. Open the download page?",
				me.version), "Open") {
			openURL(me.wnd.Hwnd(), repoURL+"/releases/latest")
		}
		return
	}
	if !ui.MsgOkCancel(me.wnd, "Update", "scp4win "+r.Version+" is available",
		fmt.Sprintf("You have %s. Download and install %s now? scp4win closes during the installation.",
			me.version, r.Version), "Install") {
		return
	}
	me.busy = true
	me.updateButtons()
	var path string
	err := me.progressDialog("Downloading update", func(ctx context.Context, progress app.Progress) error {
		var err error
		path, err = update.Download(ctx, r, os.TempDir(), func(done, total int64) {
			progress(app.ProgressInfo{Action: "Downloading", File: r.MsiName, FileCount: 1, Bytes: done, TotalBytes: total})
		})
		return err
	})
	me.busy = false
	me.updateButtons()
	if errors.Is(err, context.Canceled) {
		return
	}
	if err == nil {
		// msiexec shows the installer; the MSI replaces the running version,
		// so close to release scp4win.exe.
		log := filepath.Join(os.TempDir(), "scp4win-update.log")
		err = exec.Command("msiexec", "/i", path, "/l*v", log).Start()
	}
	if err != nil {
		slog.Error("update failed", "err", err)
		ui.MsgError(me.wnd, "Update", "Could not install the update", err.Error())
		return
	}
	slog.Info("installing update, closing", "msi", path)
	me.wnd.Hwnd().PostMessage(co.WM_CLOSE, 0, 0)
}

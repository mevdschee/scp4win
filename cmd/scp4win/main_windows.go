// Command scp4win is the scp4win explorer for Windows.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"runtime/debug"

	"github.com/mevdschee/scp4win/internal/config"
	"github.com/mevdschee/scp4win/internal/logging"
	"github.com/mevdschee/scp4win/internal/ui"
	wui "github.com/rodrigocfd/windigo/ui"
	"github.com/rodrigocfd/windigo/win"
)

// version is set at build time by release.sh (-ldflags -X main.version=...).
var version = "dev"

func main() {
	runtime.LockOSThread() // the Win32 GUI is single-threaded

	dir, err := config.Dir()
	if err != nil {
		wui.MsgError(nil, "scp4win", "No settings folder", err.Error())
		os.Exit(1)
	}
	logFile, closeLog, err := logging.Setup(dir)
	if err != nil {
		wui.MsgError(nil, "scp4win", "Could not open the log file", err.Error())
	} else {
		defer closeLog()
	}
	commit := ""
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" {
				commit = s.Value
			}
		}
	}
	slog.Info("start", "version", version, "commit", commit, "go", runtime.Version(), "settings", dir, "log", logFile)
	defer func() {
		if r := recover(); r != nil {
			slog.Error("crash", "panic", fmt.Sprint(r), "stack", string(debug.Stack()))
			wui.MsgError(nil, "scp4win", "scp4win crashed", fmt.Sprintf("%v\n\nDetails are in %s", r, logFile))
			os.Exit(2)
		}
	}()

	// OLE (not only COM) is needed for drag and drop to Windows Explorer.
	if err := win.OleInitialize(); err != nil {
		slog.Error("OleInitialize", "err", err)
	}
	defer win.OleUninitialize()

	cfg, err := config.Load(dir)
	if err != nil {
		slog.Error("read settings", "err", err)
		wui.MsgError(nil, "scp4win", "Could not read settings", err.Error())
		cfg = &config.Config{}
	}
	code := ui.Run(dir, cfg, version)
	slog.Info("exit", "code", code)
	os.Exit(code)
}

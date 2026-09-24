package main

// MEXXIC Tweaks: application entry point.
//
// Project layout
//   main.go              start-up, window
//   ui_bridge.go         UI  ↔ services (functions callable from ui/index.html)
//   ui/index.html        complete interface (HTML/CSS/JS, embedded into the EXE)
//   job_service.go       Services: restore point → backup → apply/restore → verify
//   app_state.go         Services: settings, login, activity, restart tracking, system info
//   backup_store.go      Backup: original values + sessions (backup.json)
//   restore_service.go   Restore: Windows restore points
//   tweak.go             Tweaks: Tweak type (Apply / Restore / Active / State)
//   tweaks_*.go          Tweaks per category (Input, Network, Windows, Gaming, FiveM)
//   models.go            Models
//   util_winapi.go       Utilities: Windows API
//   util_registry.go     Utilities: registry

import (
	_ "embed"
	"os"
	"path/filepath"
	"runtime"

	"github.com/jchv/go-webview2"
)

//go:embed ui/index.html
var uiHTML string

const appVersion = "2.0.0"

var (
	appDir    string
	tweaks    []*Tweak
	tweakByID = map[string]*Tweak{}
)

func main() {
	runtime.LockOSThread()

	base := os.Getenv("APPDATA")
	if base == "" {
		base, _ = os.UserConfigDir()
	}
	appDir = filepath.Join(base, "MexxicTweaks")

	tweaks = buildTweaks()
	for _, t := range tweaks {
		tweakByID[t.ID] = t
	}
	loadJSON("settings.json", &settings)
	loadJSON("activity.json", &activity)
	loadStore()

	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		local = appDir
	}
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		AutoFocus: true,
		DataPath:  filepath.Join(local, "MexxicTweaks", "WebView2"),
		WindowOptions: webview2.WindowOptions{
			Title: "MEXXIC Tweaks", Width: 1320, Height: 840, IconId: 1, Center: true,
		},
	})
	if w == nil {
		messageBox("MEXXIC Tweaks", "The Microsoft Edge WebView2 Runtime is missing.\nDie Microsoft Edge WebView2 Runtime fehlt.\n\nhttps://developer.microsoft.com/microsoft-edge/webview2/")
		return
	}
	defer w.Destroy()
	w.SetSize(1120, 740, webview2.HintMin)
	bindAll(w)
	w.SetHtml(uiHTML)
	w.Run()

	if timerOn {
		setTimerRes(5000, false)
	}
}

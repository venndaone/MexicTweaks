package main

// UI: bridge between the HTML interface (ui/index.html) and the services.
// Every function bound here is callable from JavaScript as window.<name>().

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
)

var (
	authed     bool
	timerOn    bool
	errNoLogin = errors.New("not logged in")
)

type TweakState struct {
	*Tweak
	Active    bool      `json:"active"`
	HasBackup bool      `json:"hasBackup"`
	BackupAt  time.Time `json:"backupAt"`
}

type SessionView struct {
	*Session
	Restorable []string `json:"restorable"`
}

type AppState struct {
	Tweaks        []TweakState  `json:"tweaks"`
	Categories    []string      `json:"categories"`
	Sessions      []SessionView `json:"sessions"`
	LastBackup    time.Time     `json:"lastBackup"`
	RestoreAvail  bool          `json:"restoreAvailable"`
	RestartNeeded bool          `json:"restartNeeded"`
	Activity      []LogLine     `json:"activity"`
	Admin         bool          `json:"admin"`
}

func buildState() AppState {
	st := AppState{Categories: categoryOrder, Admin: isAdmin(), RestartNeeded: restartNeeded()}
	storeMu.Lock()
	for _, t := range tweaks {
		b, ok := store.Tweaks[t.ID]
		ts := TweakState{Tweak: t, Active: t.Active(), HasBackup: ok}
		if ok {
			ts.BackupAt = b.CreatedAt
		}
		st.Tweaks = append(st.Tweaks, ts)
	}
	for i := len(store.Sessions) - 1; i >= 0; i-- { // newest first
		s := store.Sessions[i]
		v := SessionView{Session: s, Restorable: []string{}}
		for _, x := range s.Tweaks {
			if b, ok := store.Tweaks[x.ID]; ok && b.SessionID == s.ID {
				v.Restorable = append(v.Restorable, x.ID)
			}
		}
		st.Sessions = append(st.Sessions, v)
	}
	st.LastBackup = lastBackupTime()
	st.RestoreAvail = len(store.Tweaks) > 0
	storeMu.Unlock()

	activityMu.Lock()
	for i := len(activity) - 1; i >= 0 && len(st.Activity) < 12; i-- {
		st.Activity = append(st.Activity, activity[i])
	}
	activityMu.Unlock()
	return st
}

func isFiveMProc(name string) bool {
	l := strings.ToLower(name)
	return strings.HasPrefix(l, "fivem") && strings.HasSuffix(l, "gtaprocess.exe")
}

func relaunchAsAdmin() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(exe)
	return windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL)
}

func bindAll(w webview2.WebView) {
	need := func() error {
		if !authed {
			return errNoLogin
		}
		return nil
	}

	// ---- app / login ----
	w.Bind("mxInfo", func() map[string]interface{} {
		settingsMu.Lock()
		lang := settings.Lang
		settingsMu.Unlock()
		return map[string]interface{}{"admin": isAdmin(), "version": appVersion, "lang": lang}
	})
	w.Bind("mxSetLang", func(lang string) {
		if lang != "de" && lang != "en" {
			return
		}
		settingsMu.Lock()
		settings.Lang = lang
		settingsMu.Unlock()
		saveSettings()
	})
	w.Bind("mxLogin", func(pw string) bool {
		authed = checkPw(pw)
		if !authed {
			time.Sleep(500 * time.Millisecond)
		}
		return authed
	})
	w.Bind("mxRelaunchAdmin", func() error {
		if err := relaunchAsAdmin(); err != nil {
			return err
		}
		w.Terminate()
		return nil
	})

	// ---- state / tweaks ----
	w.Bind("mxState", func() (AppState, error) {
		if err := need(); err != nil {
			return AppState{}, err
		}
		return buildState(), nil
	})
	w.Bind("mxTweakValues", func(id string) ([]KV, error) {
		if err := need(); err != nil {
			return nil, err
		}
		t := tweakByID[id]
		if t == nil {
			return nil, errors.New("unknown tweak")
		}
		return t.State(), nil
	})

	// ---- jobs ----
	validate := func(ids []string) error {
		for _, id := range ids {
			if tweakByID[id] == nil {
				return errors.New("unknown tweak: " + id)
			}
		}
		return nil
	}
	w.Bind("mxApply", func(applyIDs, restoreIDs []string) error {
		if err := need(); err != nil {
			return err
		}
		if err := validate(append(append([]string{}, applyIDs...), restoreIDs...)); err != nil {
			return err
		}
		if len(applyIDs)+len(restoreIDs) == 0 {
			return errors.New("nothing selected")
		}
		kind := "apply"
		if len(applyIDs) == 0 {
			kind = "restore"
		}
		return startJob(kind, applyIDs, restoreIDs)
	})
	w.Bind("mxRestore", func(ids []string) error {
		if err := need(); err != nil {
			return err
		}
		if err := validate(ids); err != nil {
			return err
		}
		return startJob("restore", nil, ids)
	})
	w.Bind("mxCreateRestorePoint", func() error {
		if err := need(); err != nil {
			return err
		}
		return startJob("rp", nil, nil)
	})
	w.Bind("mxJob", func() Job { return jobSnapshot() })
	w.Bind("mxConfirm", func(ok bool) {
		select {
		case confirmCh <- ok:
		default:
		}
	})

	// ---- restore points ----
	w.Bind("mxRestorePoints", func(refresh bool) map[string]interface{} {
		if refresh || (!rpLoaded && !rpLoading) {
			refreshRestorePoints()
		}
		rpMu.Lock()
		defer rpMu.Unlock()
		return map[string]interface{}{"loading": rpLoading, "items": rpItems, "error": rpErr}
	})
	w.Bind("mxOpenSystemRestore", func() { runHidden("rstrui.exe") })

	// ---- system / live tools ----
	w.Bind("mxSysInfo", func() (SysInfo, error) {
		if err := need(); err != nil {
			return SysInfo{}, err
		}
		return systemInfo(), nil
	})
	w.Bind("mxKeys", func() []bool {
		return []bool{keyDown('W'), keyDown('A'), keyDown('S'), keyDown('D'), keyDown(0x10), keyDown(0x20), keyDown(0x11)}
	})
	w.Bind("mxTimer", func() map[string]interface{} {
		c, f, cur := queryTimer()
		return map[string]interface{}{"on": timerOn, "current": float64(cur) / 10000, "finest": float64(f) / 10000, "coarsest": float64(c) / 10000}
	})
	w.Bind("mxSetTimer", func(on bool) error {
		if err := need(); err != nil {
			return err
		}
		if on {
			setTimerRes(5000, true)
		} else if timerOn {
			setTimerRes(5000, false)
		}
		timerOn = on
		return nil
	})
	w.Bind("mxFiveM", func() map[string]interface{} {
		n, high := 0, 0
		for _, p := range listProcs() {
			if isFiveMProc(p.Name) {
				n++
				if procPriority(p.Pid) == windows.HIGH_PRIORITY_CLASS {
					high++
				}
			}
		}
		return map[string]interface{}{"running": n > 0, "high": n > 0 && high == n}
	})
	w.Bind("mxBoostFiveM", func() (int, error) {
		if err := need(); err != nil {
			return 0, err
		}
		c := 0
		for _, p := range listProcs() {
			if isFiveMProc(p.Name) && setProcHigh(p.Pid) == nil {
				c++
			}
		}
		return c, nil
	})
	w.Bind("mxReboot", func() error {
		if err := need(); err != nil {
			return err
		}
		_, err := runHidden("shutdown", "/r", "/t", "5", "/c", "MEXXIC Tweaks: restart to finish applying tweaks")
		return err
	})
	w.Bind("mxOpenFolder", func() {
		os.MkdirAll(appDir, 0o755)
		runHidden("explorer.exe", appDir)
	})
	w.Bind("mxQuit", func() { w.Terminate() })
}

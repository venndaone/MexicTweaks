package main

import (
	"crypto/sha256"
	"crypto/subtle"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/jchv/go-webview2"
)

//go:embed ui.html
var uiHTML string

const appVersion = "1.1.0"

var (
	appDir     string
	tweaks     []*Tweak
	tweakByID  = map[string]*Tweak{}
	backups    = map[string]*Backup{}
	backupMu   sync.Mutex
	authed     bool
	timerOn    bool
	errNoLogin = errors.New("Nicht eingeloggt")
)

// ---------------- Storage ----------------

func loadJSON(name string, v interface{}) {
	b, err := os.ReadFile(filepath.Join(appDir, name))
	if err == nil {
		json.Unmarshal(b, v)
	}
}

func saveJSON(name string, v interface{}) error {
	os.MkdirAll(appDir, 0o755)
	b, _ := json.MarshalIndent(v, "", "  ")
	return os.WriteFile(filepath.Join(appDir, name), b, 0o600)
}

// ---------------- Auth ----------------

// Zugangspasswort ist fest eingebaut (nur als Hash gespeichert).
const pwSalt = "mexic-tweaks-v1"
const pwHash = "0f5682c05e4392150e8da3d420d2a7c3fd406483cf4b756c90344a37adefe226"

func hashPw(salt, pw string) string {
	h := []byte(salt + "|" + pw)
	for i := 0; i < 120000; i++ {
		s := sha256.Sum256(h)
		h = s[:]
	}
	return hex.EncodeToString(h)
}

func checkPw(pw string) bool {
	return subtle.ConstantTimeCompare([]byte(hashPw(pwSalt, pw)), []byte(pwHash)) == 1
}

// ---------------- Jobs ----------------

type Job struct {
	Running    bool     `json:"running"`
	Finished   bool     `json:"finished"`
	Title      string   `json:"title"`
	Done       int      `json:"done"`
	Total      int      `json:"total"`
	Current    string   `json:"current"`
	Log        []string `json:"log"`
	NeedReboot bool     `json:"needReboot"`
	Errors     int      `json:"errors"`
}

var (
	job   Job
	jobMu sync.Mutex
)

func jobUpdate(f func(j *Job)) { jobMu.Lock(); f(&job); jobMu.Unlock() }

func createRestorePoint() error {
	const srKey = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\SystemRestore`
	old := regSnapshot("HKLM", srKey, "SystemRestorePointCreationFrequency")
	regSet("HKLM", srKey, "SystemRestorePointCreationFrequency", false, "", 0)
	defer regRestore(old)
	out, err := runHidden("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command",
		`Enable-ComputerRestore -Drive "$env:SystemDrive\" -ErrorAction SilentlyContinue; Checkpoint-Computer -Description 'Mexic Tweaks' -RestorePointType MODIFY_SETTINGS -ErrorAction Stop`)
	if err != nil {
		msg := strings.TrimSpace(out)
		if len(msg) > 160 {
			msg = msg[:160] + "…"
		}
		if msg == "" {
			msg = err.Error()
		}
		return errors.New(msg)
	}
	return nil
}

func runJob(action string, ids []string, restorePoint bool) {
	var list []*Tweak
	if action == "restore" {
		// reverse order so dependent tweaks (USB on power plan) unwind cleanly
		for i := len(tweaks) - 1; i >= 0; i-- {
			for _, id := range ids {
				if tweaks[i].ID == id {
					list = append(list, tweaks[i])
				}
			}
		}
	} else {
		for _, t := range tweaks {
			for _, id := range ids {
				if t.ID == id {
					list = append(list, t)
				}
			}
		}
	}
	total := len(list)
	if restorePoint {
		total++
	}
	title := map[string]string{"apply": "Tweaks werden angewendet", "restore": "Originalzustand wird wiederhergestellt", "rp": "Wiederherstellungspunkt"}[action]
	jobUpdate(func(j *Job) { *j = Job{Running: true, Title: title, Total: total, Log: []string{}} })

	log := func(ok bool, s string) {
		jobUpdate(func(j *Job) {
			p := "✔ "
			if !ok {
				p = "✖ "
				j.Errors++
			}
			j.Log = append(j.Log, p+s)
			j.Done++
		})
	}

	if restorePoint {
		jobUpdate(func(j *Job) { j.Current = "Wiederherstellungspunkt wird erstellt …" })
		if err := createRestorePoint(); err != nil {
			log(false, "Wiederherstellungspunkt: "+err.Error())
		} else {
			log(true, "Wiederherstellungspunkt erstellt")
		}
	}

	for _, t := range list {
		t := t
		jobUpdate(func(j *Job) { j.Current = t.Name })
		var err error
		switch action {
		case "apply":
			backupMu.Lock()
			b, ok := backups[t.ID]
			if !ok {
				b = t.Snapshot()
				b.Date = time.Now().Format("02.01.2006 15:04")
				backups[t.ID] = b
				saveJSON("backup.json", backups)
			}
			backupMu.Unlock()
			err = t.Apply(b)
			backupMu.Lock()
			saveJSON("backup.json", backups) // may contain e.g. created power plan GUID
			backupMu.Unlock()
		case "restore":
			backupMu.Lock()
			b, ok := backups[t.ID]
			backupMu.Unlock()
			if !ok {
				err = errors.New("kein Backup vorhanden")
				break
			}
			err = t.Restore(b)
			if err == nil {
				backupMu.Lock()
				delete(backups, t.ID)
				saveJSON("backup.json", backups)
				backupMu.Unlock()
			}
		}
		if err != nil {
			log(false, t.Name+": "+err.Error())
		} else {
			log(true, t.Name)
			if t.Reboot {
				jobUpdate(func(j *Job) { j.NeedReboot = true })
			}
		}
		time.Sleep(120 * time.Millisecond)
	}
	jobUpdate(func(j *Job) { j.Running = false; j.Finished = true; j.Current = "" })
}

// ---------------- Bindings ----------------

type TweakState struct {
	*Tweak
	Active    bool   `json:"active"`
	HasBackup bool   `json:"hasBackup"`
	BackupAt  string `json:"backupAt"`
}

func bindAll(w webview2.WebView) {
	w.Bind("mxAuthInfo", func() map[string]interface{} {
		return map[string]interface{}{"admin": isAdmin(), "version": appVersion}
	})
	w.Bind("mxLogin", func(pw string) bool {
		authed = checkPw(pw)
		if !authed {
			time.Sleep(600 * time.Millisecond)
		}
		return authed
	})
	w.Bind("mxState", func() ([]TweakState, error) {
		if !authed {
			return nil, errNoLogin
		}
		backupMu.Lock()
		defer backupMu.Unlock()
		res := make([]TweakState, 0, len(tweaks))
		for _, t := range tweaks {
			b, ok := backups[t.ID]
			at := ""
			if ok {
				at = b.Date
			}
			res = append(res, TweakState{Tweak: t, Active: t.Active(), HasBackup: ok, BackupAt: at})
		}
		return res, nil
	})
	w.Bind("mxKeys", func() []bool {
		return []bool{keyDown('W'), keyDown('A'), keyDown('S'), keyDown('D'), keyDown(0x10), keyDown(0x20), keyDown(0x11)}
	})
	w.Bind("mxStartJob", func(action string, ids []string, rp bool) error {
		if !authed {
			return errNoLogin
		}
		jobMu.Lock()
		running := job.Running
		jobMu.Unlock()
		if running {
			return errors.New("Es läuft bereits ein Vorgang")
		}
		for _, id := range ids {
			if tweakByID[id] == nil {
				return errors.New("Unbekannter Tweak: " + id)
			}
		}
		if action != "apply" && action != "restore" && action != "rp" {
			return errors.New("Unbekannte Aktion")
		}
		go runJob(action, ids, rp || action == "rp")
		return nil
	})
	w.Bind("mxJob", func() Job { jobMu.Lock(); defer jobMu.Unlock(); j := job; j.Log = append([]string{}, job.Log...); return j })
	w.Bind("mxTimer", func() map[string]interface{} {
		c, f, cur := queryTimer()
		return map[string]interface{}{"on": timerOn, "current": float64(cur) / 10000, "finest": float64(f) / 10000, "coarsest": float64(c) / 10000}
	})
	w.Bind("mxSetTimer", func(on bool) error {
		if !authed {
			return errNoLogin
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
			l := strings.ToLower(p.Name)
			if strings.HasPrefix(l, "fivem") && strings.HasSuffix(l, "gtaprocess.exe") {
				n++
				if procPriority(p.Pid) == 0x80 {
					high++
				}
			}
		}
		return map[string]interface{}{"running": n > 0, "high": n > 0 && high == n}
	})
	w.Bind("mxBoostFiveM", func() (int, error) {
		if !authed {
			return 0, errNoLogin
		}
		c := 0
		for _, p := range listProcs() {
			l := strings.ToLower(p.Name)
			if strings.HasPrefix(l, "fivem") && strings.HasSuffix(l, "gtaprocess.exe") {
				if setProcHigh(p.Pid) == nil {
					c++
				}
			}
		}
		return c, nil
	})
	w.Bind("mxReboot", func() error {
		if !authed {
			return errNoLogin
		}
		_, err := runHidden("shutdown", "/r", "/t", "5", "/c", "Mexic Tweaks: Neustart für volle Wirkung")
		return err
	})
	w.Bind("mxOpenFolder", func() {
		os.MkdirAll(appDir, 0o755)
		runHidden("explorer.exe", appDir)
	})
	w.Bind("mxQuit", func() { w.Terminate() })

}

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
	loadJSON("backup.json", &backups)
	if backups == nil {
		backups = map[string]*Backup{}
	}

	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		local = appDir
	}
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		AutoFocus: true,
		DataPath:  filepath.Join(local, "MexxicTweaks", "WebView2"),
		WindowOptions: webview2.WindowOptions{
			Title: "Mexic Tweaks", Width: 1280, Height: 820, IconId: 1, Center: true,
		},
	})
	if w == nil {
		messageBox("Mexic Tweaks", "Die Microsoft Edge WebView2 Runtime fehlt.\n\nInstalliere sie kostenlos von:\nhttps://developer.microsoft.com/microsoft-edge/webview2/\n\nund starte Mexic Tweaks danach neu.")
		return
	}
	defer w.Destroy()
	w.SetSize(1100, 720, webview2.HintMin)
	bindAll(w)
	w.SetHtml(uiHTML)
	w.Run()

	if timerOn {
		setTimerRes(5000, false)
	}
}

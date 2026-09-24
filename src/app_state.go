package main

// Services: settings, login, activity log, restart tracking, system info.

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows/registry"
)

// ---------------- Settings ----------------

type Settings struct {
	Lang         string    `json:"lang"` // "de" | "en" | "" (auto)
	RestartSince time.Time `json:"restartSince"`
}

var (
	settings   Settings
	settingsMu sync.Mutex
)

func loadJSON(name string, v interface{}) {
	if b, err := os.ReadFile(filepath.Join(appDir, name)); err == nil {
		json.Unmarshal(b, v)
	}
}

func saveJSON(name string, v interface{}) error {
	os.MkdirAll(appDir, 0o755)
	b, _ := json.MarshalIndent(v, "", "  ")
	return os.WriteFile(filepath.Join(appDir, name), b, 0o600)
}

func saveSettings() {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	saveJSON("settings.json", settings)
}

// ---------------- Login ----------------
// The access password is built in and only stored as an iterated SHA-256 hash.

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

// ---------------- Restart tracking ----------------

func bootTime() time.Time { return time.Now().Add(-time.Duration(uptimeMs()) * time.Millisecond) }

func markRestartNeeded() {
	settingsMu.Lock()
	settings.RestartSince = time.Now()
	settingsMu.Unlock()
	saveSettings()
}

// restartNeeded is true until Windows was rebooted after the last change that needs it.
func restartNeeded() bool {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	return !settings.RestartSince.IsZero() && settings.RestartSince.After(bootTime())
}

// ---------------- Activity ----------------

var (
	activity   []LogLine
	activityMu sync.Mutex
)

func addActivity(l LogLine) {
	activityMu.Lock()
	activity = append(activity, l)
	if len(activity) > 80 {
		activity = activity[len(activity)-80:]
	}
	activityMu.Unlock()
}

func saveActivity() {
	activityMu.Lock()
	defer activityMu.Unlock()
	saveJSON("activity.json", activity)
}

// ---------------- System info ----------------

type SysInfo struct {
	OS        string  `json:"os"`
	Build     string  `json:"build"`
	CPU       string  `json:"cpu"`
	Threads   int     `json:"threads"`
	GPU       string  `json:"gpu"`
	RAMTotal  float64 `json:"ramTotal"`
	RAMUsed   float64 `json:"ramUsed"`
	RAMLoad   uint32  `json:"ramLoad"`
	PowerPlan string  `json:"powerPlan"`
	UptimeMin uint64  `json:"uptimeMin"`
	Admin     bool    `json:"admin"`
	Computer  string  `json:"computer"`
}

func regStr(root registry.Key, path, name string) string {
	k, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	s, _, _ := k.GetStringValue(name)
	return strings.TrimSpace(s)
}

var (
	planName_     string
	planAt        time.Time
	sysStatic     SysInfo
	sysStaticOnce sync.Once
)

func systemInfo() SysInfo {
	sysStaticOnce.Do(func() {
		const cv = `SOFTWARE\Microsoft\Windows NT\CurrentVersion`
		prod := regStr(registry.LOCAL_MACHINE, cv, "ProductName")
		build := regStr(registry.LOCAL_MACHINE, cv, "CurrentBuild")
		var b int
		fmt.Sscan(build, &b)
		if b >= 22000 {
			prod = strings.Replace(prod, "Windows 10", "Windows 11", 1)
		}
		disp := regStr(registry.LOCAL_MACHINE, cv, "DisplayVersion")
		sysStatic.OS = strings.TrimSpace(prod + " " + disp)
		sysStatic.Build = build
		sysStatic.CPU = regStr(registry.LOCAL_MACHINE, `HARDWARE\DESCRIPTION\System\CentralProcessor\0`, "ProcessorNameString")
		sysStatic.Threads = runtime.NumCPU()
		for i := 0; i < 10; i++ {
			d := regStr(registry.LOCAL_MACHINE, fmt.Sprintf(`SYSTEM\CurrentControlSet\Control\Class\{4d36e968-e325-11ce-bfc1-08002be10318}\%04d`, i), "DriverDesc")
			if d != "" && !strings.Contains(d, "Basic") && !strings.Contains(d, "Remote") {
				sysStatic.GPU = d
				break
			}
		}
		sysStatic.Computer, _ = os.Hostname()
	})
	s := sysStatic
	total, avail, load := memoryStatus()
	s.RAMTotal = float64(total) / (1 << 30)
	s.RAMUsed = float64(total-avail) / (1 << 30)
	s.RAMLoad = load
	if time.Since(planAt) > 10*time.Second { // powercfg is slow: cache for 10 s
		_, raw := activeScheme()
		planName_, planAt = schemeName(raw), time.Now()
	}
	s.PowerPlan = planName_
	s.UptimeMin = uptimeMs() / 60000
	s.Admin = isAdmin()
	return s
}

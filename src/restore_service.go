package main

// Restore: Windows System Restore points (create + list).

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

const rpDescription = "MEXXIC Tweaks"

// createRestorePoint creates a real Windows restore point. The 24h frequency
// limit is lifted temporarily and restored afterwards.
func createRestorePoint() error {
	const srKey = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\SystemRestore`
	old := regSnapshot("HKLM", srKey, "SystemRestorePointCreationFrequency")
	regSet("HKLM", srKey, "SystemRestorePointCreationFrequency", false, "", 0)
	defer regRestore(old)
	out, err := runHidden("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command",
		`$ErrorActionPreference='Stop'; Enable-ComputerRestore -Drive "$env:SystemDrive\" -ErrorAction SilentlyContinue; Checkpoint-Computer -Description '`+rpDescription+`' -RestorePointType MODIFY_SETTINGS`)
	if err != nil {
		msg := strings.TrimSpace(out)
		if i := strings.Index(msg, "\n"); i > 0 {
			msg = strings.TrimSpace(msg[:i])
		}
		if len(msg) > 180 {
			msg = msg[:180] + "…"
		}
		if msg == "" {
			msg = err.Error()
		}
		return errors.New(msg)
	}
	return nil
}

type RestorePoint struct {
	Seq         int       `json:"seq"`
	Description string    `json:"description"`
	Date        time.Time `json:"date"`
	Mexxic      bool      `json:"mexxic"`
}

var (
	rpMu      sync.Mutex
	rpItems   []RestorePoint
	rpLoading bool
	rpLoaded  bool
	rpErr     string
)

// refreshRestorePoints loads the list in the background (PowerShell is slow).
func refreshRestorePoints() {
	rpMu.Lock()
	if rpLoading {
		rpMu.Unlock()
		return
	}
	rpLoading = true
	rpMu.Unlock()
	go func() {
		out, err := runHidden("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command",
			`Get-ComputerRestorePoint | Select-Object SequenceNumber,Description,CreationTime | ConvertTo-Json -Compress`)
		var items []RestorePoint
		msg := ""
		if err != nil {
			msg = strings.TrimSpace(out)
			if msg == "" {
				msg = err.Error()
			}
		} else if s := strings.TrimSpace(out); s != "" {
			type raw struct {
				SequenceNumber int
				Description    string
				CreationTime   string
			}
			var list []raw
			if strings.HasPrefix(s, "{") {
				var one raw
				json.Unmarshal([]byte(s), &one)
				list = []raw{one}
			} else {
				json.Unmarshal([]byte(s), &list)
			}
			for _, r := range list {
				// WMI time: 20260924101500.000000-000
				t, _ := time.ParseInLocation("20060102150405", firstN(r.CreationTime, 14), time.UTC)
				items = append(items, RestorePoint{Seq: r.SequenceNumber, Description: r.Description, Date: t.Local(),
					Mexxic: strings.Contains(strings.ToLower(r.Description), "mexxic") || strings.Contains(strings.ToLower(r.Description), "mexic")})
			}
			sort.Slice(items, func(i, j int) bool { return items[i].Date.After(items[j].Date) })
		}
		rpMu.Lock()
		rpItems, rpErr, rpLoading, rpLoaded = items, msg, false, true
		rpMu.Unlock()
	}()
}

func firstN(s string, n int) string {
	if len(s) < n {
		return s
	}
	return s[:n]
}

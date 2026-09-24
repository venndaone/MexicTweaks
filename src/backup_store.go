package main

// Backup: persistent store of original values and apply sessions.
// File: %APPDATA%\MexxicTweaks\backup.json (written atomically).

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	store   = &Store{Version: 2, Tweaks: map[string]*Backup{}}
	storeMu sync.Mutex
)

func storePath() string { return filepath.Join(appDir, "backup.json") }

// loadStore reads the backup file and migrates the v1 format (plain map).
func loadStore() {
	raw, err := os.ReadFile(storePath())
	if err != nil {
		return
	}
	var s Store
	if json.Unmarshal(raw, &s) == nil && s.Version >= 2 {
		if s.Tweaks == nil {
			s.Tweaks = map[string]*Backup{}
		}
		store = &s
		return
	}
	// v1: map[tweakID]*Backup
	var legacy map[string]*Backup
	if json.Unmarshal(raw, &legacy) != nil || len(legacy) == 0 {
		return
	}
	sess := &Session{ID: "legacy", Date: time.Now()}
	for id, b := range legacy {
		if t, e := time.ParseInLocation("02.01.2006 15:04", b.Date, time.Local); e == nil {
			b.CreatedAt, b.AppliedAt = t, t
			if sess.Date.After(t) {
				sess.Date = t
			}
		}
		b.SessionID = sess.ID
		name, cat := id, ""
		if t := tweakByID[id]; t != nil {
			name, cat = t.Name, t.Category
		}
		sess.Tweaks = append(sess.Tweaks, &SessionTweak{ID: id, Name: name, Category: cat, OK: true})
	}
	store = &Store{Version: 2, Tweaks: legacy, Sessions: []*Session{sess}}
	saveStoreLocked()
}

// saveStoreLocked writes the store atomically (temp file + rename). Caller holds storeMu.
func saveStoreLocked() error {
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	tmp := storePath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, storePath()); err != nil {
		return err
	}
	// verify it can be read back
	chk, err := os.ReadFile(storePath())
	if err != nil || len(chk) != len(data) {
		return errors.New("backup file verification failed")
	}
	return nil
}

func newSessionID() string { return fmt.Sprintf("s%d", time.Now().UnixNano()) }

func findSession(id string) *Session {
	for _, s := range store.Sessions {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// markRestored flags every session entry of a tweak as restored.
func markRestored(id string) {
	for _, s := range store.Sessions {
		for _, st := range s.Tweaks {
			if st.ID == id {
				st.Restored = true
			}
		}
	}
}

func lastBackupTime() time.Time {
	var t time.Time
	for _, s := range store.Sessions {
		if s.Date.After(t) {
			t = s.Date
		}
	}
	return t
}

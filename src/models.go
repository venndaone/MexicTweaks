package main

// Models: data structures shared by tweaks, backup, restore and the UI bridge.

import "time"

// KV is one observable value of a tweak (label + current value as text).
type KV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Change documents one modification: what, original value, new value.
type Change struct {
	Target   string `json:"target"`
	Original string `json:"original"`
	New      string `json:"new"`
}

// Backup holds everything needed to undo one tweak. It is written to disk
// BEFORE the tweak changes anything.
type Backup struct {
	Date      string            `json:"date,omitempty"` // legacy (v1) display date
	CreatedAt time.Time         `json:"createdAt"`
	AppliedAt time.Time         `json:"appliedAt,omitempty"`
	SessionID string            `json:"sessionId,omitempty"`
	Regs      []RegVal          `json:"regs,omitempty"`
	Nums      map[string]int64  `json:"nums,omitempty"`
	Strs      map[string]string `json:"strs,omitempty"`
	Changes   []Change          `json:"changes,omitempty"`
}

// SessionTweak is one tweak inside an apply session.
type SessionTweak struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Category string   `json:"category"`
	OK       bool     `json:"ok"`
	Error    string   `json:"error,omitempty"`
	Changes  []Change `json:"changes,omitempty"`
	Restored bool     `json:"restored"`
}

// Session is one "MEXXIC Backup": a single APPLY SELECTED run.
type Session struct {
	ID              string          `json:"id"`
	Date            time.Time       `json:"date"`
	RestorePoint    bool            `json:"restorePoint"`
	RestorePointMsg string          `json:"restorePointMsg,omitempty"`
	Tweaks          []*SessionTweak `json:"tweaks"`
}

// Store is the on-disk backup file (%APPDATA%\MexxicTweaks\backup.json).
type Store struct {
	Version  int                `json:"version"`
	Tweaks   map[string]*Backup `json:"tweaks"`
	Sessions []*Session         `json:"sessions"`
}

// LogLine is one status line shown while a job runs and in Recent Activity.
// Code + Args are translated by the UI (English / Deutsch); Text is a fallback.
type LogLine struct {
	Kind string    `json:"kind"` // ok | err | warn | info
	Code string    `json:"code"`
	Args []string  `json:"args,omitempty"`
	Text string    `json:"text"`
	Time time.Time `json:"time"`
}

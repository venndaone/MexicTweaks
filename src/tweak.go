package main

// Tweaks: the Tweak type. Every optimization is one Tweak object with
// Name, Description, Category, current state, admin/restart flags and
// Apply() / Restore(). New tweaks are added in the tweaks_<category>.go files.

import (
	"errors"
	"strings"
)

// rv describes one registry value a tweak sets.
type rv struct {
	Root, Path, Name string
	Sz               bool
	S                string
	D                uint32
}

func dw(root, path, name string, d uint32) rv { return rv{root, path, name, false, "", d} }
func sz(root, path, name, s string) rv        { return rv{root, path, name, true, s, 0} }

const (
	CatInput   = "Input"
	CatNetwork = "Network"
	CatWindows = "Windows"
	CatGaming  = "Gaming"
	CatFiveM   = "FiveM"
)

var categoryOrder = []string{CatInput, CatNetwork, CatWindows, CatGaming, CatFiveM}

type Tweak struct {
	ID              string `json:"id"`
	Category        string `json:"category"`
	Name            string `json:"name"`          // English
	Description     string `json:"description"`   // English
	NameDE          string `json:"nameDe"`        // Deutsch
	DescriptionDE   string `json:"descriptionDe"` // Deutsch
	Impact          string `json:"impact"`        // noticeable | subtle | comfort
	Icon            string `json:"icon"`
	RequiresRestart bool   `json:"requiresRestart"`
	RequiresAdmin   bool   `json:"requiresAdmin"`

	regs    []rv
	snap    func(b *Backup)       // save non-registry originals
	prepare func(b *Backup)       // refresh backup right before apply (order-dependent tweaks)
	apply   func(b *Backup) error // non-registry changes
	restore func(b *Backup) error // non-registry undo
	check   func() bool           // non-registry "is active"
	state   func() []KV           // non-registry observable values
	stateOv func() []KV           // replaces the automatic registry state (e.g. very long lists)
}

// finalize derives flags that can be computed from the definition.
func (t *Tweak) finalize() {
	for _, r := range t.regs {
		if r.Root == "HKLM" {
			t.RequiresAdmin = true
		}
	}
}

func (t *Tweak) Snapshot() *Backup {
	b := &Backup{Nums: map[string]int64{}, Strs: map[string]string{}}
	for _, r := range t.regs {
		b.Regs = append(b.Regs, regSnapshot(r.Root, r.Path, r.Name))
	}
	if t.snap != nil {
		t.snap(b)
	}
	return b
}

func (t *Tweak) Apply(b *Backup) error {
	var errs []string
	for _, r := range t.regs {
		if err := regSet(r.Root, r.Path, r.Name, r.Sz, r.S, r.D); err != nil {
			errs = append(errs, r.Name+": "+err.Error())
		}
	}
	if t.apply != nil {
		if err := t.apply(b); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func (t *Tweak) Restore(b *Backup) error {
	var errs []string
	if t.restore != nil {
		if err := t.restore(b); err != nil {
			errs = append(errs, err.Error())
		}
	}
	for _, v := range b.Regs {
		if err := regRestore(v); err != nil {
			errs = append(errs, v.Name+": "+err.Error())
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// Active reports the CURRENT real system state (not a saved flag).
func (t *Tweak) Active() bool {
	for _, r := range t.regs {
		if !regMatches(r.Root, r.Path, r.Name, r.Sz, r.S, r.D) {
			return false
		}
	}
	if t.check != nil {
		return t.check()
	}
	return true
}

// State returns every value this tweak touches, as currently set in Windows.
func (t *Tweak) State() []KV {
	if t.stateOv != nil {
		return t.stateOv()
	}
	var res []KV
	for _, r := range t.regs {
		res = append(res, KV{regLabel(r.Root, r.Path, r.Name), regDisplay(r.Root, r.Path, r.Name)})
	}
	if t.state != nil {
		res = append(res, t.state()...)
	}
	return res
}

// diffChanges pairs a before/after state into Change records.
func diffChanges(before, after []KV) []Change {
	idx := map[string]string{}
	for _, kv := range before {
		idx[kv.Key] = kv.Value
	}
	var out []Change
	for _, kv := range after {
		out = append(out, Change{Target: kv.Key, Original: idx[kv.Key], New: kv.Value})
	}
	return out
}

// ---- registry of all tweaks ----

func buildTweaks() []*Tweak {
	var all []*Tweak
	all = append(all, inputTweaks()...)
	all = append(all, networkTweaks()...)
	all = append(all, windowsTweaks()...)
	all = append(all, gamingTweaks()...)
	all = append(all, fivemTweaks()...)
	for _, t := range all {
		t.finalize()
	}
	return all
}

package main

import (
	"errors"
	"fmt"
	"strings"
)

// Backup holds the original state of one tweak, captured before it is first applied.
type Backup struct {
	Date string            `json:"date"`
	Regs []RegVal          `json:"regs,omitempty"`
	Nums map[string]int64  `json:"nums,omitempty"`
	Strs map[string]string `json:"strs,omitempty"`
}

type rv struct {
	Root, Path, Name string
	Sz               bool
	S                string
	D                uint32
}

func dw(root, path, name string, d uint32) rv { return rv{root, path, name, false, "", d} }
func sz(root, path, name, s string) rv       { return rv{root, path, name, true, s, 0} }

type Tweak struct {
	ID     string `json:"id"`
	Cat    string `json:"cat"`
	Name   string `json:"name"`
	Desc   string `json:"desc"`
	Effect string `json:"effect"` // "spürbar" | "leicht" | "komfort"
	Icon   string `json:"icon"`
	Reboot bool   `json:"reboot"`

	regs    []rv
	snap    func(b *Backup)
	apply   func(b *Backup) error
	restore func(b *Backup) error
	check   func() bool
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

const (
	mmcss       = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Multimedia\SystemProfile`
	mmcssGames  = mmcss + `\Tasks\Games`
	ifeo        = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Image File Execution Options\`
	usbSub      = "2a737441-1930-4402-8d77-b2bebba308a3"
	usbSuspend  = "48e6b7a6-50f5-4782-a5d4-53bb8f07e226"
	ultimateTpl = "e9a42b02-d5df-448d-aa00-03f14749eb61"
	highPerf    = "8c5e7fda-e8bf-4a96-9a85-a6e23a8c635c"
	planName    = "Mexic Tweaks Ultimate"
)

var fivemBuilds = []string{"", "b2060_", "b2189_", "b2372_", "b2545_", "b2612_", "b2699_", "b2802_", "b2944_", "b3095_", "b3258_", "b3323_", "b3407_", "b3570_"}

func usbValues(scheme string) (ac, dc int64, ok bool) {
	out, err := runHidden("powercfg", "/query", scheme, usbSub, usbSuspend)
	if err != nil {
		return 1, 1, false
	}
	m := hexRe.FindAllString(out, -1)
	if len(m) < 2 {
		return 1, 1, false
	}
	var a, d int64
	fmt.Sscanf(m[len(m)-2], "0x%x", &a)
	fmt.Sscanf(m[len(m)-1], "0x%x", &d)
	return a, d, true
}

func buildTweaks() []*Tweak {
	var fivemRegs []rv
	for _, b := range fivemBuilds {
		fivemRegs = append(fivemRegs, dw("HKLM", ifeo+"FiveM_"+b+"GTAProcess.exe\\PerfOptions", "CpuPriorityClass", 3))
	}

	return []*Tweak{
		// ---------------- Tastatur ----------------
		{
			ID: "kb_popups", Cat: "Tastatur", Icon: "shift", Effect: "spürbar",
			Name: "Shift-Popups AUS",
			Desc: "Einrastfunktion (5× Shift), Anschlagverzögerung (8 Sek. Shift halten) und Statustasten-Hotkey aus. Kein Popup mehr, das dich beim Sprinten aus dem Spiel wirft.",
			snap: func(b *Backup) {
				f := getFilter()
				b.Nums["sticky"], b.Nums["toggle"] = int64(getSticky()), int64(getToggle())
				b.Nums["fk_flags"], b.Nums["fk_wait"], b.Nums["fk_delay"] = int64(f.Flags), int64(f.Wait), int64(f.Delay)
				b.Nums["fk_repeat"], b.Nums["fk_bounce"] = int64(f.Repeat), int64(f.Bounce)
			},
			apply: func(b *Backup) error {
				setSticky(getSticky() &^ 5)
				setToggle(getToggle() &^ 5)
				f := getFilter()
				f.Flags &^= 5
				setFilter(f)
				return nil
			},
			restore: func(b *Backup) error {
				setSticky(uint32(b.Nums["sticky"]))
				setToggle(uint32(b.Nums["toggle"]))
				setFilter(fkStruct{Flags: uint32(b.Nums["fk_flags"]), Wait: uint32(b.Nums["fk_wait"]), Delay: uint32(b.Nums["fk_delay"]), Repeat: uint32(b.Nums["fk_repeat"]), Bounce: uint32(b.Nums["fk_bounce"])})
				return nil
			},
			check: func() bool { return getSticky()&5 == 0 && getToggle()&5 == 0 && getFilter().Flags&5 == 0 },
		},
		{
			ID: "kb_repeat", Cat: "Tastatur", Icon: "keyboard", Effect: "komfort",
			Name: "Tastenwiederholung MAX",
			Desc: "Kürzeste Verzögerung und schnellste Wiederholrate. Wirkt beim Tippen (Chat, F8-Konsole), nicht auf gehaltene WASD-Bewegung.",
			snap: func(b *Backup) { b.Nums["delay"], b.Nums["speed"] = int64(getKbDelay()), int64(getKbSpeed()) },
			apply: func(b *Backup) error { setKbDelay(0); setKbSpeed(31); return nil },
			restore: func(b *Backup) error {
				setKbDelay(uint32(b.Nums["delay"]))
				setKbSpeed(uint32(b.Nums["speed"]))
				return nil
			},
			check: func() bool { return getKbDelay() == 0 && getKbSpeed() == 31 },
		},
		{
			ID: "kb_lang", Cat: "Tastatur", Icon: "globe", Effect: "spürbar", Reboot: true,
			Name: "Alt+Shift Layoutwechsel AUS",
			Desc: "Verhindert, dass Windows beim Sprinten mit Alt/Strg+Shift plötzlich auf ein anderes Tastaturlayout (z. B. QWERTY) springt.",
			regs: []rv{
				sz("HKCU", `Keyboard Layout\Toggle`, "Hotkey", "3"),
				sz("HKCU", `Keyboard Layout\Toggle`, "Language Hotkey", "3"),
				sz("HKCU", `Keyboard Layout\Toggle`, "Layout Hotkey", "3"),
			},
		},
		{
			ID: "kb_snip", Cat: "Tastatur", Icon: "camera", Effect: "komfort",
			Name: "Druck-Taste ohne Snipping Tool",
			Desc: "Die Druck-Taste öffnet nicht mehr das Snipping Tool über dem Spiel.",
			regs: []rv{dw("HKCU", `Control Panel\Keyboard`, "PrintScreenKeyForSnippingEnabled", 0)},
		},

		// ---------------- Maus ----------------
		{
			ID: "mouse_accel", Cat: "Maus", Icon: "mouse", Effect: "spürbar",
			Name: "Mausbeschleunigung AUS",
			Desc: "Deaktiviert „Zeigerbeschleunigung verbessern“. Mausweg = Zielweg, 1:1. Der Tweak mit dem größten spürbaren Effekt fürs Aiming.",
			snap: func(b *Backup) {
				m := getMouse()
				b.Nums["m0"], b.Nums["m1"], b.Nums["m2"] = int64(m[0]), int64(m[1]), int64(m[2])
			},
			apply: func(b *Backup) error { setMouse([3]int32{0, 0, 0}); return nil },
			restore: func(b *Backup) error {
				setMouse([3]int32{int32(b.Nums["m0"]), int32(b.Nums["m1"]), int32(b.Nums["m2"])})
				return nil
			},
			check: func() bool { return getMouse()[2] == 0 },
		},

		// ---------------- System ----------------
		{
			ID: "power_plan", Cat: "System", Icon: "bolt", Effect: "leicht",
			Name: "Ultimative Leistung (Energieplan)",
			Desc: "Eigener Energieplan „Mexic Tweaks Ultimate“: CPU taktet nicht mehr runter, keine Stromspar-Verzögerungen. Am Laptop nur mit Netzteil sinnvoll.",
			snap: func(b *Backup) { g, _ := activeScheme(); b.Strs["active"] = g },
			apply: func(b *Backup) error {
				if c := b.Strs["created"]; c != "" {
					if _, err := runHidden("powercfg", "-setactive", c); err == nil {
						return nil
					}
				}
				out, err := runHidden("powercfg", "-duplicatescheme", ultimateTpl)
				g := strings.ToLower(guidRe.FindString(out))
				if err == nil && g != "" && g != ultimateTpl {
					b.Strs["created"] = g
					runHidden("powercfg", "-changename", g, planName, "Erstellt von Mexic Tweaks")
					_, err = runHidden("powercfg", "-setactive", g)
					return err
				}
				if _, err := runHidden("powercfg", "-setactive", highPerf); err != nil {
					return errors.New("Energieplan wird von diesem PC nicht unterstützt")
				}
				return nil
			},
			restore: func(b *Backup) error {
				if a := b.Strs["active"]; a != "" {
					runHidden("powercfg", "-setactive", a)
				}
				if c := b.Strs["created"]; c != "" {
					runHidden("powercfg", "-delete", c)
				}
				return nil
			},
			check: func() bool {
				g, raw := activeScheme()
				return strings.Contains(raw, planName) || g == highPerf || g == ultimateTpl
			},
		},
		{
			ID: "usb_suspend", Cat: "System", Icon: "usb", Effect: "leicht",
			Name: "USB-Energiesparen AUS",
			Desc: "Tastatur und Maus werden von Windows nie in den Energiesparmodus geschickt. Kein verzögerter erster Input nach Pausen.",
			snap: func(b *Backup) {
				g, _ := activeScheme()
				b.Strs["scheme"] = g
				b.Nums["ac"], b.Nums["dc"], _ = usbValues(g)
			},
			apply: func(b *Backup) error {
				runHidden("powercfg", "/setacvalueindex", "SCHEME_CURRENT", usbSub, usbSuspend, "0")
				runHidden("powercfg", "/setdcvalueindex", "SCHEME_CURRENT", usbSub, usbSuspend, "0")
				_, err := runHidden("powercfg", "/setactive", "SCHEME_CURRENT")
				return err
			},
			restore: func(b *Backup) error {
				s := b.Strs["scheme"]
				if s == "" {
					s = "SCHEME_CURRENT"
				}
				runHidden("powercfg", "/setacvalueindex", s, usbSub, usbSuspend, fmt.Sprint(b.Nums["ac"]))
				runHidden("powercfg", "/setdcvalueindex", s, usbSub, usbSuspend, fmt.Sprint(b.Nums["dc"]))
				runHidden("powercfg", "/setactive", "SCHEME_CURRENT")
				return nil
			},
			check: func() bool { ac, _, ok := usbValues("SCHEME_CURRENT"); return ok && ac == 0 },
		},
		{
			ID: "cpu_fg", Cat: "System", Icon: "cpu", Effect: "leicht",
			Name: "CPU-Priorität für Vordergrund",
			Desc: "Win32PrioritySeparation = 0x26: Das aktive Fenster (dein Spiel) bekommt kürzere, bevorzugte CPU-Zeitscheiben.",
			regs: []rv{dw("HKLM", `SYSTEM\CurrentControlSet\Control\PriorityControl`, "Win32PrioritySeparation", 0x26)},
		},
		{
			ID: "sys_resp", Cat: "System", Icon: "rocket", Effect: "leicht",
			Name: "System-Reaktionsfähigkeit MAX",
			Desc: "SystemResponsiveness auf das Minimum (10). Hintergrund-Tasks reservieren weniger CPU für sich.",
			regs: []rv{dw("HKLM", mmcss, "SystemResponsiveness", 10)},
		},
		{
			ID: "game_prio", Cat: "System", Icon: "gamepad", Effect: "leicht",
			Name: "Spiele-Priorität ULTRA",
			Desc: "Multimedia-Scheduler: Spiele bekommen höchste GPU-Priorität (8), CPU-Priorität 6, Kategorie „High“.",
			regs: []rv{
				dw("HKLM", mmcssGames, "GPU Priority", 8),
				dw("HKLM", mmcssGames, "Priority", 6),
				sz("HKLM", mmcssGames, "Scheduling Category", "High"),
				sz("HKLM", mmcssGames, "SFIO Priority", "High"),
			},
		},
		{
			ID: "net_throttle", Cat: "System", Icon: "wifi", Effect: "leicht", Reboot: true,
			Name: "Netzwerk-Drosselung AUS",
			Desc: "Schaltet die Windows-Paketdrosselung bei Multimedia-Last ab (NetworkThrottlingIndex).",
			regs: []rv{dw("HKLM", mmcss, "NetworkThrottlingIndex", 0xFFFFFFFF)},
		},
		{
			ID: "game_dvr", Cat: "System", Icon: "record", Effect: "spürbar",
			Name: "Xbox Game DVR AUS",
			Desc: "Stoppt die Hintergrund-Aufnahme der Xbox Game Bar. Spart FPS und Frametime-Spikes.",
			regs: []rv{
				dw("HKCU", `System\GameConfigStore`, "GameDVR_Enabled", 0),
				dw("HKCU", `Software\Microsoft\Windows\CurrentVersion\GameDVR`, "AppCaptureEnabled", 0),
				dw("HKLM", `SOFTWARE\Policies\Microsoft\Windows\GameDVR`, "AllowGameDVR", 0),
			},
		},
		{
			ID: "game_mode", Cat: "System", Icon: "star", Effect: "leicht",
			Name: "Windows-Spielmodus AN",
			Desc: "Windows priorisiert das laufende Spiel und blockiert Treiber-Updates oder Benachrichtigungen während du spielst.",
			regs: []rv{
				dw("HKCU", `Software\Microsoft\GameBar`, "AutoGameModeEnabled", 1),
				dw("HKCU", `Software\Microsoft\GameBar`, "AllowAutoGameMode", 1),
			},
		},
		{
			ID: "timer_global", Cat: "System", Icon: "clock", Effect: "leicht", Reboot: true,
			Name: "Globale Timer-Auflösung erlauben",
			Desc: "Nötig für Windows 11, damit „Timer 0,5 ms“ (rechts) für das ganze System gilt und nicht nur für Mexic Tweaks.",
			regs: []rv{dw("HKLM", `SYSTEM\CurrentControlSet\Control\Session Manager\kernel`, "GlobalTimerResolutionRequests", 1)},
		},

		// ---------------- FiveM ----------------
		{
			ID: "fivem_prio", Cat: "FiveM", Icon: "car", Effect: "spürbar",
			Name: "FiveM immer hohe Priorität",
			Desc: "FiveM (alle Game-Builds) startet automatisch mit CPU-Priorität „Hoch“. Weniger Ruckler, wenn im Hintergrund etwas läuft.",
			regs: fivemRegs,
		},
	}
}

package main

// Tweaks › Windows: power, USB, scheduler, timer.

import (
	"errors"
	"fmt"
	"strings"
)

const (
	mmcss       = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Multimedia\SystemProfile`
	usbSub      = "2a737441-1930-4402-8d77-b2bebba308a3"
	usbSuspend  = "48e6b7a6-50f5-4782-a5d4-53bb8f07e226"
	ultimateTpl = "e9a42b02-d5df-448d-aa00-03f14749eb61"
	highPerf    = "8c5e7fda-e8bf-4a96-9a85-a6e23a8c635c"
	planName    = "MEXXIC Ultimate"
	planNameV1  = "Mexic Tweaks Ultimate" // created by v1.x
)

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

func windowsTweaks() []*Tweak {
	return []*Tweak{
		{
			ID: "power_plan", Category: CatWindows, Icon: "bolt", Impact: "subtle", RequiresAdmin: true,
			Name:          "Ultimate Performance Power Plan",
			Description:   "Creates and activates “MEXXIC Ultimate”: no CPU down-clocking, no power-saving delays. On laptops only useful when plugged in.",
			NameDE:        "Energieplan „Ultimative Leistung“",
			DescriptionDE: "Erstellt und aktiviert „MEXXIC Ultimate“: kein Heruntertakten der CPU, keine Stromspar-Verzögerungen. Am Laptop nur mit Netzteil sinnvoll.",
			snap:          func(b *Backup) { g, _ := activeScheme(); b.Strs["active"] = g },
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
					runHidden("powercfg", "-changename", g, planName, "Created by MEXXIC Tweaks")
					_, err = runHidden("powercfg", "-setactive", g)
					return err
				}
				if _, err := runHidden("powercfg", "-setactive", highPerf); err != nil {
					return errors.New("power plan not supported on this PC")
				}
				return nil
			},
			restore: func(b *Backup) error {
				if a := b.Strs["active"]; a != "" {
					if _, err := runHidden("powercfg", "-setactive", a); err != nil {
						return errors.New("could not re-activate previous power plan")
					}
				}
				if c := b.Strs["created"]; c != "" {
					runHidden("powercfg", "-delete", c)
				}
				return nil
			},
			check: func() bool {
				g, raw := activeScheme()
				return strings.Contains(raw, planName) || strings.Contains(raw, planNameV1) || g == highPerf || g == ultimateTpl
			},
			state: func() []KV {
				g, raw := activeScheme()
				return []KV{{"Active power plan (powercfg)", schemeName(raw) + " {" + g + "}"}}
			},
		},
		{
			ID: "usb_suspend", Category: CatWindows, Icon: "usb", Impact: "subtle", RequiresAdmin: true,
			Name:          "Disable USB Selective Suspend",
			Description:   "Keyboard and mouse are never put into power-saving mode. No delayed first input after idle.",
			NameDE:        "USB-Energiesparen aus",
			DescriptionDE: "Tastatur und Maus werden nie in den Energiesparmodus geschickt. Kein verzögerter erster Input nach Pausen.",
			snap: func(b *Backup) {
				g, _ := activeScheme()
				b.Strs["scheme"] = g
				b.Nums["ac"], b.Nums["dc"], _ = usbValues(g)
			},
			// If another tweak switched the power plan in the same run, back up the values of the plan we will actually change.
			prepare: func(b *Backup) {
				if g, _ := activeScheme(); g != "" && g != b.Strs["scheme"] {
					b.Strs["scheme"] = g
					b.Nums["ac"], b.Nums["dc"], _ = usbValues(g)
				}
			},
			apply: func(b *Backup) error {
				if _, err := runHidden("powercfg", "/setacvalueindex", "SCHEME_CURRENT", usbSub, usbSuspend, "0"); err != nil {
					return errors.New("powercfg failed")
				}
				runHidden("powercfg", "/setdcvalueindex", "SCHEME_CURRENT", usbSub, usbSuspend, "0")
				_, err := runHidden("powercfg", "/setactive", "SCHEME_CURRENT")
				return err
			},
			restore: func(b *Backup) error {
				s := b.Strs["scheme"]
				if s == "" {
					s = "SCHEME_CURRENT"
				}
				if _, err := runHidden("powercfg", "/setacvalueindex", s, usbSub, usbSuspend, fmt.Sprint(b.Nums["ac"])); err != nil {
					return errors.New("powercfg failed")
				}
				runHidden("powercfg", "/setdcvalueindex", s, usbSub, usbSuspend, fmt.Sprint(b.Nums["dc"]))
				runHidden("powercfg", "/setactive", "SCHEME_CURRENT")
				return nil
			},
			check: func() bool { ac, _, ok := usbValues("SCHEME_CURRENT"); return ok && ac == 0 },
			state: func() []KV {
				ac, dc, _ := usbValues("SCHEME_CURRENT")
				return []KV{{"USB selective suspend AC / DC (powercfg)", fmt.Sprintf("%d / %d", ac, dc)}}
			},
		},
		{
			ID: "cpu_fg", Category: CatWindows, Icon: "cpu", Impact: "subtle",
			Name:          "Foreground CPU Priority",
			Description:   "Win32PrioritySeparation = 0x26: the active window (your game) gets shorter, boosted CPU time slices.",
			NameDE:        "CPU-Priorität für Vordergrund",
			DescriptionDE: "Win32PrioritySeparation = 0x26: Das aktive Fenster (dein Spiel) bekommt kürzere, bevorzugte CPU-Zeitscheiben.",
			regs:          []rv{dw("HKLM", `SYSTEM\CurrentControlSet\Control\PriorityControl`, "Win32PrioritySeparation", 0x26)},
		},
		{
			ID: "sys_resp", Category: CatWindows, Icon: "rocket", Impact: "subtle",
			Name:          "Max System Responsiveness",
			Description:   "SystemResponsiveness = 10 (minimum): background multimedia tasks reserve less CPU time.",
			NameDE:        "System-Reaktionsfähigkeit MAX",
			DescriptionDE: "SystemResponsiveness = 10 (Minimum): Hintergrund-Multimedia-Tasks reservieren weniger CPU-Zeit.",
			regs:          []rv{dw("HKLM", mmcss, "SystemResponsiveness", 10)},
		},
		{
			ID: "timer_global", Category: CatWindows, Icon: "clock", Impact: "subtle", RequiresRestart: true,
			Name:          "Allow Global Timer Resolution",
			Description:   "Required on Windows 11 so the 0.5 ms timer (System page) applies system-wide, not only to MEXXIC.",
			NameDE:        "Globale Timer-Auflösung erlauben",
			DescriptionDE: "Nötig unter Windows 11, damit der 0,5-ms-Timer (Seite System) systemweit gilt und nicht nur für MEXXIC.",
			regs:          []rv{dw("HKLM", `SYSTEM\CurrentControlSet\Control\Session Manager\kernel`, "GlobalTimerResolutionRequests", 1)},
		},
	}
}

package main

// Tweaks › Input: keyboard & mouse.

import "fmt"

func inputTweaks() []*Tweak {
	return []*Tweak{
		{
			ID: "kb_popups", Category: CatInput, Icon: "shift", Impact: "noticeable",
			Name:          "Disable Shift Pop-ups",
			Description:   "Turns off the Sticky Keys (5× Shift), Filter Keys (hold Shift 8 s) and Toggle Keys hotkeys. No Windows dialog interrupts you while sprinting.",
			NameDE:        "Shift-Popups aus",
			DescriptionDE: "Schaltet die Hotkeys für Einrastfunktion (5× Shift), Anschlagverzögerung (8 Sek. Shift halten) und Statustasten ab. Kein Windows-Fenster unterbricht dich mehr beim Sprinten.",
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
			state: func() []KV {
				return []KV{
					{"StickyKeys flags (SPI)", fmt.Sprintf("0x%X", getSticky())},
					{"FilterKeys flags (SPI)", fmt.Sprintf("0x%X", getFilter().Flags)},
					{"ToggleKeys flags (SPI)", fmt.Sprintf("0x%X", getToggle())},
				}
			},
		},
		{
			ID: "kb_repeat", Category: CatInput, Icon: "keyboard", Impact: "comfort",
			Name:          "Max Key Repeat",
			Description:   "Shortest repeat delay and fastest repeat rate. Affects typing (chat, F8 console), not held WASD movement.",
			NameDE:        "Tastenwiederholung MAX",
			DescriptionDE: "Kürzeste Verzögerung und schnellste Wiederholrate. Wirkt beim Tippen (Chat, F8-Konsole), nicht auf gehaltene WASD-Bewegung.",
			snap:          func(b *Backup) { b.Nums["delay"], b.Nums["speed"] = int64(getKbDelay()), int64(getKbSpeed()) },
			apply:         func(b *Backup) error { setKbDelay(0); setKbSpeed(31); return nil },
			restore: func(b *Backup) error {
				setKbDelay(uint32(b.Nums["delay"]))
				setKbSpeed(uint32(b.Nums["speed"]))
				return nil
			},
			check: func() bool { return getKbDelay() == 0 && getKbSpeed() == 31 },
			state: func() []KV {
				return []KV{
					{"Keyboard repeat delay (SPI, 0–3)", fmt.Sprint(getKbDelay())},
					{"Keyboard repeat rate (SPI, 0–31)", fmt.Sprint(getKbSpeed())},
				}
			},
		},
		{
			ID: "kb_lang", Category: CatInput, Icon: "globe", Impact: "noticeable", RequiresRestart: true,
			Name:          "Disable Alt+Shift Layout Switch",
			Description:   "Stops Windows from switching the keyboard layout (e.g. to QWERTY) when you press Alt/Ctrl+Shift mid-game.",
			NameDE:        "Alt+Shift-Layoutwechsel aus",
			DescriptionDE: "Verhindert, dass Windows bei Alt/Strg+Shift mitten im Spiel das Tastaturlayout wechselt (z. B. auf QWERTY).",
			regs: []rv{
				sz("HKCU", `Keyboard Layout\Toggle`, "Hotkey", "3"),
				sz("HKCU", `Keyboard Layout\Toggle`, "Language Hotkey", "3"),
				sz("HKCU", `Keyboard Layout\Toggle`, "Layout Hotkey", "3"),
			},
		},
		{
			ID: "kb_snip", Category: CatInput, Icon: "camera", Impact: "comfort",
			Name:          "Print Key without Snipping Tool",
			Description:   "The Print key no longer opens the Snipping Tool overlay on top of your game.",
			NameDE:        "Druck-Taste ohne Snipping Tool",
			DescriptionDE: "Die Druck-Taste öffnet nicht mehr das Snipping Tool über dem Spiel.",
			regs:          []rv{dw("HKCU", `Control Panel\Keyboard`, "PrintScreenKeyForSnippingEnabled", 0)},
		},
		{
			ID: "mouse_accel", Category: CatInput, Icon: "mouse", Impact: "noticeable",
			Name:          "Disable Mouse Acceleration",
			Description:   "Turns off “Enhance pointer precision”. Mouse movement maps 1:1 to your aim. The most noticeable tweak.",
			NameDE:        "Mausbeschleunigung aus",
			DescriptionDE: "Deaktiviert „Zeigerbeschleunigung verbessern“. Mausweg = Zielweg, 1:1. Der am deutlichsten spürbare Tweak.",
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
			state: func() []KV {
				m := getMouse()
				return []KV{{"Mouse threshold1, threshold2, acceleration (SPI)", fmt.Sprintf("%d, %d, %d", m[0], m[1], m[2])}}
			},
		},
	}
}

package main

// Tweaks › Network.

func networkTweaks() []*Tweak {
	return []*Tweak{
		{
			ID: "net_throttle", Category: CatNetwork, Icon: "wifi", Impact: "subtle", RequiresRestart: true,
			Name:          "Disable Network Throttling",
			Description:   "Removes the Windows multimedia network throttling (NetworkThrottlingIndex). Helps with packet bursts. It does not change your ping.",
			NameDE:        "Netzwerk-Drosselung aus",
			DescriptionDE: "Schaltet die Windows-Paketdrosselung bei Multimedia-Last ab (NetworkThrottlingIndex). Hilft bei Paket-Spitzen, deinen Ping ändert sie nicht.",
			regs:          []rv{dw("HKLM", mmcss, "NetworkThrottlingIndex", 0xFFFFFFFF)},
		},
	}
}

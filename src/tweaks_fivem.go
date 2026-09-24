package main

// Tweaks › FiveM.

import "fmt"

const ifeo = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Image File Execution Options\`

var fivemBuilds = []string{"", "b2060_", "b2189_", "b2372_", "b2545_", "b2612_", "b2699_", "b2802_", "b2944_", "b3095_", "b3258_", "b3323_", "b3407_", "b3570_"}

func fivemTweaks() []*Tweak {
	var regs []rv
	for _, b := range fivemBuilds {
		regs = append(regs, dw("HKLM", ifeo+"FiveM_"+b+"GTAProcess.exe\\PerfOptions", "CpuPriorityClass", 3))
	}
	return []*Tweak{
		{
			ID: "fivem_prio", Category: CatFiveM, Icon: "car", Impact: "noticeable",
			Name:          "FiveM High Priority",
			Description:   "FiveM (all game builds) always starts with CPU priority “High”. Fewer stutters when Discord, a browser etc. run in the background.",
			NameDE:        "FiveM immer hohe Priorität",
			DescriptionDE: "FiveM (alle Game-Builds) startet immer mit CPU-Priorität „Hoch“. Weniger Ruckler, wenn Discord, Browser usw. nebenbei laufen.",
			regs:          regs,
			// 14 identical values: summarise instead of listing each key
			stateOv: func() []KV {
				set := 0
				for _, r := range regs {
					if regMatches(r.Root, r.Path, r.Name, false, "", 3) {
						set++
					}
				}
				return []KV{{`HKLM\…\Image File Execution Options\FiveM_*GTAProcess.exe\PerfOptions → CpuPriorityClass = 3`, fmt.Sprintf("%d / %d builds", set, len(regs))}}
			},
		},
	}
}

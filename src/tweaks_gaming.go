package main

// Tweaks › Gaming: scheduler priority, Game DVR, Game Mode.

const mmcssGames = mmcss + `\Tasks\Games`

func gamingTweaks() []*Tweak {
	return []*Tweak{
		{
			ID: "game_prio", Category: CatGaming, Icon: "gamepad", Impact: "subtle",
			Name:          "Game Scheduling Priority",
			Description:   "Multimedia scheduler: games get GPU priority 8, CPU priority 6 and scheduling category “High”.",
			NameDE:        "Spiele-Priorität ULTRA",
			DescriptionDE: "Multimedia-Scheduler: Spiele bekommen GPU-Priorität 8, CPU-Priorität 6 und Kategorie „High“.",
			regs: []rv{
				dw("HKLM", mmcssGames, "GPU Priority", 8),
				dw("HKLM", mmcssGames, "Priority", 6),
				sz("HKLM", mmcssGames, "Scheduling Category", "High"),
				sz("HKLM", mmcssGames, "SFIO Priority", "High"),
			},
		},
		{
			ID: "game_dvr", Category: CatGaming, Icon: "record", Impact: "noticeable",
			Name:          "Disable Xbox Game DVR",
			Description:   "Stops background recording by the Xbox Game Bar. Fewer FPS drops and frametime spikes.",
			NameDE:        "Xbox Game DVR aus",
			DescriptionDE: "Stoppt die Hintergrund-Aufnahme der Xbox Game Bar. Weniger FPS-Einbrüche und Frametime-Spikes.",
			regs: []rv{
				dw("HKCU", `System\GameConfigStore`, "GameDVR_Enabled", 0),
				dw("HKCU", `Software\Microsoft\Windows\CurrentVersion\GameDVR`, "AppCaptureEnabled", 0),
				dw("HKLM", `SOFTWARE\Policies\Microsoft\Windows\GameDVR`, "AllowGameDVR", 0),
			},
		},
		{
			ID: "game_mode", Category: CatGaming, Icon: "star", Impact: "subtle",
			Name:          "Enable Windows Game Mode",
			Description:   "Windows prioritizes the running game and holds back driver updates and notifications while you play.",
			NameDE:        "Windows-Spielmodus an",
			DescriptionDE: "Windows priorisiert das laufende Spiel und hält Treiber-Updates und Benachrichtigungen zurück, während du spielst.",
			regs: []rv{
				dw("HKCU", `Software\Microsoft\GameBar`, "AutoGameModeEnabled", 1),
				dw("HKCU", `Software\Microsoft\GameBar`, "AllowAutoGameMode", 1),
			},
		},
	}
}

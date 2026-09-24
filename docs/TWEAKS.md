# Alle Tweaks im Detail

Kategorien in der App: **Input · Network · Windows · Gaming · FiveM**. Nach jedem Anwenden prüft MEXXIC den echten Wert. Die Restore-Seite zeigt pro Sicherung *Original → Neu*.

Hier steht jede Änderung, die MEXXIC Tweaks vornimmt. Vor dem ersten Anwenden wird der **aktuelle** Wert
gesichert (`%APPDATA%\MexxicTweaks\backup.json`). *Rückgängig* schreibt genau diesen Wert zurück.
Falls es den Wert vorher nicht gab, wird er wieder gelöscht.

> **HKCU** = `HKEY_CURRENT_USER` · **HKLM** = `HKEY_LOCAL_MACHINE`
> **SPI** = Windows-API `SystemParametersInfo` (wirkt sofort, Windows speichert selbst in der Registry)

---

## Input (Tastatur & Maus)

### Shift-Popups AUS · `kb_popups`
| Einstellung | Änderung |
|---|---|
| Einrastfunktion (StickyKeys) | SPI: Flags `ON` + `HOTKEYACTIVE` entfernt |
| Anschlagverzögerung (FilterKeys) | SPI: Flags `ON` + `HOTKEYACTIVE` entfernt |
| Statustasten (ToggleKeys) | SPI: Flags `ON` + `HOTKEYACTIVE` entfernt |

### Tastenwiederholung MAX · `kb_repeat`
| Einstellung | Neu | Windows-Standard |
|---|---|---|
| Wiederholverzögerung (SPI) | `0` (kürzeste) | `1` |
| Wiederholrate (SPI) | `31` (schnellste) | `31` |

### Alt+Shift Layoutwechsel AUS · `kb_lang` · *Neustart/Abmelden*
| Pfad | Wert | Neu |
|---|---|---|
| `HKCU\Keyboard Layout\Toggle` | `Hotkey` (SZ) | `3` |
| `HKCU\Keyboard Layout\Toggle` | `Language Hotkey` (SZ) | `3` |
| `HKCU\Keyboard Layout\Toggle` | `Layout Hotkey` (SZ) | `3` |

### Druck-Taste ohne Snipping Tool · `kb_snip`
| Pfad | Wert | Neu |
|---|---|---|
| `HKCU\Control Panel\Keyboard` | `PrintScreenKeyForSnippingEnabled` (DWORD) | `0` |

---


### Mausbeschleunigung AUS · `mouse_accel`
| Einstellung | Neu | Windows-Standard |
|---|---|---|
| SPI_SETMOUSE (Threshold1, Threshold2, Acceleration) | `0, 0, 0` | `6, 10, 1` |

Entspricht: *Mauseigenschaften → Zeigeroptionen → „Zeigerbeschleunigung verbessern“ aus*.

---

## Windows

### Ultimative Leistung · `power_plan`
- `powercfg -duplicatescheme e9a42b02-d5df-448d-aa00-03f14749eb61` → neuer Plan **„MEXXIC Ultimate“**, wird aktiviert
- Falls nicht unterstützt: Fallback auf *Höchstleistung* (`8c5e7fda-…`)
- Rückgängig: vorheriger Plan wird aktiviert, der erstellte Plan gelöscht

### USB-Energiesparen AUS · `usb_suspend`
- `powercfg /setacvalueindex SCHEME_CURRENT 2a737441-… 48e6b7a6-… 0` (+ DC)
- Rückgängig: Originalwerte (AC/DC) auf den damals aktiven Plan zurück

### CPU-Priorität für Vordergrund · `cpu_fg`
| Pfad | Wert | Neu | Standard |
|---|---|---|---|
| `HKLM\SYSTEM\CurrentControlSet\Control\PriorityControl` | `Win32PrioritySeparation` | `0x26` (38) | `0x2` |

### System-Reaktionsfähigkeit MAX · `sys_resp`
| Pfad | Wert | Neu | Standard |
|---|---|---|---|
| `HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Multimedia\SystemProfile` | `SystemResponsiveness` | `10` | `20` |

### Spiele-Priorität ULTRA (Kategorie Gaming) · `game_prio`
Pfad: `HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Multimedia\SystemProfile\Tasks\Games`

| Wert | Neu | Standard |
|---|---|---|
| `GPU Priority` (DWORD) | `8` | `8` |
| `Priority` (DWORD) | `6` | `2` |
| `Scheduling Category` (SZ) | `High` | `Medium` |
| `SFIO Priority` (SZ) | `High` | `Normal` |

### Netzwerk-Drosselung AUS (Kategorie Network) · `net_throttle` · *Neustart*
| Pfad | Wert | Neu | Standard |
|---|---|---|---|
| `HKLM\…\Multimedia\SystemProfile` | `NetworkThrottlingIndex` | `0xFFFFFFFF` | `10` |

### Xbox Game DVR AUS (Kategorie Gaming) · `game_dvr`
| Pfad | Wert | Neu |
|---|---|---|
| `HKCU\System\GameConfigStore` | `GameDVR_Enabled` | `0` |
| `HKCU\Software\Microsoft\Windows\CurrentVersion\GameDVR` | `AppCaptureEnabled` | `0` |
| `HKLM\SOFTWARE\Policies\Microsoft\Windows\GameDVR` | `AllowGameDVR` | `0` |

### Windows-Spielmodus AN (Kategorie Gaming) · `game_mode`
| Pfad | Wert | Neu |
|---|---|---|
| `HKCU\Software\Microsoft\GameBar` | `AutoGameModeEnabled` | `1` |
| `HKCU\Software\Microsoft\GameBar` | `AllowAutoGameMode` | `1` |

### Globale Timer-Auflösung erlauben · `timer_global` · *Neustart*
| Pfad | Wert | Neu |
|---|---|---|
| `HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\kernel` | `GlobalTimerResolutionRequests` | `1` |

Nötig ab Windows 11, damit der Schalter **Timer 0,5 ms** (Seite System) systemweit gilt.

---

## FiveM

### FiveM immer hohe Priorität · `fivem_prio`
Pfad: `HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Image File Execution Options\<EXE>\PerfOptions`
Wert: `CpuPriorityClass` = `3` (Hoch)

Gilt für `FiveM_GTAProcess.exe` sowie die Game-Builds
`b2060, b2189, b2372, b2545, b2612, b2699, b2802, b2944, b3095, b3258, b3323, b3407, b3570`
(z. B. `FiveM_b3095_GTAProcess.exe`).

---

## Live-Funktionen (keine dauerhafte Änderung)

| Funktion | Was passiert |
|---|---|
| **Timer 0,5 ms** | `NtSetTimerResolution(5000)`, gilt nur, solange die App offen ist |
| **FiveM Booster** | Setzt laufende FiveM-Prozesse sofort auf `HIGH_PRIORITY_CLASS` |
| **WASD Live-Test** | Liest nur `GetAsyncKeyState`, speichert oder sendet nichts |

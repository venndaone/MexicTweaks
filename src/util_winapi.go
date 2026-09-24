package main

// Utilities: thin wrappers around the Windows API used by tweaks and live tools.

import (
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                     = windows.NewLazySystemDLL("user32.dll")
	procSPI                    = user32.NewProc("SystemParametersInfoW")
	procGetAsyncKeyState       = user32.NewProc("GetAsyncKeyState")
	procMessageBoxW            = user32.NewProc("MessageBoxW")
	ntdll                      = windows.NewLazySystemDLL("ntdll.dll")
	procNtSetTimerResolution   = ntdll.NewProc("NtSetTimerResolution")
	procNtQueryTimerResolution = ntdll.NewProc("NtQueryTimerResolution")
)

const spifSave = 0x01 | 0x02 // SPIF_UPDATEINIFILE | SPIF_SENDCHANGE

func spi(action, uiParam uint32, pv unsafe.Pointer, winIni uint32) bool {
	r, _, _ := procSPI.Call(uintptr(action), uintptr(uiParam), uintptr(pv), uintptr(winIni))
	return r != 0
}

// ---------- Accessibility (Sticky / Toggle / Filter keys) ----------

type skStruct struct{ Cb, Flags uint32 }
type fkStruct struct{ Cb, Flags, Wait, Delay, Repeat, Bounce uint32 }

func getSticky() uint32  { s := skStruct{Cb: 8}; spi(0x3A, 8, unsafe.Pointer(&s), 0); return s.Flags }
func setSticky(f uint32) { s := skStruct{Cb: 8, Flags: f}; spi(0x3B, 8, unsafe.Pointer(&s), spifSave) }
func getToggle() uint32  { s := skStruct{Cb: 8}; spi(0x34, 8, unsafe.Pointer(&s), 0); return s.Flags }
func setToggle(f uint32) { s := skStruct{Cb: 8, Flags: f}; spi(0x35, 8, unsafe.Pointer(&s), spifSave) }
func getFilter() fkStruct {
	f := fkStruct{Cb: 24}
	spi(0x32, 24, unsafe.Pointer(&f), 0)
	return f
}
func setFilter(f fkStruct) { f.Cb = 24; spi(0x33, 24, unsafe.Pointer(&f), spifSave) }

// ---------- Keyboard repeat ----------

func getKbDelay() uint32  { var v uint32; spi(0x16, 0, unsafe.Pointer(&v), 0); return v }
func setKbDelay(v uint32) { spi(0x17, v, nil, spifSave) }
func getKbSpeed() uint32  { var v uint32; spi(0x0A, 0, unsafe.Pointer(&v), 0); return v }
func setKbSpeed(v uint32) { spi(0x0B, v, nil, spifSave) }

// ---------- Mouse acceleration ----------

func getMouse() [3]int32  { var m [3]int32; spi(0x03, 0, unsafe.Pointer(&m[0]), 0); return m }
func setMouse(m [3]int32) { spi(0x04, 0, unsafe.Pointer(&m[0]), spifSave) }

// ---------- Keys ----------

func keyDown(vk int) bool {
	r, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
	return r&0x8000 != 0
}

// ---------- Timer resolution (units of 100ns) ----------

func queryTimer() (coarsest, finest, current uint32) {
	procNtQueryTimerResolution.Call(uintptr(unsafe.Pointer(&coarsest)), uintptr(unsafe.Pointer(&finest)), uintptr(unsafe.Pointer(&current)))
	return
}

func setTimerRes(desired uint32, set bool) uint32 {
	var actual uint32
	s := uintptr(0)
	if set {
		s = 1
	}
	procNtSetTimerResolution.Call(uintptr(desired), s, uintptr(unsafe.Pointer(&actual)))
	return actual
}

// ---------- Misc ----------

func messageBox(title, text string) {
	t, _ := windows.UTF16PtrFromString(title)
	m, _ := windows.UTF16PtrFromString(text)
	procMessageBoxW.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), 0x40)
}

func isAdmin() bool { return windows.GetCurrentProcessToken().IsElevated() }

func runHidden(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

var guidRe = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
var hexRe = regexp.MustCompile(`0x[0-9a-fA-F]{8}`)

func activeScheme() (guid string, raw string) {
	out, _ := runHidden("powercfg", "/getactivescheme")
	return strings.ToLower(guidRe.FindString(out)), out
}

type procInfo struct {
	Pid  uint32
	Name string
}

func listProcs() []procInfo {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snap)
	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	var res []procInfo
	for err = windows.Process32First(snap, &pe); err == nil; err = windows.Process32Next(snap, &pe) {
		res = append(res, procInfo{pe.ProcessID, windows.UTF16ToString(pe.ExeFile[:])})
	}
	return res
}

func setProcHigh(pid uint32) error {
	h, err := windows.OpenProcess(windows.PROCESS_SET_INFORMATION|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	return windows.SetPriorityClass(h, windows.HIGH_PRIORITY_CLASS)
}

func procPriority(pid uint32) uint32 {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(h)
	p, _ := windows.GetPriorityClass(h)
	return p
}

// ---------- System info helpers ----------

var (
	kernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
	procGetTickCount64       = kernel32.NewProc("GetTickCount64")
)

type memStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

func memoryStatus() (total, avail uint64, load uint32) {
	m := memStatusEx{Length: uint32(unsafe.Sizeof(memStatusEx{}))}
	procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&m)))
	return m.TotalPhys, m.AvailPhys, m.MemoryLoad
}

func uptimeMs() uint64 {
	r, _, _ := procGetTickCount64.Call()
	return uint64(r)
}

// schemeName extracts the friendly name from `powercfg /getactivescheme` output.
func schemeName(raw string) string {
	a, b := strings.Index(raw, "("), strings.LastIndex(raw, ")")
	if a >= 0 && b > a {
		return strings.TrimSpace(raw[a+1 : b])
	}
	return ""
}

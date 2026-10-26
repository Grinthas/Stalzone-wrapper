// Package tweaks — системные настройки на время игры: приоритет процесса,
// схема питания и выбор видеокарты. Всё обратимо и не требует прав администратора.
package tweaks

import (
	"regexp"
	"strings"
	"syscall"

	"github.com/grinthas/stalzone-wrapper/internal/sys"
)

const (
	aboveNormalPriority = 0x00008000
	processSetInfo      = 0x0200

	// GUID встроенной схемы «Высокая производительность».
	HighPerformanceScheme = "8c5e7fda-e8bf-4a96-9a85-a6e23a8c635c"

	gpuKey = `HKCU\Software\Microsoft\DirectX\UserGpuPreferences`
)

var procSetPriorityClass = syscall.NewLazyDLL("kernel32.dll").NewProc("SetPriorityClass")

// RaisePriority повышает приоритет процесса до «Выше среднего».
// «Высокий» не используем: он может отнимать время у звука и ввода.
func RaisePriority(pid int) error {
	h, err := syscall.OpenProcess(processSetInfo, false, uint32(pid))
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(h)
	if r, _, err := procSetPriorityClass.Call(uintptr(h), aboveNormalPriority); r == 0 {
		return err
	}
	return nil
}

var guidRe = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

// ActivePowerScheme возвращает GUID текущей схемы питания.
func ActivePowerScheme() string {
	out, err := sys.Command("powercfg.exe", "/getactivescheme").Output()
	if err != nil {
		return ""
	}
	return strings.ToLower(guidRe.FindString(string(out)))
}

func SetPowerScheme(guid string) error {
	return sys.Command("powercfg.exe", "/setactive", guid).Run()
}

// SetHighPerformanceGPU просит Windows запускать exe на мощной видеокарте
// (имеет смысл на ноутбуках со встроенной и дискретной графикой).
func SetHighPerformanceGPU(exe string, on bool) error {
	if on {
		return sys.Command("reg.exe", "add", gpuKey, "/v", exe, "/t", "REG_SZ", "/d", "GpuPreference=2;", "/f").Run()
	}
	sys.Command("reg.exe", "delete", gpuKey, "/v", exe, "/f").Run()
	return nil
}

// Package ifeo регистрирует hook через Image File Execution Options: при запуске
// exe игры Windows вместо него запускает service.exe и передаёт ему исходную команду.
//
// Hook ставится по имени exe игры, как в EXBO-Community/stalcraft-jvm-optimization:
// stalzone.exe запускает отдельный лаунчер, stalzonew.exe — Steam.
package ifeo

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/grinthas/stalzone-wrapper/internal/sys"
)

const root = `HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Image File Execution Options`

// Target — exe игры, на который ставится hook.
type Target struct {
	Exe   string `json:"exe"`
	Title string `json:"title"`
}

var Targets = []Target{
	{"stalzone.exe", "Лаунчер Stalzone"},
	{"stalzonew.exe", "Steam"},
}

func reg(args ...string) (string, error) {
	out, err := sys.Command("reg.exe", args...).CombinedOutput()
	return string(out), err
}

// Install направляет все exe игры на service.exe.
func Install(service string) error {
	for _, t := range Targets {
		if out, err := reg("add", root+`\`+t.Exe, "/v", "Debugger", "/t", "REG_SZ", "/d", `"`+service+`"`, "/f"); err != nil {
			Uninstall()
			return fmt.Errorf("%s: %v: %s", t.Exe, err, strings.TrimSpace(out))
		}
	}
	return nil
}

// Uninstall удаляет hook со всех exe игры. Удаляется только значение Debugger:
// другие настройки IFEO, если они есть у этих exe, остаются.
func Uninstall() error {
	for _, t := range Targets {
		if Debugger(t.Exe) == "" {
			continue
		}
		if out, err := reg("delete", root+`\`+t.Exe, "/v", "Debugger", "/f"); err != nil {
			return fmt.Errorf("%s: %v: %s", t.Exe, err, strings.TrimSpace(out))
		}
	}
	return nil
}

// Debugger возвращает путь к service.exe из hook для exe ("" — hook не установлен).
func Debugger(exe string) string {
	return queryDebugger(root + `\` + exe)
}

func queryDebugger(key string) string {
	out, err := reg("query", key, "/v", "Debugger")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(out, "\n") {
		if _, value, ok := strings.Cut(line, "REG_SZ"); ok && strings.Contains(line, "Debugger") {
			return strings.Trim(strings.TrimSpace(value), `"`)
		}
	}
	return ""
}

// RemoveLegacy удаляет hook старых версий (≤1.3.3), которые ставились на
// java.exe/javaw.exe игры с фильтром по полному пути.
func RemoveLegacy(paths []string) {
	for _, p := range paths {
		exeKey := root + `\` + strings.ToLower(filepath.Base(p))
		var h uint32 = 2166136261
		for _, c := range strings.ToLower(p) {
			h = (h ^ uint32(c)) * 16777619
		}
		reg("delete", fmt.Sprintf(`%s\StalzoneWrapper_%08x`, exeKey, h), "/f")
		out, _ := reg("query", exeKey)
		if !strings.Contains(strings.ToLower(out), strings.ToLower(exeKey)+`\`) {
			reg("delete", exeKey, "/v", "UseFilter", "/f")
		}
	}
}

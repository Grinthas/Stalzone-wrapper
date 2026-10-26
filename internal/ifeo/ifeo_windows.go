// Package ifeo регистрирует hook через Image File Execution Options.
//
// Используется режим фильтра (UseFilter + FilterFullPath, Windows 10 1607+),
// поэтому перехватывается только java.exe из папки игры, а не любая Java.
package ifeo

import (
	"fmt"
	"github.com/grinthas/stalzone-wrapper/internal/sys"
	"path/filepath"
	"strings"
)

const (
	root   = `HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Image File Execution Options`
	subkey = "StalzoneWrapper"
)

func reg(args ...string) (string, error) {
	out, err := sys.Command("reg.exe", args...).CombinedOutput()
	return string(out), err
}

// keyFor возвращает ключ фильтра; имя подключа включает хеш пути, чтобы
// несколько целей с одинаковым именем exe не конфликтовали.
func keyFor(target string) (exeKey, filterKey string) {
	exeKey = root + `\` + strings.ToLower(filepath.Base(target))
	var h uint32 = 2166136261
	for _, c := range strings.ToLower(target) {
		h = (h ^ uint32(c)) * 16777619
	}
	return exeKey, fmt.Sprintf(`%s\%s_%08x`, exeKey, subkey, h)
}

// Install ставит hook на target, при запуске которого Windows вызовет service.
func Install(target, service string) error {
	exeKey, filterKey := keyFor(target)
	steps := [][]string{
		{"add", exeKey, "/v", "UseFilter", "/t", "REG_DWORD", "/d", "1", "/f"},
		{"add", filterKey, "/v", "FilterFullPath", "/t", "REG_SZ", "/d", target, "/f"},
		{"add", filterKey, "/v", "Debugger", "/t", "REG_SZ", "/d", `"` + service + `"`, "/f"},
	}
	for _, s := range steps {
		if out, err := reg(s...); err != nil {
			return fmt.Errorf("%v: %s", err, strings.TrimSpace(out))
		}
	}
	return nil
}

// Uninstall удаляет hook; UseFilter снимается, если других фильтров не осталось.
func Uninstall(target string) error {
	exeKey, filterKey := keyFor(target)
	if out, err := reg("delete", filterKey, "/f"); err != nil && Installed(target) {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(out))
	}
	out, _ := reg("query", exeKey)
	if !strings.Contains(strings.ToLower(out), strings.ToLower(exeKey)+`\`) {
		reg("delete", exeKey, "/v", "UseFilter", "/f")
	}
	return nil
}

func Installed(target string) bool {
	_, filterKey := keyFor(target)
	_, err := reg("query", filterKey, "/v", "Debugger")
	return err == nil
}

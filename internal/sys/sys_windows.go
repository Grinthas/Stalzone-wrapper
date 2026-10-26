// Package sys — системные вызовы Windows без внешних зависимостей.
package sys

import (
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
	procDebugActiveStop      = kernel32.NewProc("DebugActiveProcessStop")
	procDebugSetKillOnExit   = kernel32.NewProc("DebugSetProcessKillOnExit")
)

const debugOnlyThisProcess = 0x00000002

// TotalRAMGB возвращает объём физической памяти в гигабайтах.
func TotalRAMGB() int {
	var m struct {
		Length, MemoryLoad                                 uint32
		TotalPhys, AvailPhys, TotalPageFile, AvailPageFile uint64
		TotalVirtual, AvailVirtual, AvailExtendedVirtual   uint64
	}
	m.Length = uint32(unsafe.Sizeof(m))
	if r, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&m))); r == 0 {
		return 8
	}
	return int((m.TotalPhys + (1 << 29)) >> 30)
}

// StartBypassingIFEO запускает процесс так, чтобы Windows не вызвала hook снова:
// процесс, создаваемый под отладкой, не проходит IFEO, после чего отладчик сразу отцепляется.
func StartBypassingIFEO(exe string, args []string) (*os.Process, error) {
	cmd := exec.Command(exe, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: debugOnlyThisProcess}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	procDebugSetKillOnExit.Call(0)
	procDebugActiveStop.Call(uintptr(cmd.Process.Pid))
	return cmd.Process, nil
}

// IsAdmin проверяет права администратора.
func IsAdmin() bool {
	return exec.Command("net.exe", "session").Run() == nil
}

// Elevate перезапускает текущую программу с правами администратора.
func Elevate(args ...string) error {
	exe, _ := os.Executable()
	ps := "Start-Process -Verb RunAs -FilePath '" + exe + "'"
	if len(args) > 0 {
		ps += " -ArgumentList '" + joinQuoted(args) + "'"
	}
	return exec.Command("powershell.exe", "-NoProfile", "-Command", ps).Run()
}

func joinQuoted(a []string) string {
	s := ""
	for i, x := range a {
		if i > 0 {
			s += " "
		}
		s += `"` + x + `"`
	}
	return s
}

// Package sys — системные вызовы Windows без внешних зависимостей.
package sys

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
	procDebugActiveStop      = kernel32.NewProc("DebugActiveProcessStop")
	procDebugSetKillOnExit   = kernel32.NewProc("DebugSetProcessKillOnExit")

	shell32           = syscall.NewLazyDLL("shell32.dll")
	procIsUserAnAdmin = shell32.NewProc("IsUserAnAdmin")
	procShellExecuteW = shell32.NewProc("ShellExecuteW")

	comdlg32             = syscall.NewLazyDLL("comdlg32.dll")
	procGetOpenFileNameW = comdlg32.NewProc("GetOpenFileNameW")
)

const (
	debugOnlyThisProcess = 0x00000002
	createNoWindow       = 0x08000000
)

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

// Command создаёт команду без всплывающего окна консоли.
func Command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	return cmd
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
	r, _, _ := procIsUserAnAdmin.Call()
	return r != 0
}

// Elevate перезапускает текущую программу с правами администратора (окно UAC).
func Elevate() error {
	exe, _ := os.Executable()
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(exe)
	r, _, err := procShellExecuteW.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(file)), 0, 0, 1)
	if r <= 32 {
		return err
	}
	return nil
}

// OpenFileDialog показывает стандартный диалог выбора .exe; "" — если отменён.
func OpenFileDialog(owner uintptr, title string) string {
	buf := make([]uint16, 1024)
	filter := utf16.Encode([]rune("Java (java.exe, javaw.exe)\x00java.exe;javaw.exe\x00Программы (*.exe)\x00*.exe\x00\x00"))
	t, _ := syscall.UTF16PtrFromString(title)
	ofn := struct {
		StructSize                uint32
		Owner, Instance           uintptr
		Filter, CustomFilter      *uint16
		MaxCustFilter, FilterIdx  uint32
		File                      *uint16
		MaxFile                   uint32
		FileTitle                 *uint16
		MaxFileTitle              uint32
		InitialDir, Title         *uint16
		Flags                     uint32
		FileOffset, FileExtension uint16
		DefExt                    *uint16
		CustData, Hook            uintptr
		TemplateName              *uint16
		Reserved                  uintptr
		Reserved2, FlagsEx        uint32
	}{Owner: owner, Filter: &filter[0], FilterIdx: 1, File: &buf[0], MaxFile: uint32(len(buf)), Title: t,
		Flags: 0x00001000 | 0x00000800 | 0x00080000} // FILEMUSTEXIST | PATHMUSTEXIST | EXPLORER
	ofn.StructSize = uint32(unsafe.Sizeof(ofn))
	if r, _, _ := procGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn))); r == 0 {
		return ""
	}
	return strings.TrimRight(syscall.UTF16ToString(buf), "\x00")
}

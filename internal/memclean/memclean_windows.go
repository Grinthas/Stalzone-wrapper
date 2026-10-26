// Package memclean освобождает оперативную память, как Mem Reduct:
// урезает рабочие наборы процессов, очищает список ожидания (standby),
// сбрасывает изменённые страницы и системный файловый кэш.
//
// Всё, кроме урезания рабочих наборов, требует прав администратора.
package memclean

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")
	ntdll    = syscall.NewLazyDLL("ntdll.dll")
	psapi    = syscall.NewLazyDLL("psapi.dll")

	procGlobalMemoryStatusEx      = kernel32.NewProc("GlobalMemoryStatusEx")
	procCreateToolhelp32Snapshot  = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW           = kernel32.NewProc("Process32FirstW")
	procProcess32NextW            = kernel32.NewProc("Process32NextW")
	procQueryFullProcessImageName = kernel32.NewProc("QueryFullProcessImageNameW")
	procSetSystemFileCacheSize    = kernel32.NewProc("SetSystemFileCacheSize")
	procGetCurrentProcess         = kernel32.NewProc("GetCurrentProcess")
	procOpenProcessToken          = advapi32.NewProc("OpenProcessToken")
	procLookupPrivilegeValueW     = advapi32.NewProc("LookupPrivilegeValueW")
	procAdjustTokenPrivileges     = advapi32.NewProc("AdjustTokenPrivileges")
	procNtSetSystemInformation    = ntdll.NewProc("NtSetSystemInformation")
	procEmptyWorkingSet           = psapi.NewProc("EmptyWorkingSet")
)

const (
	systemMemoryListInformation = 80
	memoryFlushModifiedList     = 3
	memoryPurgeStandbyList      = 4

	processQueryLimitedInformation = 0x1000
	processSetQuota                = 0x0100
	th32csSnapProcess              = 0x2
)

// Options — что именно очищать.
type Options struct {
	WorkingSets  bool `json:"working_sets"`
	StandbyList  bool `json:"standby_list"`
	ModifiedList bool `json:"modified_list"`
	FileCache    bool `json:"file_cache"`
}

func DefaultOptions() Options {
	return Options{WorkingSets: true, StandbyList: true, ModifiedList: true, FileCache: true}
}

// Stats — состояние памяти в мегабайтах.
type Stats struct {
	TotalMB int `json:"totalMB"`
	FreeMB  int `json:"freeMB"`
	UsedPct int `json:"usedPct"`
}

func Current() Stats {
	var m struct {
		Length, MemoryLoad                                 uint32
		TotalPhys, AvailPhys, TotalPageFile, AvailPageFile uint64
		TotalVirtual, AvailVirtual, AvailExtendedVirtual   uint64
	}
	m.Length = uint32(unsafe.Sizeof(m))
	procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&m)))
	return Stats{TotalMB: int(m.TotalPhys >> 20), FreeMB: int(m.AvailPhys >> 20), UsedPct: int(m.MemoryLoad)}
}

// Result — итог очистки.
type Result struct {
	FreedMB int      `json:"freedMB"`
	Before  Stats    `json:"before"`
	After   Stats    `json:"after"`
	Errors  []string `json:"errors"`
}

// Clean выполняет очистку. skip — полные пути exe, чьи процессы не трогаем
// (сама игра: урезание её рабочего набора вызывает подтормаживания).
func Clean(o Options, skip []string) Result {
	r := Result{Before: Current()}
	fail := func(what string, err error) {
		if err != nil {
			r.Errors = append(r.Errors, fmt.Sprintf("%s: %v", what, err))
		}
	}
	if o.WorkingSets {
		fail("рабочие наборы", trimWorkingSets(skip))
	}
	if o.FileCache {
		fail("файловый кэш", flushFileCache())
	}
	if o.ModifiedList {
		fail("изменённые страницы", memoryListCommand(memoryFlushModifiedList))
	}
	if o.StandbyList {
		fail("список ожидания", memoryListCommand(memoryPurgeStandbyList))
	}
	r.After = Current()
	r.FreedMB = r.After.FreeMB - r.Before.FreeMB
	if r.FreedMB < 0 {
		r.FreedMB = 0
	}
	return r
}

func enablePrivilege(name string) error {
	var token syscall.Handle
	self, _, _ := procGetCurrentProcess.Call()
	if r, _, err := procOpenProcessToken.Call(self, 0x0020|0x0008, uintptr(unsafe.Pointer(&token))); r == 0 {
		return err
	}
	defer syscall.CloseHandle(token)
	var tp struct {
		Count      uint32
		Luid       [2]uint32
		Attributes uint32
	}
	n, _ := syscall.UTF16PtrFromString(name)
	if r, _, err := procLookupPrivilegeValueW.Call(0, uintptr(unsafe.Pointer(n)), uintptr(unsafe.Pointer(&tp.Luid))); r == 0 {
		return err
	}
	tp.Count, tp.Attributes = 1, 2 // SE_PRIVILEGE_ENABLED
	r, _, err := procAdjustTokenPrivileges.Call(uintptr(token), 0, uintptr(unsafe.Pointer(&tp)), 0, 0, 0)
	// AdjustTokenPrivileges возвращает успех и при ERROR_NOT_ALL_ASSIGNED (1300).
	if r == 0 || err == syscall.Errno(1300) {
		return errors.New("нужны права администратора")
	}
	return nil
}

func memoryListCommand(cmd uint32) error {
	if err := enablePrivilege("SeProfileSingleProcessPrivilege"); err != nil {
		return err
	}
	status, _, _ := procNtSetSystemInformation.Call(systemMemoryListInformation, uintptr(unsafe.Pointer(&cmd)), 4)
	if status != 0 {
		return fmt.Errorf("NTSTATUS 0x%08X", uint32(status))
	}
	return nil
}

func flushFileCache() error {
	if err := enablePrivilege("SeIncreaseQuotaPrivilege"); err != nil {
		return err
	}
	max := ^uintptr(0)
	if r, _, err := procSetSystemFileCacheSize.Call(max, max, 0); r == 0 {
		return err
	}
	return nil
}

func trimWorkingSets(skip []string) error {
	_ = enablePrivilege("SeDebugPrivilege") // позволяет трогать больше процессов; без него — только свои
	snap, _, err := procCreateToolhelp32Snapshot.Call(th32csSnapProcess, 0)
	if snap == uintptr(syscall.InvalidHandle) {
		return err
	}
	defer syscall.CloseHandle(syscall.Handle(snap))

	var e struct {
		Size            uint32
		Usage           uint32
		ProcessID       uint32
		DefaultHeapID   uintptr
		ModuleID        uint32
		Threads         uint32
		ParentProcessID uint32
		PriClassBase    int32
		Flags           uint32
		ExeFile         [260]uint16
	}
	e.Size = uint32(unsafe.Sizeof(e))
	for ok, _, _ := procProcess32FirstW.Call(snap, uintptr(unsafe.Pointer(&e))); ok != 0; ok, _, _ = procProcess32NextW.Call(snap, uintptr(unsafe.Pointer(&e))) {
		if e.ProcessID <= 4 {
			continue
		}
		h, err := syscall.OpenProcess(processQueryLimitedInformation|processSetQuota, false, e.ProcessID)
		if err != nil {
			continue // системные и защищённые процессы недоступны — это нормально
		}
		if !skipped(h, skip) {
			procEmptyWorkingSet.Call(uintptr(h))
		}
		syscall.CloseHandle(h)
	}
	return nil
}

func skipped(h syscall.Handle, skip []string) bool {
	if len(skip) == 0 {
		return false
	}
	buf := make([]uint16, 1024)
	size := uint32(len(buf))
	if r, _, _ := procQueryFullProcessImageName.Call(uintptr(h), 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size))); r == 0 {
		return false
	}
	path := syscall.UTF16ToString(buf[:size])
	for _, s := range skip {
		if strings.EqualFold(path, s) {
			return true
		}
	}
	return false
}

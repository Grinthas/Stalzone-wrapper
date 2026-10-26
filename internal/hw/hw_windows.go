// Package hw определяет характеристики компьютера.
package hw

import (
	"encoding/json"
	"runtime"
	"strings"

	"github.com/grinthas/stalzone-wrapper/internal/sys"
)

type Info struct {
	CPU     string   `json:"cpu"`
	Cores   int      `json:"cores"`
	Threads int      `json:"threads"`
	RAMGB   int      `json:"ramGB"`
	GPUs    []string `json:"gpus"`
	OS      string   `json:"os"`
}

// Один вызов PowerShell с CIM: быстрее и надёжнее устаревшего wmic.
const script = `$c = Get-CimInstance Win32_Processor | Select-Object -First 1
$g = @(Get-CimInstance Win32_VideoController | ForEach-Object { $_.Name })
$o = Get-CimInstance Win32_OperatingSystem
[pscustomobject]@{ cpu = $c.Name.Trim(); cores = [int]$c.NumberOfCores; threads = [int]$c.NumberOfLogicalProcessors; gpus = $g; os = $o.Caption + ' ' + $o.BuildNumber } | ConvertTo-Json -Compress`

func Detect() Info {
	info := Info{Threads: runtime.NumCPU(), RAMGB: sys.TotalRAMGB()}
	out, err := sys.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err == nil {
		var raw struct {
			CPU     string          `json:"cpu"`
			Cores   int             `json:"cores"`
			Threads int             `json:"threads"`
			GPUs    json.RawMessage `json:"gpus"`
			OS      string          `json:"os"`
		}
		if json.Unmarshal(out, &raw) == nil {
			info.CPU, info.OS = raw.CPU, raw.OS
			if raw.Cores > 0 {
				info.Cores = raw.Cores
			}
			if raw.Threads > 0 {
				info.Threads = raw.Threads
			}
			// ConvertTo-Json превращает массив из одного элемента в строку.
			if json.Unmarshal(raw.GPUs, &info.GPUs) != nil {
				var one string
				if json.Unmarshal(raw.GPUs, &one) == nil && one != "" {
					info.GPUs = []string{one}
				}
			}
		}
	}
	if info.Cores == 0 {
		info.Cores = max(1, info.Threads/2)
	}
	for i, g := range info.GPUs {
		info.GPUs[i] = strings.TrimSpace(g)
	}
	return info
}

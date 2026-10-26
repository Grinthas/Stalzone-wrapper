// service.exe вызывается Windows через IFEO вместо java.exe игры:
//
//	service.exe "C:\...\java.exe" <исходные аргументы>
//
// Он подставляет флаги из активного профиля и запускает настоящую Java.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/grinthas/stalzone-wrapper/internal/config"
	"github.com/grinthas/stalzone-wrapper/internal/hw"
	"github.com/grinthas/stalzone-wrapper/internal/jvm"
	"github.com/grinthas/stalzone-wrapper/internal/process"
	"github.com/grinthas/stalzone-wrapper/internal/sys"
	"github.com/grinthas/stalzone-wrapper/internal/tunnel"
	"github.com/grinthas/stalzone-wrapper/internal/tweaks"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("service.exe запускается Windows автоматически; используйте «Stalzone Wrapper.exe»")
		os.Exit(2)
	}
	if f, err := os.OpenFile(filepath.Join(config.Dir(), "wrapper.log"),
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644); err == nil {
		log.SetOutput(f)
		defer f.Close()
	}

	target, args := os.Args[1], os.Args[2:]
	s := config.LoadSettings()
	if s.Profile == "auto" {
		s = syncAutoProfile(s)
	}
	flags := tunnel.Flags(s.Servers)
	if p, err := config.LoadProfile(s.Profile); err != nil {
		log.Printf("[config] профиль %q не загружен (%v), флаги JVM не меняются", s.Profile, err)
	} else {
		flags = append(p.Flags(), flags...)
		log.Printf("[config] профиль %q", s.Profile)
	}
	if len(flags) > 0 {
		args = jvm.Rewrite(args, flags)
	}
	log.Printf("[launch] %s %v", target, args)

	// Настройка видеокарты привязана к пути exe игры, поэтому применяется здесь, при запуске.
	tweaks.SetHighPerformanceGPU(target, s.Tweaks.GPU)
	proc, err := process.Start(target, args)
	if err != nil {
		log.Printf("[launch] ошибка: %v", err)
		os.Exit(1)
	}
	if s.Tweaks.HighPriority {
		if err := tweaks.RaisePriority(int(proc.PID)); err != nil {
			log.Printf("[tweaks] приоритет: %v", err)
		}
	}
	restorePower := ""
	if s.Tweaks.PowerPlan {
		if prev := tweaks.ActivePowerScheme(); prev != "" && prev != tweaks.HighPerformanceScheme {
			if err := tweaks.SetPowerScheme(tweaks.HighPerformanceScheme); err != nil {
				log.Printf("[tweaks] схема питания недоступна: %v", err)
			} else {
				log.Printf("[tweaks] схема питания: высокая производительность (вернётся %s)", prev)
				restorePower = prev
			}
		}
	}
	code, err := proc.Wait()
	proc.Close()
	if err != nil {
		log.Printf("[launch] ожидание: %v", err)
	}
	if restorePower != "" {
		tweaks.SetPowerScheme(restorePower)
	}
	os.Exit(code)
}

// syncAutoProfile пересоздаёт профиль auto, если с момента его создания
// изменился объём памяти или число потоков процессора.
func syncAutoProfile(s config.Settings) config.Settings {
	ram, threads := sys.TotalRAMGB(), runtime.NumCPU()
	if s.AutoHW != nil && s.AutoHW.RAMGB == ram && s.AutoHW.Threads == threads {
		return s
	}
	h := hw.Detect() // число физических ядер — только через PowerShell, поэтому лишь при изменениях
	stamp := config.HWStamp{RAMGB: ram, Cores: h.Cores, Threads: threads}
	notes, err := config.SaveAutoProfile(stamp)
	if err != nil {
		log.Printf("[auto] не удалось обновить профиль: %v", err)
		return s
	}
	if s.AutoHW != nil {
		log.Printf("[auto] железо изменилось (%d→%d ГБ, %d→%d потоков), профиль auto пересоздан",
			s.AutoHW.RAMGB, stamp.RAMGB, s.AutoHW.Threads, stamp.Threads)
	} else {
		log.Printf("[auto] профиль auto привязан к железу: %d ГБ, %d ядер / %d потоков", stamp.RAMGB, stamp.Cores, stamp.Threads)
	}
	for _, n := range notes {
		log.Printf("[auto] %s", n)
	}
	return config.LoadSettings()
}

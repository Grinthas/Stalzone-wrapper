// Package config хранит настройки wrapper и JVM-профили.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Settings — состояние установки, лежит рядом с exe в wrapper.json.
type Settings struct {
	// Targets — полные пути к java.exe/javaw.exe игры, на которые ставится hook.
	Targets  []string `json:"targets"`
	Profile  string   `json:"profile"`
	Language string   `json:"language"`
	Memory   *Memory  `json:"memory,omitempty"`
	// Servers — выбранные узлы подключения: регион → host:port. Пусто — выбор игры.
	Servers map[string]string `json:"servers,omitempty"`
	Tweaks  Tweaks            `json:"tweaks"`
	// AutoHW — железо, под которое создан профиль auto; при изменении профиль пересоздаётся.
	AutoHW *HWStamp `json:"auto_hw,omitempty"`
}

// HWStamp — характеристики, от которых зависит профиль auto.
type HWStamp struct {
	RAMGB   int `json:"ram_gb"`
	Cores   int `json:"cores"`
	Threads int `json:"threads"`
}

// Tweaks — системные настройки на время игры.
type Tweaks struct {
	HighPriority bool `json:"high_priority"` // приоритет «Выше среднего» для игры
	PowerPlan    bool `json:"power_plan"`    // схема «Высокая производительность», пока идёт игра
	GPU          bool `json:"gpu"`           // запуск на дискретной видеокарте
}

// Memory — настройки очистки памяти (аналог Mem Reduct).
type Memory struct {
	WorkingSets  bool `json:"working_sets"`
	StandbyList  bool `json:"standby_list"`
	ModifiedList bool `json:"modified_list"`
	FileCache    bool `json:"file_cache"`
	// Auto — очищать автоматически, когда занято не меньше Threshold процентов RAM.
	Auto      bool `json:"auto"`
	Threshold int  `json:"threshold"`
}

// MemorySettings возвращает настройки памяти или значения по умолчанию.
func (s Settings) MemorySettings() Memory {
	if s.Memory != nil {
		return *s.Memory
	}
	return Memory{WorkingSets: true, StandbyList: true, ModifiedList: true, FileCache: true, Threshold: 85}
}

// Profile — набор JVM-параметров. Нулевые значения означают «не задавать».
type Profile struct {
	HeapSizeGB              int      `json:"heap_size_gb"`
	PreTouch                bool     `json:"pre_touch"`
	MetaspaceMB             int      `json:"metaspace_mb"`
	MaxGCPauseMillis        int      `json:"max_gc_pause_millis"`
	G1HeapRegionSizeMB      int      `json:"g1_heap_region_size_mb"`
	G1NewSizePercent        int      `json:"g1_new_size_percent"`
	G1MaxNewSizePercent     int      `json:"g1_max_new_size_percent"`
	G1ReservePercent        int      `json:"g1_reserve_percent"`
	G1HeapWastePercent      int      `json:"g1_heap_waste_percent"`
	G1MixedGCCountTarget    int      `json:"g1_mixed_gc_count_target"`
	InitiatingHeapOccupancy int      `json:"initiating_heap_occupancy_percent"`
	G1MixedGCLiveThreshold  int      `json:"g1_mixed_gc_live_threshold_percent"`
	G1RSetUpdatingPauseTime int      `json:"g1_rset_updating_pause_time_percent"`
	SurvivorRatio           int      `json:"survivor_ratio"`
	MaxTenuringThreshold    int      `json:"max_tenuring_threshold"`
	ParallelGCThreads       int      `json:"parallel_gc_threads"`
	ConcGCThreads           int      `json:"conc_gc_threads"`
	ReservedCodeCacheSizeMB int      `json:"reserved_code_cache_size_mb"`
	UseStringDeduplication  bool     `json:"use_string_deduplication"`
	UseLargePages           bool     `json:"use_large_pages"`
	ExtraArgs               []string `json:"extra_args,omitempty"`
}

// Flags превращает профиль в аргументы командной строки JVM.
func (p Profile) Flags() []string {
	f := []string{"-XX:+UseG1GC", "-XX:+UnlockExperimentalVMOptions", "-XX:+DisableExplicitGC"}
	add := func(format string, v int) {
		if v > 0 {
			f = append(f, fmt.Sprintf(format, v))
		}
	}
	if p.HeapSizeGB > 0 {
		f = append(f, fmt.Sprintf("-Xms%dG", p.HeapSizeGB), fmt.Sprintf("-Xmx%dG", p.HeapSizeGB))
	}
	if p.PreTouch {
		f = append(f, "-XX:+AlwaysPreTouch")
	}
	add("-XX:MaxMetaspaceSize=%dM", p.MetaspaceMB)
	add("-XX:MaxGCPauseMillis=%d", p.MaxGCPauseMillis)
	add("-XX:G1HeapRegionSize=%dM", p.G1HeapRegionSizeMB)
	add("-XX:G1NewSizePercent=%d", p.G1NewSizePercent)
	add("-XX:G1MaxNewSizePercent=%d", p.G1MaxNewSizePercent)
	add("-XX:G1ReservePercent=%d", p.G1ReservePercent)
	add("-XX:G1HeapWastePercent=%d", p.G1HeapWastePercent)
	add("-XX:G1MixedGCCountTarget=%d", p.G1MixedGCCountTarget)
	add("-XX:InitiatingHeapOccupancyPercent=%d", p.InitiatingHeapOccupancy)
	add("-XX:G1MixedGCLiveThresholdPercent=%d", p.G1MixedGCLiveThreshold)
	add("-XX:G1RSetUpdatingPauseTimePercent=%d", p.G1RSetUpdatingPauseTime)
	add("-XX:SurvivorRatio=%d", p.SurvivorRatio)
	if p.MaxTenuringThreshold > 0 {
		f = append(f, fmt.Sprintf("-XX:MaxTenuringThreshold=%d", p.MaxTenuringThreshold))
	}
	add("-XX:ParallelGCThreads=%d", p.ParallelGCThreads)
	add("-XX:ConcGCThreads=%d", p.ConcGCThreads)
	add("-XX:ReservedCodeCacheSize=%dM", p.ReservedCodeCacheSizeMB)
	if p.UseStringDeduplication {
		f = append(f, "-XX:+UseStringDeduplication")
	}
	if p.UseLargePages {
		f = append(f, "-XX:+UseLargePages")
	}
	return append(f, p.ExtraArgs...)
}

// Dir — папка, где лежат exe wrapper.
func Dir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func ConfigsDir() string { return filepath.Join(Dir(), "configs") }

func settingsPath() string { return filepath.Join(Dir(), "wrapper.json") }

func LoadSettings() Settings {
	s := Settings{Language: "ru"}
	data, err := os.ReadFile(settingsPath())
	if err == nil {
		_ = json.Unmarshal(data, &s)
	}
	return s
}

func SaveSettings(s Settings) error {
	data, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(settingsPath(), data, 0o644)
}

// ListProfiles возвращает имена профилей из configs/ без .json.
func ListProfiles() []string {
	entries, _ := os.ReadDir(ConfigsDir())
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	sort.Strings(names)
	return names
}

func LoadProfile(name string) (Profile, error) {
	var p Profile
	if name == "" || strings.ContainsAny(name, `/\`) {
		return p, errors.New("invalid config name")
	}
	data, err := os.ReadFile(filepath.Join(ConfigsDir(), name+".json"))
	if err != nil {
		return p, err
	}
	return p, json.Unmarshal(data, &p)
}

func SaveProfile(name string, p Profile) error {
	if err := os.MkdirAll(ConfigsDir(), 0o755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(p, "", "  ")
	return os.WriteFile(filepath.Join(ConfigsDir(), name+".json"), data, 0o644)
}

// Generate создаёт профили под объём RAM и число ядер.
func Generate(ramGB, cpus int) map[string]Profile {
	heap := func(frac float64, max int) int {
		h := int(float64(ramGB) * frac)
		if h < 2 {
			h = 2
		}
		if h > max {
			h = max
		}
		return h
	}
	threads := cpus
	if threads > 8 {
		threads = 8
	}
	if threads < 2 {
		threads = 2
	}
	conc := threads / 3
	if conc < 1 {
		conc = 1
	}
	base := Profile{
		PreTouch: true, MetaspaceMB: 512, MaxGCPauseMillis: 150, G1HeapRegionSizeMB: 8,
		G1NewSizePercent: 30, G1MaxNewSizePercent: 40, G1ReservePercent: 20, G1HeapWastePercent: 5,
		G1MixedGCCountTarget: 4, InitiatingHeapOccupancy: 15, G1MixedGCLiveThreshold: 90,
		G1RSetUpdatingPauseTime: 5, SurvivorRatio: 32, MaxTenuringThreshold: 1,
		ParallelGCThreads: threads, ConcGCThreads: conc, ReservedCodeCacheSizeMB: 400,
	}
	low, bal, high := base, base, base
	low.HeapSizeGB, low.PreTouch, low.ReservedCodeCacheSizeMB = heap(0.35, 4), false, 256
	bal.HeapSizeGB = heap(0.4, 6)
	high.HeapSizeGB, high.MaxGCPauseMillis = heap(0.5, 10), 100
	return map[string]Profile{"low": low, "balanced": bal, "high": high}
}

// AutoProfile подбирает профиль под конкретный компьютер и возвращает
// пояснения к выбору.
func AutoProfile(ramGB, cores, threads int) (Profile, []string) {
	var notes []string
	if cores < 1 {
		cores = max(1, threads/2)
	}

	// Куча: игре нужно 3–8 ГБ; остальное оставляем системе, браузеру и Discord.
	heap := 2
	switch {
	case ramGB <= 6:
		heap = 2
		notes = append(notes, "Мало памяти: куча 2 ГБ. Закройте браузер перед игрой и включите файл подкачки.")
	case ramGB <= 8:
		heap = 3
		notes = append(notes, "8 ГБ ОЗУ: куча 3 ГБ, файл подкачки обязателен.")
	case ramGB <= 12:
		heap = 4
	case ramGB <= 16:
		heap = 6
	case ramGB <= 32:
		heap = 8
	default:
		heap = 10
	}

	// Потоки GC: по числу физических ядер, но не больше 8, и одно-два ядра оставляем игре.
	parallel := min(8, max(2, cores))
	if cores <= 4 {
		parallel = max(2, cores-1)
		notes = append(notes, fmt.Sprintf("%d ядра: GC использует %d потока, чтобы не отнимать ядро у игры.", cores, parallel))
	}
	conc := max(1, parallel/4)

	p := Profile{
		HeapSizeGB: heap, MetaspaceMB: 512, G1HeapRegionSizeMB: 8,
		G1NewSizePercent: 30, G1MaxNewSizePercent: 40, G1ReservePercent: 20, G1HeapWastePercent: 5,
		G1MixedGCCountTarget: 4, InitiatingHeapOccupancy: 15, G1MixedGCLiveThreshold: 90,
		G1RSetUpdatingPauseTime: 5, SurvivorRatio: 32, MaxTenuringThreshold: 1,
		ParallelGCThreads: parallel, ConcGCThreads: conc,
	}
	// Короткие паузы GC стоят процессорного времени — на слабых CPU даём больше запаса.
	switch {
	case cores <= 4:
		p.MaxGCPauseMillis = 200
	case cores <= 6:
		p.MaxGCPauseMillis = 150
	default:
		p.MaxGCPauseMillis = 100
	}
	// Предварительное выделение памяти ускоряет игру, но удлиняет запуск и занимает ОЗУ сразу.
	p.PreTouch = ramGB >= 16
	if p.PreTouch {
		notes = append(notes, "Достаточно памяти: включено предварительное выделение кучи (меньше фризов в бою).")
	}
	if ramGB >= 12 {
		p.ReservedCodeCacheSizeMB = 400
	} else {
		p.ReservedCodeCacheSizeMB = 256
	}
	notes = append(notes, fmt.Sprintf("Куча %d ГБ, потоков GC %d/%d, цель паузы %d мс.", heap, parallel, conc, p.MaxGCPauseMillis))
	return p, notes
}

// SaveAutoProfile создаёт профиль auto под железо, делает его активным и запоминает железо.
func SaveAutoProfile(hw HWStamp) ([]string, error) {
	p, notes := AutoProfile(hw.RAMGB, hw.Cores, hw.Threads)
	if err := SaveProfile("auto", p); err != nil {
		return nil, err
	}
	s := LoadSettings()
	s.Profile = "auto"
	s.AutoHW = &hw
	return notes, SaveSettings(s)
}

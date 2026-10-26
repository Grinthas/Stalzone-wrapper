// Stalzone Wrapper.exe — оконная программа управления wrapper.
// Интерфейс — HTML в WebView2, логика — Go-функции, доступные из JS.
package main

import (
	_ "embed"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"github.com/jchv/go-webview2"

	"github.com/grinthas/stalzone-wrapper/internal/config"
	"github.com/grinthas/stalzone-wrapper/internal/ifeo"
	"github.com/grinthas/stalzone-wrapper/internal/sys"
)

//go:embed index.html
var page string

type target struct {
	Path      string `json:"path"`
	Installed bool   `json:"installed"`
}

type profileInfo struct {
	Name   string   `json:"name"`
	HeapGB int      `json:"heapGB"`
	Flags  []string `json:"flags"`
}

type state struct {
	Admin    bool          `json:"admin"`
	RAM      int           `json:"ram"`
	CPUs     int           `json:"cpus"`
	Language string        `json:"language"`
	Profile  string        `json:"profile"`
	Targets  []target      `json:"targets"`
	Profiles []profileInfo `json:"profiles"`
	Dir      string        `json:"dir"`
}

func getState() state {
	s := config.LoadSettings()
	st := state{Admin: sys.IsAdmin(), RAM: sys.TotalRAMGB(), CPUs: runtime.NumCPU(),
		Language: s.Language, Profile: s.Profile, Dir: config.Dir(),
		Targets: []target{}, Profiles: []profileInfo{}}
	for _, t := range s.Targets {
		st.Targets = append(st.Targets, target{t, ifeo.Installed(t)})
	}
	for _, n := range config.ListProfiles() {
		p, err := config.LoadProfile(n)
		if err == nil {
			st.Profiles = append(st.Profiles, profileInfo{n, p.HeapSizeGB, p.Flags()})
		}
	}
	return st
}

var errNeedAdmin = errors.New("need_admin")

func generate() error {
	for name, p := range config.Generate(sys.TotalRAMGB(), runtime.NumCPU()) {
		if err := config.SaveProfile(name, p); err != nil {
			return err
		}
	}
	s := config.LoadSettings()
	if s.Profile == "" {
		s.Profile = "balanced"
		return config.SaveSettings(s)
	}
	return nil
}

func install(path string) error {
	if !sys.IsAdmin() {
		return errNeedAdmin
	}
	path, _ = filepath.Abs(strings.Trim(strings.TrimSpace(path), `"`))
	if fi, err := os.Stat(path); err != nil || fi.IsDir() {
		return errors.New("not_found")
	}
	if err := ifeo.Install(path, filepath.Join(config.Dir(), "service.exe")); err != nil {
		return err
	}
	s := config.LoadSettings()
	known := false
	for _, t := range s.Targets {
		known = known || strings.EqualFold(t, path)
	}
	if !known {
		s.Targets = append(s.Targets, path)
	}
	if err := config.SaveSettings(s); err != nil {
		return err
	}
	if len(config.ListProfiles()) == 0 {
		return generate()
	}
	return nil
}

func uninstall(path string) error {
	if !sys.IsAdmin() {
		return errNeedAdmin
	}
	if err := ifeo.Uninstall(path); err != nil {
		return err
	}
	s := config.LoadSettings()
	var rest []string
	for _, t := range s.Targets {
		if !strings.EqualFold(t, path) {
			rest = append(rest, t)
		}
	}
	s.Targets = rest
	return config.SaveSettings(s)
}

func main() {
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		AutoFocus: true,
		DataPath:  filepath.Join(os.TempDir(), "StalzoneWrapper"),
		WindowOptions: webview2.WindowOptions{
			Title: "Stalzone Wrapper", Width: 940, Height: 640, Center: true,
		},
	})
	if w == nil {
		msg, _ := syscall.UTF16PtrFromString("Нужен Microsoft Edge WebView2 Runtime: https://go.microsoft.com/fwlink/p/?LinkId=2124703")
		title, _ := syscall.UTF16PtrFromString("Stalzone Wrapper")
		syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW").Call(0,
			uintptr(unsafe.Pointer(msg)), uintptr(unsafe.Pointer(title)), 0x10)
		return
	}
	defer w.Destroy()

	w.Bind("getState", getState)
	w.Bind("install", install)
	w.Bind("uninstall", uninstall)
	w.Bind("generate", generate)
	w.Bind("pickJava", func(title string) string { return sys.OpenFileDialog(uintptr(w.Window()), title) })
	w.Bind("selectProfile", func(name string) error {
		if _, err := config.LoadProfile(name); err != nil {
			return err
		}
		s := config.LoadSettings()
		s.Profile = name
		return config.SaveSettings(s)
	})
	w.Bind("setLanguage", func(lang string) error {
		s := config.LoadSettings()
		s.Language = lang
		return config.SaveSettings(s)
	})
	w.Bind("elevate", func() error {
		if err := sys.Elevate(); err != nil {
			return err
		}
		w.Terminate()
		return nil
	})
	w.Bind("openFolder", func(sub string) {
		dir := config.Dir()
		if sub == "configs" {
			os.MkdirAll(config.ConfigsDir(), 0o755)
			dir = config.ConfigsDir()
		}
		sys.Command("explorer.exe", dir).Start()
	})
	w.Bind("readLog", func() string {
		data, err := os.ReadFile(filepath.Join(config.Dir(), "wrapper.log"))
		if err != nil {
			return ""
		}
		return string(data)
	})

	w.SetHtml(page)
	w.Run()
}

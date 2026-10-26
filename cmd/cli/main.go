// cli.exe — меню управления wrapper: установка hook, выбор профиля, язык.
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/grinthas/stalzone-wrapper/internal/config"
	"github.com/grinthas/stalzone-wrapper/internal/i18n"
	"github.com/grinthas/stalzone-wrapper/internal/ifeo"
	"github.com/grinthas/stalzone-wrapper/internal/sys"
)

var in = bufio.NewReader(os.Stdin)

func readLine(prompt string) string {
	fmt.Print(prompt)
	s, _ := in.ReadString('\n')
	return strings.Trim(strings.TrimSpace(s), `"`)
}

func pause() { readLine(i18n.T("enter")) }

func main() {
	s := config.LoadSettings()
	i18n.Set(s.Language)

	// Повторный запуск с правами администратора: cli.exe install "<путь>"
	if len(os.Args) == 3 && os.Args[1] == "install" {
		install(&s, os.Args[2])
		pause()
		return
	}

	for {
		profile := s.Profile
		if profile == "" {
			profile = i18n.T("unset")
		}
		fmt.Printf("\n== %s ==\n%s\n\n", i18n.T("title"), i18n.T("profile.current", profile))
		items := []string{"menu.install", "menu.uninstall", "menu.status", "menu.profile", "menu.generate", "menu.language", "menu.exit"}
		for i, k := range items {
			fmt.Printf("%d) %s\n", i+1, i18n.T(k))
		}
		switch readLine("\n" + i18n.T("prompt")) {
		case "1":
			install(&s, readLine(i18n.T("ask.target")))
		case "2":
			uninstall(&s)
		case "3":
			status(s)
		case "4":
			selectProfile(&s)
		case "5":
			generate(&s)
		case "6":
			if s.Language == "en" {
				s.Language = "ru"
			} else {
				s.Language = "en"
			}
			i18n.Set(s.Language)
			save(s)
			continue
		case "7", "q", "Q":
			return
		default:
			continue
		}
		pause()
	}
}

func save(s config.Settings) {
	if err := config.SaveSettings(s); err != nil {
		fmt.Println(i18n.T("error", err))
	}
}

func install(s *config.Settings, target string) {
	target, _ = filepath.Abs(target)
	if fi, err := os.Stat(target); err != nil || fi.IsDir() {
		fmt.Println(i18n.T("err.notfound", target))
		return
	}
	if !sys.IsAdmin() {
		fmt.Println(i18n.T("need.admin"))
		if err := sys.Elevate("install", target); err != nil {
			fmt.Println(i18n.T("error", err))
		}
		return
	}
	service := filepath.Join(config.Dir(), "service.exe")
	if err := ifeo.Install(target, service); err != nil {
		fmt.Println(i18n.T("error", err))
		return
	}
	for _, t := range s.Targets {
		if strings.EqualFold(t, target) {
			goto done
		}
	}
	s.Targets = append(s.Targets, target)
done:
	if s.Profile == "" {
		if names := config.ListProfiles(); len(names) == 0 {
			generate(s)
		}
		s.Profile = "balanced"
	}
	save(*s)
	fmt.Println(i18n.T("installed", target))
}

func uninstall(s *config.Settings) {
	if len(s.Targets) == 0 {
		fmt.Println(i18n.T("no.targets"))
		return
	}
	if !sys.IsAdmin() {
		fmt.Println(i18n.T("need.admin"))
		sys.Elevate()
		return
	}
	for _, t := range s.Targets {
		if err := ifeo.Uninstall(t); err != nil {
			fmt.Println(i18n.T("error", err))
			return
		}
		fmt.Println(i18n.T("uninstalled", t))
	}
	s.Targets = nil
	save(*s)
}

func status(s config.Settings) {
	if len(s.Targets) == 0 {
		fmt.Println(i18n.T("no.targets"))
	}
	for _, t := range s.Targets {
		if ifeo.Installed(t) {
			fmt.Println(i18n.T("status.on", t))
		} else {
			fmt.Println(i18n.T("status.off", t))
		}
	}
}

func selectProfile(s *config.Settings) {
	names := config.ListProfiles()
	if len(names) == 0 {
		fmt.Println(i18n.T("no.configs"))
		return
	}
	for i, n := range names {
		fmt.Printf("%d) %s\n", i+1, n)
	}
	fmt.Println(i18n.T("back"))
	n, err := strconv.Atoi(readLine(i18n.T("prompt")))
	if err != nil || n < 1 || n > len(names) {
		return
	}
	s.Profile = names[n-1]
	save(*s)
	fmt.Println(i18n.T("selected", s.Profile))
}

func generate(s *config.Settings) {
	ram, cpus := sys.TotalRAMGB(), runtime.NumCPU()
	for name, p := range config.Generate(ram, cpus) {
		if err := config.SaveProfile(name, p); err != nil {
			fmt.Println(i18n.T("error", err))
			return
		}
	}
	fmt.Println(i18n.T("generated", ram, cpus))
	if ram < 8 {
		fmt.Println(i18n.T("mem.low"))
	}
	if s.Profile == "" {
		s.Profile = "balanced"
		save(*s)
	}
}

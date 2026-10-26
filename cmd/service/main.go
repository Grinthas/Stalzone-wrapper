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

	"github.com/grinthas/stalzone-wrapper/internal/config"
	"github.com/grinthas/stalzone-wrapper/internal/jvm"
	"github.com/grinthas/stalzone-wrapper/internal/sys"
	"github.com/grinthas/stalzone-wrapper/internal/tunnel"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("service.exe запускается Windows автоматически; используйте cli.exe")
		os.Exit(2)
	}
	if f, err := os.OpenFile(filepath.Join(config.Dir(), "wrapper.log"),
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644); err == nil {
		log.SetOutput(f)
		defer f.Close()
	}

	target, args := os.Args[1], os.Args[2:]
	s := config.LoadSettings()
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

	proc, err := sys.StartBypassingIFEO(target, args)
	if err != nil {
		log.Printf("[launch] ошибка: %v", err)
		os.Exit(1)
	}
	st, err := proc.Wait()
	if err != nil {
		os.Exit(1)
	}
	os.Exit(st.ExitCode())
}

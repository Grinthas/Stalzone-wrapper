// Package jvm переписывает командную строку запуска Java.
package jvm

import "strings"

// optionsWithValue — опции, значение которых идёт отдельным аргументом.
var optionsWithValue = map[string]bool{
	"-cp": true, "-classpath": true, "--class-path": true,
	"-p": true, "--module-path": true, "--add-modules": true,
	"--add-opens": true, "--add-exports": true, "--add-reads": true,
	"--enable-native-access": true,
}

// overridden сообщает, задаёт ли наш профиль этот флаг сам.
func overridden(arg string) bool {
	return strings.HasPrefix(arg, "-Xmx") || strings.HasPrefix(arg, "-Xms") ||
		strings.HasPrefix(arg, "-XX:")
}

// Rewrite убирает из JVM-части аргументов флаги памяти и -XX:, вставляет flags
// и оставляет без изменений главный класс/-jar и аргументы самой игры.
func Rewrite(args, flags []string) []string {
	out := append([]string{}, flags...)
	i := 0
	for i < len(args) {
		a := args[i]
		if a == "-jar" || a == "-m" || a == "--module" || !strings.HasPrefix(a, "-") {
			break // дальше идут главный класс и аргументы приложения
		}
		if optionsWithValue[a] && i+1 < len(args) {
			out = append(out, a, args[i+1])
			i += 2
			continue
		}
		if !overridden(a) {
			out = append(out, a)
		}
		i++
	}
	return append(out, args[i:]...)
}

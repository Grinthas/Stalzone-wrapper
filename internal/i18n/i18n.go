// Package i18n — тексты интерфейса на русском и английском.
package i18n

import "fmt"

var lang = "ru"

func Set(l string) {
	if l == "en" {
		lang = "en"
	} else {
		lang = "ru"
	}
}

// T возвращает строку по ключу; аргументы подставляются через fmt.
func T(key string, a ...any) string {
	m := ru
	if lang == "en" {
		m = en
	}
	s, ok := m[key]
	if !ok {
		s = key
	}
	if len(a) > 0 {
		return fmt.Sprintf(s, a...)
	}
	return s
}

var ru = map[string]string{
	"title":           "Stalzone Wrapper — главное меню",
	"menu.install":    "Установить hook",
	"menu.uninstall":  "Удалить hook",
	"menu.status":     "Статус",
	"menu.profile":    "Выбрать конфиг",
	"menu.generate":   "Пересоздать конфиги",
	"menu.language":   "Язык / Language",
	"menu.exit":       "Выход",
	"prompt":          "Выберите пункт: ",
	"profile.current": "Профиль: %s",
	"unset":           "не выбран",
	"ask.target":      "Путь к java.exe/javaw.exe игры (Диспетчер задач → Подробности → «Открыть расположение файла»): ",
	"err.notfound":    "Файл не найден: %s",
	"need.admin":      "Нужны права администратора, перезапускаю...",
	"installed":       "Hook установлен: %s",
	"uninstalled":     "Hook удалён: %s",
	"no.targets":      "Hook ещё не устанавливался.",
	"status.on":       "[вкл]  %s",
	"status.off":      "[выкл] %s",
	"no.configs":      "Нет конфигов. Сначала пересоздайте их.",
	"selected":        "Выбран профиль: %s",
	"generated":       "RAM: %d GB, ядер: %d. Созданы профили: low, balanced, high.",
	"mem.low":         "Меньше 8 GB RAM: включите файл подкачки, чтобы избежать зависаний.",
	"back":            "0) Назад",
	"error":           "Ошибка: %v",
	"enter":           "Нажмите Enter...",
}

var en = map[string]string{
	"title":           "Stalzone Wrapper — main menu",
	"menu.install":    "Install hook",
	"menu.uninstall":  "Remove hook",
	"menu.status":     "Status",
	"menu.profile":    "Select config",
	"menu.generate":   "Regenerate configs",
	"menu.language":   "Language / Язык",
	"menu.exit":       "Exit",
	"prompt":          "Choose an option: ",
	"profile.current": "Profile: %s",
	"unset":           "not set",
	"ask.target":      "Path to the game's java.exe/javaw.exe (Task Manager → Details → Open file location): ",
	"err.notfound":    "File not found: %s",
	"need.admin":      "Administrator rights required, restarting...",
	"installed":       "Hook installed: %s",
	"uninstalled":     "Hook removed: %s",
	"no.targets":      "Hook has not been installed yet.",
	"status.on":       "[on]  %s",
	"status.off":      "[off] %s",
	"no.configs":      "No configs. Regenerate them first.",
	"selected":        "Selected profile: %s",
	"generated":       "RAM: %d GB, cores: %d. Created profiles: low, balanced, high.",
	"mem.low":         "Less than 8 GB RAM: enable the page file to avoid freezes.",
	"back":            "0) Back",
	"error":           "Error: %v",
	"enter":           "Press Enter...",
}

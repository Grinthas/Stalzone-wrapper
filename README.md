# Stalzone Wrapper

Подставляет оптимизированные JVM-флаги при запуске Java-клиента Stalzone.
Работает через IFEO hook Windows в режиме фильтра по полному пути, поэтому
перехватывается только Java игры, а не все Java-программы на компьютере.

## Сборка

```
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H windowsgui" -o "dist/Stalzone Wrapper.exe" ./cmd/gui
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H windowsgui" -o dist/service.exe ./cmd/service
```

## Использование

1. Положите `Stalzone Wrapper.exe` и `service.exe` в одну постоянную папку.
2. Запустите `Stalzone Wrapper.exe` (при необходимости — «Перезапустить от администратора»).
3. На «Главной» нажмите «Обзор…», выберите `java.exe`/`javaw.exe` игры и «Установить».
4. На вкладке «Профили» выберите `low`, `balanced` или `high` (создаются под ваш ПК в `configs/`).
   Можно положить свой JSON-профиль (пример: `examples/aikar.json`).
5. Запустите игру. Что было передано в Java — на вкладке «Журнал».

Требуется Windows 10 1607+ и WebView2 Runtime (есть в Windows 10/11 по умолчанию).

## Как работает

`service.exe` получает исходную команду запуска, убирает из JVM-части `-Xmx`, `-Xms` и `-XX:*`,
добавляет флаги профиля и запускает настоящую Java (обходя повторный вызов IFEO).
Аргументы самой игры после главного класса / `-jar` не трогаются.

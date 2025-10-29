# Employee Monitoring Agent for Windows

Агент мониторинга активности сотрудников для Windows. Работает в фоновом режиме без GUI.

## Возможности

- ✅ **Мониторинг активности** - отслеживание активных окон и процессов
- ✅ **USB мониторинг** - обнаружение подключения USB устройств
- ✅ **Скриншоты** - периодический захват экрана
- ✅ **Мониторинг файлов** - отслеживание копирования больших объемов данных
- ✅ **Keylogger** - опциональная запись клавиатурного ввода (требует согласия)
- ✅ **Скрытый режим** - работа без отображения окон и диалогов
- ✅ **Защита от дублирования** - только один экземпляр может работать
- ✅ **Graceful shutdown** - корректное завершение работы

## Сборка

### Из macOS/Linux:

```bash
# Обычный режим (с консолью для отладки)
make build-windows

# Скрытый режим (без консоли, для production)
make build-service

# С пакетированием
make package
```

### Флаги сборки:

- `-H=windowsgui` - убирает консольное окно
- `-s -w` - удаляет отладочную информацию (уменьшает размер)

## Установка

### Ручная установка:

1. Скопируйте `employee-agent.exe` на целевую машину
2. Создайте конфигурационный файл `config.yaml` в той же директории
3. Запустите агент от имени пользователя

### Установка как Windows Service:

Для установки агента как службы Windows используйте утилиту `sc`:

```cmd
sc create "EmployeeMonitorAgent" binPath="C:\Path\To\employee-agent.exe" start=auto
sc description "EmployeeMonitorAgent" "Employee activity monitoring service"
sc start "EmployeeMonitorAgent"
```

**Важно:** Для работы как служба требуется переделать код с использованием `golang.org/x/sys/windows/svc`.

### Автозапуск через планировщик задач:

Более простой способ - использовать Task Scheduler:

```cmd
schtasks /create /tn "EmployeeMonitor" /tr "C:\Path\To\employee-agent.exe" /sc onlogon /rl highest
```

## Конфигурация

Пример `config.yaml`:

```yaml
agent:
  computer_name: "${COMPUTERNAME}"
  api_key: "your-api-key-here"
  server:
    url: "http://monitoring-server:5000"
    timeout_seconds: 30
    retry_attempts: 3
    retry_delay_seconds: 5

activity_monitoring:
  enabled: true
  interval_seconds: 30

usb_monitoring:
  enabled: true
  shadow_copy_enabled: false

screenshots:
  enabled: false  # Включать только после получения согласия
  interval_minutes: 15

keylogger:
  enabled: false  # ТРЕБУЕТ ЯВНОГО СОГЛАСИЯ

logging:
  level: "info"
  file: "C:\\ProgramData\\MonitoringAgent\\agent.log"
```

## Логирование

Логи пишутся в:
- `C:\ProgramData\MonitoringAgent\agent.log` (по умолчанию)
- Или в путь, указанный в конфигурации

Уровни логирования:
- `debug` - детальная отладочная информация
- `info` - нормальная работа (по умолчанию)
- `warn` - предупреждения
- `error` - ошибки

## Безопасность

### Скрытие от пользователя:

Агент автоматически:
- Скрывает консольное окно (`hideConsoleWindow`)
- Отключает диалоги об ошибках Windows (`disablePanicDialogs`)
- Перехватывает паники для предотвращения крашей
- Логирует все события в файл

### Защита от дублирования:

Используется глобальный mutex `Global\OfficeMonitoringAgent_SingleInstance` для предотвращения запуска нескольких копий.

### Рекомендации:

1. Всегда информируйте сотрудников о мониторинге
2. Получайте письменное согласие перед включением keylogger
3. Шифруйте передачу данных (используйте HTTPS)
4. Регулярно обновляйте агент
5. Настройте TTL для данных на сервере (GDPR compliance)

## Производительность

Ограничения ресурсов (в конфигурации):

```yaml
performance:
  max_memory_mb: 100      # Максимум памяти
  max_cpu_percent: 10     # Максимум CPU
  event_buffer_size: 1000 # Размер буфера событий
```

Типичное потребление:
- RAM: 20-50 MB
- CPU: 1-3% в режиме ожидания, до 10% при активности

## Troubleshooting

### Агент не запускается:

1. Проверьте логи в `C:\ProgramData\MonitoringAgent\agent.log`
2. Убедитесь что конфигурационный файл корректен
3. Проверьте доступность сервера

### Агент не отправляет данные:

1. Проверьте URL сервера в конфигурации
2. Проверьте API ключ
3. Убедитесь что нет файрвола блокирующего соединение

### Высокое потребление ресурсов:

1. Увеличьте `interval_seconds` для activity monitoring
2. Отключите screenshot capture
3. Уменьшите `event_buffer_size`

## Разработка

### Запуск в dev режиме:

```bash
# С консолью для отладки
go run main.go -config config.yaml
```

### Тестирование:

```bash
go test ./...
```

### Структура проекта:

```
agent/
├── main.go              # Точка входа, инициализация
├── config/              # Загрузка конфигурации
├── httpclient/          # HTTP клиент с retry
├── logger/              # Простой структурированный логгер
├── monitoring/          # Модули мониторинга
│   ├── activity_windows.go   # Activity tracker
│   ├── usb_windows.go         # USB monitor
│   ├── screenshot_windows.go  # Screenshot capture
│   ├── file_windows.go        # File operations monitor
│   ├── keylogger_windows.go   # Keylogger (опционально)
│   └── *_stub.go              # Заглушки для не-Windows
└── Makefile             # Команды сборки
```

## Лицензия

Использование данного ПО должно соответствовать законодательству РФ и локальным нормативным актам компании.

## TODO

- [ ] Реализация Windows Service mode
- [ ] Автоматическое обновление агента
- [ ] Удаленное управление конфигурацией
- [ ] Метрики производительности агента
- [ ] Поддержка proxy серверов
- [ ] E2E шифрование данных

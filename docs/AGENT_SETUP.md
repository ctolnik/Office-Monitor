# Установка агента на Windows

## Поведение службы (Event-Driven)

Служба работает по событийной модели и автоматически обнаруживает существующие сессии:

### При запуске/перезапуске службы:
- Служба запускается и читает конфигурацию
- Проверяет доступность сервера
- **Сканирует существующие сессии пользователей** через WTS API
- Если найдена активная или подключённая сессия — сразу запускает мониторинг
- Если сессий нет — ждёт события входа пользователя
- В логах: `Found N user sessions at startup`, `Session X: user=..., state=...`

### При входе пользователя:
- Windows отправляет событие `WTS_SESSION_LOGON`
- Служба определяет имя пользователя через WTS API
- Запускаются все компоненты мониторинга
- В логах: `Logon session N`, `Monitoring started`

### Поддерживаемые состояния сессий:
- **Active (0)** — пользователь активен на консоли
- **Connected (1)** — пользователь подключён (RDP)
- **Disconnected (4)** — сессия отключена но существует

### Логируемые события сессий:
- `Logon session N` — вход пользователя
- `Logoff session N` — выход пользователя  
- `Lock session N` — блокировка сессии
- `Unlock session N` — разблокировка сессии
- `Connect session N` — подключение к сессии (RDP)
- `Disconnect session N` — отключение от сессии

### Перезапуск службы:
При перезапуске службы мониторинг автоматически возобновится для текущих активных сессий. Не требуется повторный вход пользователя.

---

## Требования

- Windows 10/11 или Windows Server 2016+
- Права администратора для установки
- Сетевой доступ к серверу мониторинга

---

## Быстрая установка

### 1. Скопировать файлы

Создайте папку и скопируйте туда:
```
C:\Program Files\OfficeMonitor\
├── agent-svc.exe    # Основная служба (Session 0)
├── agent-sh.exe     # Universal session-helper (историческое имя "screenSHoot")
└── config.yaml
```

**Примечание:** `agent-sh.exe` — отдельный процесс, который запускается службой в сессии пользователя. Изначально он делал только скриншоты (screenSHoot), но теперь это **универсальный session-helper** для всего UI-зависимого функционала (activity tracking/idle time/скриншоты/в будущем keylogger hooks).

Целевой подход: session-helper не общается с сервером по HTTP напрямую — он отправляет данные в сервис через Named Pipe (IPC), а сервис выполняет доставку на сервер.

### 2. Создать config.yaml

```yaml
agent:
  computer_name: "PC-IVANOV"
  server:
    url: "http://192.168.1.100:5000"
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
  enabled: false
  interval_minutes: 15
  quality: 80

keylogger:
  enabled: false

logging:
  level: "info"
  file: "C:\\ProgramData\\OfficeMonitor\\agent.log"
```

**Важно:** `api_key` требуется. Агент (через сервис) отправляет ключ в заголовке `X-API-Key`, сервер валидирует. Сотрудник может создаваться автоматически при первом подключении по `computer_name`, но доступ должен быть защищён ключом.

### 3. Создать папку для логов

```powershell
New-Item -Path "C:\ProgramData\OfficeMonitor" -ItemType Directory -Force
```

### 4. Зарегистрировать Event Source (опционально)

Для записи в Windows Event Log:

```powershell
New-EventLog -LogName Application -Source "OfficeMonitorAgent"
```

### 5. Установить как службу

Запустите PowerShell от администратора:

```powershell
# Создать службу с указанием пути к конфигу
New-Service -Name "OfficeMonitorAgent" `
  -BinaryPathName '"C:\Program Files\OfficeMonitor\agent-svc.exe" -config "C:\Program Files\OfficeMonitor\config.yaml"' `
  -DisplayName "Office Monitor Agent" `
  -StartupType Automatic `
  -Description "Employee activity monitoring service"

# Настроить запуск от LocalSystem
sc.exe config OfficeMonitorAgent obj= "LocalSystem"

# Запустить
Start-Service OfficeMonitorAgent
```

**Важно:** Агент теперь поддерживает Windows Service API. При запуске как служба он автоматически определяет режим и корректно взаимодействует с SCM (Service Control Manager).

### 6. Проверить работу

```powershell
# Статус службы
Get-Service OfficeMonitorAgent

# Логи агента
Get-Content "C:\ProgramData\OfficeMonitor\agent.log" -Tail 50

# Windows Event Log
Get-EventLog -LogName Application -Source "OfficeMonitorAgent" -Newest 10
```

---

## Конфигурация

### Обязательные параметры

| Параметр | Описание |
|----------|----------|
| `agent.computer_name` | Имя компьютера (уникальное для каждого ПК) |
| `agent.server.url` | URL сервера мониторинга |

### Регистрация сотрудника

Сотрудник создаётся автоматически на сервере при первом подключении агента. Имя берётся из активной Windows-сессии через WTS API.

### Модули мониторинга

| Модуль | Описание | По умолчанию |
|--------|----------|--------------|
| `activity_monitoring` | Активные окна и процессы | Включено |
| `usb_monitoring` | USB устройства | Включено |
| `screenshots` | Периодические скриншоты | **Выключено** |
| `keylogger` | Запись клавиатуры | **Выключено** |

**Важно:** Скриншоты и keylogger по умолчанию выключены. Включайте только после получения письменного согласия сотрудников.

### Пример минимальной конфигурации

```yaml
agent:
  computer_name: "PC-001"
  server:
    url: "http://monitor-server:5000"

activity_monitoring:
  enabled: true
  interval_seconds: 30
```

---

## Работа как служба (LocalSystem)

При работе от LocalSystem агент использует WTS API для определения имени пользователя вместо переменных окружения.

Преимущества:
- Автозапуск при старте системы
- Работа до входа пользователя
- Устойчивость к завершению сессии

---

## Массовое развёртывание

### Через групповые политики (GPO)

1. Разместите `agent-svc.exe`, `agent-sh.exe` и `config.yaml` на сетевом ресурсе
2. Создайте GPO со скриптом установки
3. Привяжите GPO к нужному OU

### Скрипт установки

```powershell
# install-agent.ps1
$Source = "\\server\share\OfficeMonitor"
$Dest = "C:\Program Files\OfficeMonitor"

# Копировать файлы
New-Item -Path $Dest -ItemType Directory -Force
Copy-Item "$Source\*" $Dest -Force

# Создать службу
New-Service -Name "OfficeMonitorAgent" `
  -BinaryPathName "$Dest\agent-svc.exe" `
  -StartupType Automatic

# Запустить
Start-Service OfficeMonitorAgent
```

---

## Удаление

```powershell
# Остановить и удалить службу
Stop-Service OfficeMonitorAgent -Force
sc.exe delete OfficeMonitorAgent

# Удалить файлы
Remove-Item "C:\Program Files\OfficeMonitor" -Recurse -Force
Remove-Item "C:\ProgramData\OfficeMonitor" -Recurse -Force
```

---

## Troubleshooting

### Агент не запускается

1. Проверьте логи: `C:\ProgramData\OfficeMonitor\agent.log`
2. Проверьте Windows Event Log: `Get-EventLog -LogName Application -Source "OfficeMonitorAgent" -Newest 10`
3. Проверьте права доступа к папке
4. Проверьте синтаксис config.yaml
5. Убедитесь что путь к config.yaml указан в BinaryPathName службы

### Данные не отправляются

1. Проверьте URL сервера в config.yaml
2. Проверьте сетевой доступ: `Test-NetConnection monitor-server -Port 5000`
3. Проверьте firewall

### Скриншоты не работают

1. Убедитесь что `screenshots.enabled: true`
2. Перезапустите службу после изменения конфига
3. Проверьте права на папку MinIO на сервере

---

## Сборка из исходников

```bash
# На Linux/macOS (cross-compile)
cd agent
make build-service

# Результат: agent-svc.exe, agent-sh.exe
```

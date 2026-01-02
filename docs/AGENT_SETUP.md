# Установка агента на Windows

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
├── employee-agent.exe
└── config.yaml
```

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

**Примечание:** `api_key` не требуется - сервер не использует аутентификацию (предполагается работа в защищённой внутренней сети). Сотрудник автоматически создаётся на сервере при первом подключении агента по `computer_name`.

### 3. Установить как службу (рекомендуется)

Запустите PowerShell от администратора:

```powershell
# Создать службу
New-Service -Name "OfficeMonitorAgent" `
  -BinaryPathName "C:\Program Files\OfficeMonitor\employee-agent.exe" `
  -DisplayName "Office Monitor Agent" `
  -StartupType Automatic `
  -Description "Employee activity monitoring service"

# Настроить запуск от LocalSystem
sc.exe config OfficeMonitorAgent obj= "LocalSystem"

# Запустить
Start-Service OfficeMonitorAgent
```

### 4. Проверить работу

```powershell
# Статус службы
Get-Service OfficeMonitorAgent

# Логи
Get-Content "C:\ProgramData\OfficeMonitor\agent.log" -Tail 50
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

1. Разместите `employee-agent.exe` и `config.yaml` на сетевом ресурсе
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
  -BinaryPathName "$Dest\employee-agent.exe" `
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
2. Проверьте права доступа к папке
3. Проверьте синтаксис config.yaml

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

# Результат: agent/build/employee-agent.exe
```

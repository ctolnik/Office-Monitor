# Production Server Fixes

**Дата:** 24 ноября 2025  
**Проблемы:** 2 ошибки (400 "No valid events" + 500 activity segment)

---

## 🔴 Проблема 1: Error 500 на activity segments

### Симптомы:
```
2025/11/24 22:25:02 activity_tracker_windows.go:219: Server returned non-OK status for activity segment: 500
```

### Причина:
Таблица `monitoring.activity_segments` **не создана** в ClickHouse на production сервере.

### Решение:

✅ **Таблица УЖЕ ЕСТЬ в файле миграций `clickhouse/init.sql`!**

**Вариант А: Применить готовые миграции (РЕКОМЕНДУЕТСЯ)**

```bash
# 1. Подключиться к production серверу
ssh user@monitor.net.gslaudit.ru

# 2. Скопировать init.sql на production (если ещё нет)
scp clickhouse/init.sql user@monitor.net.gslaudit.ru:/opt/monitoring/clickhouse/

# 3. Применить миграции через Docker
cd /opt/monitoring
docker exec -i clickhouse clickhouse-client --database=monitoring < clickhouse/init.sql

# Готово! ✅
```

**Что создаст:**
- ✅ Таблица `monitoring.activity_segments`
- ✅ Materialized view `monitoring.daily_activity_summary`
- ✅ Materialized view `monitoring.program_usage_daily`
- ✅ Индексы для быстрого поиска

**Примечание:** `CREATE TABLE IF NOT EXISTS` безопасен - не затрёт существующие таблицы!

**Вариант Б: Через SQL файл**

```bash
# 1. Скопировать init.sql на production сервер
scp clickhouse/init.sql user@monitor.net.gslaudit.ru:/tmp/

# 2. Подключиться к серверу
ssh user@monitor.net.gslaudit.ru

# 3. Выполнить весь init.sql (создаст все таблицы)
docker exec -i clickhouse clickhouse-client < /tmp/init.sql

# 4. Проверить что таблица создалась
docker exec -it clickhouse clickhouse-client --query "SHOW TABLES FROM monitoring"
```

**Проверка:**

```bash
# Проверить что таблица создалась
docker exec -it clickhouse clickhouse-client --query "
SELECT count(*) FROM monitoring.activity_segments
"
```

После этого ошибка 500 должна исчезнуть.

---

## 🔴 Проблема 2: Error 400 "No valid events in batch"

### Симптомы:
```
2025/11/24 22:23:02 client.go:146: [request_id=...] client error 400 after 1.1491ms: {"error":"No valid events in batch"}
2025/11/24 22:23:02 eventbuffer.go:180: Failed to flush events to server: ...
```

Повторяется каждые 30 секунд.

### Причина:
Event buffer отправляет события с `type` отличным от поддерживаемых сервером.

### Текущие поддерживаемые типы:
- `activity` → вставка в `activity_events`
- `keyboard` → вставка в `keyboard_events`
- `usb` → вставка в `usb_events`
- `file` → вставка в `file_copy_events`
- `screenshot` → вставка в `screenshot_metadata`

### Решение A: Обновить сервер (добавить поддержку новых типов)

**Если агент отправляет неизвестные типы, добавьте их в handler.**

Откройте `server/main.go`, найдите `receiveBatchEventsHandler` (около строки 432):

```go
func receiveBatchEventsHandler(c *gin.Context) {
    // ... существующий код ...
    
    // Добавьте новые типы:
    switch eventType {
    case "activity":
        // ...
    case "keyboard":
        // ...
    case "alert":  // ДОБАВИТЬ если агент шлёт alerts
        // код для обработки alerts
    case "segment":  // ДОБАВИТЬ если агент шлёт segments
        // код для обработки segments
    default:
        zapctx.Warn(ctx, "Unknown event type", zap.String("type", eventType))
        // НЕ возвращать ошибку, просто пропустить
    }
}
```

### Решение B: Отключить отправку неподдерживаемых событий в агенте

Если некоторые модули отправляют события через `eventBuffer.Add()` с типами, которые сервер не поддерживает:

**Опция 1: Временно отключить модули**

В `agent/config.yaml`:

```yaml
# Временно отключить problematic мониторинг
file_monitoring:
  enabled: false  # Если file events не поддерживаются сервером

keylogger:
  enabled: false  # Если keyboard events не поддерживаются
```

**Опция 2: Изменить код модулей**

Найти в `agent/monitoring/*.go` все вызовы:

```go
eventBuffer.Add("unknown_type", data)
```

И изменить тип на поддерживаемый или убрать вызов.

### Решение C: Логировать и игнорировать

На сервере в `receiveBatchEventsHandler` после строки 476:

```go
default:
    zapctx.Warn(ctx, "Unknown event type, ignoring", 
        zap.String("type", eventType),
        zap.Any("data", eventData))
    // НЕ добавляем в validEvents, но и не ошибку не возвращаем
    continue  // Пропускаем это событие
}
```

**Это позволит серверу принимать события и игнорировать неизвестные типы.**

---

## 📊 Проверка после исправления

### 1. Перезапустить агент

```powershell
# Windows PowerShell (Administrator)
Restart-Service "MonitoringAgent"

# Или если запущен вручную
Stop-Process -Name "agent"
Start-Process "C:\Program Files\MonitoringAgent\agent.exe"
```

### 2. Проверить лог агента

```powershell
Get-Content "C:\ProgramData\MonitoringAgent\agent.log" -Tail 50 -Wait
```

**Ожидаемый результат:**
- ✅ Нет ошибок 500 на activity segment
- ✅ Нет ошибок 400 "No valid events"
- ✅ Успешные POST запросы (200 OK)

### 3. Проверить данные в ClickHouse

```bash
docker exec -it clickhouse clickhouse-client --query "
SELECT 
    computer_name, 
    username, 
    state, 
    count(*) as segments_count,
    sum(duration_sec) as total_seconds
FROM monitoring.activity_segments
WHERE toDate(timestamp_start) = today()
GROUP BY computer_name, username, state
ORDER BY computer_name, state
"
```

**Ожидаемый результат:**
```
┌─computer_name─┬─username─┬─state───┬─segments_count─┬─total_seconds─┐
│ ADM-01        │ a-kiv    │ active  │            245 │          7350 │
│ ADM-01        │ a-kiv    │ idle    │             12 │           360 │
└───────────────┴──────────┴─────────┴────────────────┴───────────────┘
```

---

## 🚀 Быстрое решение (минимум действий)

**Если нужно исправить прямо сейчас:**

```bash
# 1. SSH на production сервер
ssh user@monitor.net.gslaudit.ru

# 2. Создать только таблицу activity_segments
docker exec -i clickhouse clickhouse-client <<EOF
CREATE TABLE IF NOT EXISTS monitoring.activity_segments (
    timestamp_start DateTime64(3),
    timestamp_end DateTime64(3),
    duration_sec UInt32,
    state Enum8('active' = 1, 'idle' = 2, 'offline' = 3),
    computer_name String,
    username String,
    process_name String,
    window_title String,
    session_id String,
    event_date Date DEFAULT toDate(timestamp_start)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(event_date)
ORDER BY (computer_name, username, timestamp_start)
TTL event_date + INTERVAL 180 DAY;
EOF

# 3. Проверить
docker exec -it clickhouse clickhouse-client --query "SHOW TABLES FROM monitoring" | grep activity_segments
```

**Результат:** ✅ activity_segments

**После этого перезапустите агент на Windows машине.**

---

## 📝 Файлы для справки

- Schema: `clickhouse/init.sql` (полная инициализация)
- Server handler: `server/main.go:432-497` (receiveBatchEventsHandler)
- Agent buffer: `agent/buffer/eventbuffer.go` (отправка событий)
- Activity tracker: `agent/monitoring/activity_tracker_windows.go:201-221` (sendSegment)

---

**Дата:** 24 ноября 2025  
**Статус:** Готово к применению на production  
**Время исправления:** ~5 минут

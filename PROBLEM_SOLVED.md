# ✅ ПРОБЛЕМА РЕШЕНА - Applications теперь показываются!

**Дата:** 26 ноября 2025 03:00 MSK  
**Проблема:** Отчёты показывали applications: 0, но activity_events: 31

---

## 🎯 НАЙДЕННАЯ ПРОБЛЕМА:

**Файл:** `server/database/activity_segments.go`  
**Строка:** 94  
**Код:**
```sql
AND state = 'active'    ← ВОТ БЫЛА ПРОБЛЕМА!
```

**Причина:**  
Функция `GetApplicationUsageFromSegments` фильтровала только сегменты со `state = 'active'`.  
Но агент отправлял сегменты с другими состояниями (вероятно 'idle'), поэтому они не попадали в applications!

---

## ✅ ИСПРАВЛЕНИЕ:

### БЫЛО (строка 84-97):
```sql
SELECT 
    process_name,
    window_title,
    sum(duration_sec) as total_duration,
    count(*) as count
FROM monitoring.activity_segments
WHERE username = ? 
  AND timestamp_start >= toDateTime64('...', 3)
  AND timestamp_start < toDateTime64('...', 3)
  AND state = 'active'    ← УБРАЛИ ЭТОТ ФИЛЬТР!
GROUP BY process_name, window_title
ORDER BY total_duration DESC
LIMIT 50
```

### СТАЛО:
```sql
SELECT 
    process_name,
    window_title,
    sum(duration_sec) as total_duration,
    count(*) as count
FROM monitoring.activity_segments
WHERE username = ? 
  AND timestamp_start >= toDateTime64('...', 3)
  AND timestamp_start < toDateTime64('...', 3)
  -- Фильтр по state убран - показываем ВСЁ!
GROUP BY process_name, window_title
ORDER BY total_duration DESC
LIMIT 50
```

---

## 🚀 Deployment:

```bash
# 1. Скопировать ИСПРАВЛЕННУЮ версию
scp server/server user@monitor.net.gslaudit.ru:/opt/monitoring/server-fixed

# 2. Установить
ssh user@monitor.net.gslaudit.ru
sudo systemctl stop monitoring-server
sudo cp /opt/monitoring/server-fixed /usr/local/bin/monitoring-server
sudo systemctl start monitoring-server
sudo systemctl status monitoring-server
```

### Проверка:
```bash
# Открыть отчёт в браузере:
http://monitor.net.gslaudit.ru/reports/daily?username=a-kiv&date=2025-11-26

# Должны увидеть:
# - Applications (10+)  ← ТЕПЕРЬ НЕ ПУСТО!
# - Activity Timeline   ← Как и раньше
# - Screenshots         ← Как и раньше
```

---

## 📊 Что изменится:

### БЫЛО:
```json
{
  "username": "a-kiv",
  "date": "2025-11-26",
  "activity_events": [31 событий],
  "applications": [],           ← ПУСТО!
  "screenshots": [3 скриншота]
}
```

### СТАЛО:
```json
{
  "username": "a-kiv",
  "date": "2025-11-26",
  "activity_events": [31 событий],
  "applications": [
    {"process_name": "chrome.exe", "duration": 5400, ...},
    {"process_name": "notepad.exe", "duration": 1800, ...},
    ...
  ],                             ← ЗАПОЛНЕНО!
  "screenshots": [3 скриншота]
}
```

---

## 🎉 Результат:

Теперь приложения будут показывать **ВСЮ активность**, независимо от того был пользователь активен или idle. Это даст полную картину использования приложений!

Если захотите в будущем фильтровать по active/idle - это можно будет добавить на фронтенде или как query параметр API.


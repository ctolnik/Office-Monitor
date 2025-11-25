# 🔍 STATE DEBUGGING версия - Проверка состояний сегментов

**Обновлено:** 26 ноября 2025 02:55 MSK  
**Проблема:** `applications: 0` при наличии `activity_events: 31`

---

## 🎯 ДИАГНОЗ:

Из логов видно:
```json
{"activity_events_count":31,"applications_count":0}
```

**Причина:** Функция `GetApplicationUsageFromSegments` фильтрует только `state = 'active'`:

```sql
WHERE username = ? 
  AND timestamp_start >= ...
  AND state = 'active'    ← ВОТ ПРОБЛЕМА!
GROUP BY process_name, window_title
```

Если все 31 сегмента имеют `state != 'active'` (например, 'idle' или 'offline'), они не попадут в applications!

---

## ✅ Что добавлено в новую версию:

### 1. **Подсчёт состояний** в GetActivitySegmentsByUsername
```json
{
  "msg": "GetActivitySegmentsByUsername result",
  "segments_count": 31,
  "states": {"active": 5, "idle": 20, "offline": 6}  ← НОВОЕ!
}
```

### 2. **SQL query** в GetApplicationUsageFromSegments
```json
{
  "msg": "GetApplicationUsageFromSegments",
  "query": "SELECT ... WHERE ... AND state = 'active' ..."  ← Видим полный запрос
}
```

---

## 🚀 Deployment:

```bash
# 1. Скопировать STATE DEBUG версию
scp server/server user@monitor.net.gslaudit.ru:/opt/monitoring/

# 2. Перезапустить
ssh user@monitor.net.gslaudit.ru
sudo systemctl stop monitoring-server
sudo cp /opt/monitoring/server /usr/local/bin/monitoring-server
sudo systemctl start monitoring-server

# 3. Открыть отчёт
# http://monitor.net.gslaudit.ru/reports/daily?username=a-kiv&date=2025-11-26

# 4. Смотреть логи
docker logs monitoring-server --tail 50 | grep -E "(states|GetApplication)"
```

---

## 📊 Что искать в логах:

### ✅ Если увидите:
```json
{"segments_count": 31, "states": {"idle": 31}}
```

**Решение:** Убрать фильтр `AND state = 'active'` из GetApplicationUsageFromSegments

### ✅ Если увидите:
```json
{"segments_count": 31, "states": {"active": 31}}
```

**Значит проблема в другом месте** (возможно в GROUP BY или агрегации)

### ✅ Если увидите:
```json
{"segments_count": 31, "states": {"active": 5, "idle": 26}}
```

**Решение:** Нужно либо:
1. Убрать фильтр по state (показывать все приложения независимо от active/idle)
2. Или изменить агента чтобы он отправлял больше 'active' сегментов

---

## 🔧 Быстрое исправление (если подтвердится):

Если в логах `states: {"idle": ...}` или `states: {"offline": ...}`, значит проблема подтверждена.

**Исправление в коде (строка 94 в activity_segments.go):**

```go
// БЫЛО:
AND state = 'active'

// СТАЛО (вариант 1 - показывать всё):
// Убрать эту строку вообще

// СТАЛО (вариант 2 - показывать active + idle):
AND state IN ('active', 'idle')
```

---

## 📋 Что мне нужно от вас:

Пришлите строку из логов с `"states"`:

```json
{"msg":"GetActivitySegmentsByUsername result","segments_count":31,"states":{...}}
```

По этому я точно скажу как исправить!


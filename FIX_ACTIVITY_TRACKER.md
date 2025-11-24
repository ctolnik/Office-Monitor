# 🔧 Исправление: Activity Tracker не работает

## 🔴 Проблема:

Агент НЕ отправляет данные на `/api/activity/segment`

## 🎯 Причина:

В `agent/main.go` строки 75-95:

```go
if cfg.ActivityMonitoring.Enabled {
    activityTracker = monitoring.NewActivityTracker(...)
    log.Printf("Activity tracking: ENABLED...")
} else {
    log.Println("Activity tracking: DISABLED")
}
```

**В логах агента НЕ ВИДНО:**
- ❌ "Activity tracking: ENABLED" 
- ❌ "Activity tracking: DISABLED"

**Значит:** `ActivityMonitoring.Enabled = false` в конфиге!

---

## ✅ Решение:

### 1. Проверьте config.yaml агента

Найдите секцию `activity_monitoring`:

```yaml
activity_monitoring:
  enabled: false  # ❌ ВОТ ОНА ПРОБЛЕМА!
  idle_threshold_seconds: 300
  interval_seconds: 30
```

### 2. Включите activity tracking:

```yaml
activity_monitoring:
  enabled: true   # ✅ ИСПРАВИТЬ на true!
  idle_threshold_seconds: 300  # 5 минут до idle
  interval_seconds: 30         # проверка каждые 30 секунд
```

### 3. Перезапустите агент

После изменения config.yaml:

```bash
# Остановите агент (Ctrl+C)
# Запустите заново
agent.exe
```

### 4. Проверьте логи

Должны увидеть:

```
Activity tracking: ENABLED (idle threshold: 5m, poll interval: 30s)
```

И через 30-60 секунд:

```
POST /api/activity/segment succeeded (200)
```

---

## 📊 Проверка после исправления:

```bash
# На production сервере
docker exec clickhouse clickhouse-client --database=monitoring --query="
SELECT 
    count() as segments,
    state,
    sum(duration_sec) as total_seconds
FROM activity_segments
WHERE timestamp_start > now() - INTERVAL 1 HOUR
GROUP BY state
"
```

Должны увидеть данные: active, idle, offline сегменты!

---

## 🎯 Итог:

1. ✅ Миграции применены (таблица activity_segments создана)
2. ✅ Агент работает без ошибок (POST /api/events/batch ok)
3. ❌ Activity tracker выключен в config.yaml
4. 🔧 Решение: `enabled: true` в секции activity_monitoring


# 🚨 Быстрое исправление после ошибки миграции

## Что произошло:

Вы применили **migrations.sql** вместо **add_activity_segments.sql**

migrations.sql содержит таблицы для frontend (которые ещё не нужны), 
а вам нужна только таблица activity_segments для агента.

---

## ✅ Правильное решение:

### 1. Примените ПРАВИЛЬНУЮ миграцию:

```bash
# На production сервере
cd /opt/monitoring/clickhouse
docker exec -i clickhouse clickhouse-client \
    --database=monitoring < add_activity_segments.sql
```

### 2. Проверьте что таблица создана:

```bash
docker exec clickhouse clickhouse-client \
    --database=monitoring \
    --query="SHOW TABLES LIKE 'activity_segments'"
```

Должны увидеть:
```
activity_segments
```

### 3. Проверьте materialized views:

```bash
docker exec clickhouse clickhouse-client \
    --database=monitoring \
    --query="SHOW TABLES LIKE '%activity%'"
```

Должны увидеть:
```
activity_events
activity_segments
activity_stats_hourly
daily_activity_summary
program_usage_daily
```

---

## 🔍 Если ошибка "table already exists":

Это ХОРОШО! Значит таблица уже создана из migrations.sql.

Проверьте что она работает:

```bash
docker exec clickhouse clickhouse-client \
    --database=monitoring \
    --query="DESC activity_segments"
```

Должны увидеть структуру таблицы с полями:
- timestamp_start
- timestamp_end  
- duration_sec
- state
- computer_name
- username
- process_name
- window_title
- session_id
- event_date

---

## 🎯 Итоговая проверка:

```bash
# Перезапустить агент на Windows машине
# Посмотреть логи агента через 30 секунд

# Должны увидеть:
# ✅ POST /api/activity/segment succeeded (200)
# ❌ Больше НЕТ: Server returned non-OK status for activity segment: 500
```

---

## 📝 Файлы миграций:

- `add_activity_segments.sql` ← ЭТОТ нужен для агента
- `migrations.sql` ← Этот для frontend (можно игнорировать сейчас)
- `init.sql` ← Полная инициализация (для новых установок)

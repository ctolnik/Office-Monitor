# Исправление ошибок агента

## ✅ ЧТО ИСПРАВЛЕНО

### 1. Ошибка 400 Bad Request - ИСПРАВЛЕНА ✅

**Проблема:** Агент отправляет `{"events": [...]}`, сервер ожидал прямой массив.

**Решение:** Переписан `receiveBatchEventsHandler` в `server/main.go`:
- Принимает формат `{"events": [...]}`
- Парсит типизированные события (`type`, `timestamp`, `data`)
- Извлекает данные из вложенного поля `data`
- Фильтрует события по типу `"activity"`

**Код:** `server/main.go:202-302`

---

## ❌ ОСТАЛАСЬ ОШИБКА 500: Activity Segment

### Симптомы
```
Server returned non-OK status for activity segment: 500
```

### Возможные причины

1. **Таблица не создана**
   - В production ClickHouse отсутствует `monitoring.activity_segments`
   - Миграции не выполнены

2. **Схема не совпадает**
   - INSERT ожидает: `timestamp_start, timestamp_end, duration_sec, state, computer_name, username, process_name, window_title, session_id`
   - Таблица имеет другие колонки

3. **ClickHouse недоступен**
   - Контейнер не запущен
   - Нет подключения

---

## 🔍 ДИАГНОСТИКА

### 1. Проверить логи сервера
```bash
docker-compose logs server | grep "Failed to insert activity segment"
```

Вы увидите точную ошибку от ClickHouse.

### 2. Проверить таблицы
```bash
docker-compose exec clickhouse clickhouse-client -q "SHOW TABLES FROM monitoring"
```

Должна быть таблица: `activity_segments`

### 3. Проверить схему
```bash
docker-compose exec clickhouse clickhouse-client -q "DESCRIBE monitoring.activity_segments"
```

Ожидаемые колонки:
- `timestamp_start` DateTime
- `timestamp_end` DateTime
- `duration_sec` UInt32
- `state` String (или Enum)
- `computer_name` String
- `username` String
- `process_name` String
- `window_title` String
- `session_id` String

### 4. Создать таблицу (если отсутствует)

Если таблицы нет, создайте её:

```sql
CREATE TABLE IF NOT EXISTS monitoring.activity_segments (
    timestamp_start DateTime,
    timestamp_end DateTime,
    duration_sec UInt32,
    state Enum8('active'=1, 'idle'=2, 'offline'=3),
    computer_name String,
    username String,
    process_name String,
    window_title String,
    session_id String
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp_start)
ORDER BY (computer_name, username, timestamp_start);
```

Выполните:
```bash
docker-compose exec clickhouse clickhouse-client -q "
CREATE TABLE IF NOT EXISTS monitoring.activity_segments (
    timestamp_start DateTime,
    timestamp_end DateTime,
    duration_sec UInt32,
    state Enum8('active'=1, 'idle'=2, 'offline'=3),
    computer_name String,
    username String,
    process_name String,
    window_title String,
    session_id String
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp_start)
ORDER BY (computer_name, username, timestamp_start);
"
```

---

## 🚀 ПЛАН ДЕЙСТВИЙ

1. **Обновите production сервер:**
   ```bash
   cd /path/to/Office-Monitor
   git pull origin main
   docker-compose build server
   docker-compose up -d
   ```

2. **Проверьте логи:**
   ```bash
   docker-compose logs server | tail -50
   ```

3. **Проверьте ClickHouse:**
   ```bash
   docker-compose exec clickhouse clickhouse-client -q "SHOW TABLES FROM monitoring"
   ```

4. **Создайте таблицу (если нужно)** - см. выше

5. **Перезапустите агент** - он автоматически повторит отправку

---

## ✅ ПОСЛЕ ИСПРАВЛЕНИЯ

Агент должен работать без ошибок:
- ✅ `POST /api/screenshot` - 200 OK
- ✅ Keyboard logging - отправляется
- ✅ `POST /api/events/batch` - 200 OK (ИСПРАВЛЕНО)
- ✅ `POST /api/activity/segment` - 200 OK (после создания таблицы)

Timeline и графики заполнятся данными!

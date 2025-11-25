# 🔍 DEBUG версия сервера - инструкции по deployment

**Цель:** Выяснить почему отчёты пустые, несмотря на правильный URL и данные в БД

---

## ✅ Что добавлено в код:

### 1. **GetActivitySegmentsByUsername** - полный debug
```
INFO: GetActivitySegmentsByUsername
  - username: какой username запрашивается
  - start: начало диапазона (2025-11-26 00:00:00)
  - end: конец диапазона (2025-11-27 00:00:00)
  - query: полный SQL запрос

INFO: GetActivitySegmentsByUsername result
  - segments_count: сколько записей вернулось
```

### 2. **GetApplicationUsageFromSegments** - полный debug
```
INFO: GetApplicationUsageFromSegments
  - username, start, end

INFO: GetApplicationUsageFromSegments result
  - apps_count: сколько приложений найдено
```

### 3. **GetDailyReport** - входные и выходные данные
```
INFO: GetDailyReport called
  - username: запрашиваемый пользователь
  - date: запрашиваемая дата
  - startOfDay: начало дня с timezone
  - endOfDay: конец дня с timezone
  - timezone: какой timezone использует код (UTC? MSK?)

INFO: GetDailyReport completed
  - activity_events_count: сколько событий в отчёте
  - applications_count: сколько приложений в отчёте
  - screenshots_count, usb_events_count, file_events_count
```

---

## 🚀 Deployment инструкции:

### Шаг 1: Обновить server binary на production

```bash
# На вашем локальном компьютере
scp server/server user@monitor.net.gslaudit.ru:/opt/monitoring/

# На production сервере
ssh user@monitor.net.gslaudit.ru
sudo systemctl stop monitoring-server
sudo cp /opt/monitoring/server /usr/local/bin/monitoring-server
sudo systemctl start monitoring-server
sudo systemctl status monitoring-server
```

### Шаг 2: Открыть отчёт в браузере

Просто откройте отчёт как обычно:
```
http://monitor.net.gslaudit.ru/reports/daily?username=a-kiv&date=2025-11-26
```

### Шаг 3: Посмотреть логи сервера

```bash
# На production сервере
docker logs monitoring-server --tail 100 -f
```

**Или если сервер запущен через systemd:**
```bash
sudo journalctl -u monitoring-server -f --lines 100
```

---

## 📊 Что искать в логах:

### ✅ Если всё работает правильно:

```json
{
  "level": "info",
  "msg": "GetDailyReport called",
  "username": "a-kiv",
  "date": "2025-11-26",
  "startOfDay": "2025-11-26 00:00:00 MSK",
  "endOfDay": "2025-11-27 00:00:00 MSK",
  "timezone": "Europe/Moscow"
}

{
  "level": "info",
  "msg": "GetActivitySegmentsByUsername",
  "username": "a-kiv",
  "start": "2025-11-26 00:00:00",
  "end": "2025-11-27 00:00:00",
  "query": "SELECT ... FROM monitoring.activity_segments WHERE username = ? AND ..."
}

{
  "level": "info",
  "msg": "GetActivitySegmentsByUsername result",
  "username": "a-kiv",
  "segments_count": 150    ← ДОЛЖНО БЫТЬ > 0!
}

{
  "level": "info",
  "msg": "GetApplicationUsageFromSegments result",
  "username": "a-kiv",
  "apps_count": 10         ← ДОЛЖНО БЫТЬ > 0!
}

{
  "level": "info",
  "msg": "GetDailyReport completed",
  "username": "a-kiv",
  "activity_events_count": 150,   ← ДОЛЖНО БЫТЬ > 0!
  "applications_count": 10,        ← ДОЛЖНО БЫТЬ > 0!
  "screenshots_count": 0,
  "usb_events_count": 0,
  "file_events_count": 0
}
```

---

### ❌ Если segments_count = 0:

```json
{
  "level": "info",
  "msg": "GetActivitySegmentsByUsername result",
  "username": "a-kiv",
  "segments_count": 0      ← ПРОБЛЕМА!
}
```

**Возможные причины:**

#### 1. **Timezone mismatch** (самая вероятная!)
```
startOfDay: "2025-11-26 00:00:00 UTC"  ← Неправильно! Должно быть MSK
```

**Решение:** Исправить timezone в коде парсинга даты

#### 2. **Неправильный формат даты в SQL**
Посмотрите на `query` в логах - должно быть:
```sql
WHERE username = 'a-kiv' 
  AND timestamp_start >= toDateTime64('2025-11-26 00:00:00', 3)
  AND timestamp_start < toDateTime64('2025-11-27 00:00:00', 3)
```

#### 3. **Данных нет за эту дату**
Проверьте вручную:
```bash
docker exec monitoring-clickhouse clickhouse-client --database=monitoring \
  --query="SELECT count(*) FROM activity_segments 
           WHERE username='a-kiv' 
           AND toDate(timestamp_start)='2025-11-26'"
```

Если вернёт 0 - данных действительно нет!

---

### ❌ Если apps_count = 0 (но segments_count > 0):

```json
{
  "level": "info",
  "msg": "GetActivitySegmentsByUsername result",
  "segments_count": 150    ← Есть данные!
}

{
  "level": "info",
  "msg": "GetApplicationUsageFromSegments result",
  "apps_count": 0          ← Но приложений нет!
}
```

**Причина:** Все segments имеют `state != 'active'` (idle или offline)

**Решение:** Убрать фильтр `AND state = 'active'` из GetApplicationUsageFromSegments

---

## 🎯 Быстрая диагностика:

После deployment, выполните:

```bash
# 1. Открыть отчёт в браузере
# 2. Сразу же смотреть логи:
docker logs monitoring-server --tail 50

# 3. Найти строки с GetDailyReport и скопировать сюда
```

Пришлите мне логи - я точно скажу в чём проблема!

---

## 🔧 Если нужно проверить SQL запрос вручную:

Скопируйте `query` из логов и выполните вручную:

```bash
# Пример из логов:
# query: "SELECT ... WHERE username = ? AND timestamp_start >= toDateTime64('2025-11-26 00:00:00', 3) ..."

docker exec monitoring-clickhouse clickhouse-client --database=monitoring \
  --query="SELECT count(*) FROM monitoring.activity_segments 
           WHERE username = 'a-kiv' 
           AND timestamp_start >= toDateTime64('2025-11-26 00:00:00', 3)
           AND timestamp_start < toDateTime64('2025-11-27 00:00:00', 3)"
```

Если вернёт > 0 - проблема в коде!  
Если вернёт 0 - проблема в данных или timezone!

---

## 📝 Checklist после deployment:

- [ ] Скопировал новый server binary
- [ ] Перезапустил сервер (`systemctl restart monitoring-server`)
- [ ] Открыл отчёт в браузере
- [ ] Посмотрел логи (`docker logs monitoring-server`)
- [ ] Нашёл строки с "GetDailyReport called"
- [ ] Проверил `segments_count` и `apps_count`
- [ ] Скопировал логи и отправил

🎉 **После этого мы точно найдём проблему!**


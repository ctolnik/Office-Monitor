# 🔍 ENHANCED DEBUG версия - Полная трассировка API запроса

**Обновлено:** 26 ноября 2025  
**Проблема:** Отчёты показывают (0) везде, но данные есть в БД, URL правильный, timezone не проблема

---

## ✅ Что добавлено в новую версию:

### 🔵 Handler Level (HTTP запрос → ответ)
```json
{
  "msg": "🔵 getDailyReportHandler called",
  "username": "a-kiv",
  "date_param": "2025-11-26",
  "request_url": "/api/reports/daily/a-kiv?date=2025-11-26"
}

{
  "msg": "🔵 Parsed date",
  "parsed_date": "2025-11-26 00:00:00 MSK",
  "timezone": "Europe/Moscow"
}

{
  "msg": "🔵 getDailyReportHandler - report retrieved",
  "activity_events": 150,      ← Сколько событий ПЕРЕД отправкой JSON
  "applications": 10,          ← Сколько приложений ПЕРЕД отправкой JSON
  "screenshots": 0,
  ...
}

{
  "msg": "🔵 First activity event",
  "process": "chrome.exe",
  "window": "GitHub - ...",
  "duration": 120
}

{
  "msg": "🔵 getDailyReportHandler - JSON response sent"
}
```

### Database Level (запросы к ClickHouse)
```json
{
  "msg": "GetDailyReport called",
  "username": "a-kiv",
  "startOfDay": "2025-11-26 00:00:00 MSK",
  "endOfDay": "2025-11-27 00:00:00 MSK"
}

{
  "msg": "GetActivitySegmentsByUsername",
  "username": "a-kiv",
  "start": "2025-11-26 00:00:00",
  "end": "2025-11-27 00:00:00",
  "query": "SELECT ... WHERE username = ? AND timestamp_start >= ..."
}

{
  "msg": "GetActivitySegmentsByUsername result",
  "segments_count": 150        ← Сколько записей вернул ClickHouse
}

{
  "msg": "GetApplicationUsageFromSegments result",
  "apps_count": 10
}

{
  "msg": "GetDailyReport completed",
  "activity_events_count": 150,
  "applications_count": 10
}
```

---

## 🎯 Сценарии диагностики:

### Сценарий 1️⃣: Данные теряются в БД слое

**Признак:**
```json
{"msg": "GetActivitySegmentsByUsername result", "segments_count": 0}
{"msg": "GetDailyReport completed", "activity_events_count": 0}
{"msg": "🔵 getDailyReportHandler - report retrieved", "activity_events": 0}
```

**Причины:**
- SQL запрос сформирован неправильно
- Timezone mismatch (хотя вы сказали это исключено)
- Данных нет за эту дату/username

**Решение:** Скопировать `query` из логов и выполнить вручную в ClickHouse

---

### Сценарий 2️⃣: Данные есть в БД, но теряются при конвертации

**Признак:**
```json
{"msg": "GetActivitySegmentsByUsername result", "segments_count": 150}  ← Данные есть!
{"msg": "GetDailyReport completed", "activity_events_count": 0}         ← Но после конвертации пропали!
```

**Причина:** Ошибка в цикле конвертации segments → activityEvents (строки 804-813)

**Решение:** Проверить код конвертации в GetDailyReport

---

### Сценарий 3️⃣: Данные есть до отправки, но не доходят до фронтенда

**Признак:**
```json
{"msg": "GetDailyReport completed", "activity_events_count": 150}       ← Данные есть!
{"msg": "🔵 getDailyReportHandler - report retrieved", "activity_events": 150}  ← Перед отправкой есть!
{"msg": "🔵 First activity event", "process": "chrome.exe", ...}        ← Даже видим первый элемент!
{"msg": "🔵 getDailyReportHandler - JSON response sent"}                ← Отправили!
```

НО фронтенд показывает (0)!

**Причина:** Проблема НА ФРОНТЕНДЕ (lovable.dev)
- Неправильная обработка JSON response
- Неправильные поля в API response (frontend ожидает другие названия)
- CORS проблема (запрос проходит, но данные не доступны)

**Решение:** 
1. Открыть DevTools → Network → посмотреть Response для `/api/reports/daily/...`
2. Проверить что JSON содержит данные
3. Проверить Console на ошибки парсинга
4. Исправить фронтенд код на lovable.dev

---

### Сценарий 4️⃣: JSON Marshal проблема

**Признак:**
```json
{"msg": "🔵 getDailyReportHandler - report retrieved", "activity_events": 150}
```

НО сразу после ошибка или JSON response пустой.

**Причина:** Gin не может сериализовать структуру в JSON (например, циклические ссылки, неэкспортируемые поля)

**Решение:** Проверить определение структуры DailyReport в models.go

---

## 🚀 Deployment:

```bash
# 1. Скопировать ENHANCED DEBUG версию
scp server/server user@monitor.net.gslaudit.ru:/opt/monitoring/server-debug

# 2. Остановить старую версию
ssh user@monitor.net.gslaudit.ru
sudo systemctl stop monitoring-server

# 3. Заменить binary
sudo cp /opt/monitoring/server-debug /usr/local/bin/monitoring-server

# 4. Запустить
sudo systemctl start monitoring-server

# 5. Открыть отчёт в браузере
# http://monitor.net.gslaudit.ru/reports/daily?username=a-kiv&date=2025-11-26

# 6. СРАЗУ смотреть логи
docker logs monitoring-server --tail 100 -f
# ИЛИ
sudo journalctl -u monitoring-server -f --lines 100
```

---

## 📊 Что мне нужно увидеть:

Скопируйте **все логи с emoji 🔵** после открытия отчёта:

```
🔵 getDailyReportHandler called
🔵 Parsed date
GetDailyReport called (без emoji - из БД слоя)
GetActivitySegmentsByUsername
GetActivitySegmentsByUsername result
GetApplicationUsageFromSegments
GetApplicationUsageFromSegments result
GetDailyReport completed
🔵 getDailyReportHandler - report retrieved
🔵 First activity event (если есть)
🔵 First application (если есть)
🔵 getDailyReportHandler - JSON response sent
```

---

## 🔧 Дополнительная проверка - DevTools:

1. Откройте страницу отчётов
2. F12 → Network tab
3. Обновите страницу
4. Найдите запрос `/api/reports/daily/a-kiv?date=2025-11-26`
5. **Response tab** → скопируйте ПЕРВЫЕ 100 строк JSON

Если там:
```json
{
  "username": "a-kiv",
  "date": "2025-11-26",
  "activity_events": [],    ← ПУСТОЙ!
  "applications": [],        ← ПУСТОЙ!
  ...
}
```

Значит проблема в backend!

Если там:
```json
{
  "username": "a-kiv",
  "date": "2025-11-26",
  "activity_events": [
    {"timestamp": "...", "process_name": "chrome.exe", ...},
    ...
  ],
  "applications": [...]
}
```

Значит проблема во frontend (lovable.dev)!

---

## 🎯 После получения логов я скажу:

✅ **Точную строку кода** где теряются данные  
✅ **Точную причину** проблемы  
✅ **Точное исправление** которое нужно сделать


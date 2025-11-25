# 🔍 Чеклист для проверки пустых отчётов на production

**Проблема:** Отчёты показывают (0) везде, но ошибок в логах нет

---

## ✅ Шаг 1: Проверить что миграции применены

```bash
# Подключиться к production серверу
ssh user@monitor.net.gslaudit.ru

# Проверить что таблица application_categories существует
docker exec monitoring-clickhouse clickhouse-client --database=monitoring \
  --query="SELECT count(*) FROM application_categories"

# Должно вернуть: 14 (или больше если добавляли категории)
# Если ошибка "Unknown table" - применить миграции из MIGRATION_SETUP_INSTRUCTIONS.md
```

---

## ✅ Шаг 2: Проверить что данные есть в activity_segments

```bash
# Посмотреть какие username есть в базе
docker exec monitoring-clickhouse clickhouse-client --database=monitoring \
  --query="SELECT DISTINCT username FROM activity_segments ORDER BY username"

# Должны увидеть: a-kiv (или другие usernames)

# Проверить количество записей за сегодня
docker exec monitoring-clickhouse clickhouse-client --database=monitoring \
  --query="SELECT 
    username, 
    count(*) as records,
    min(timestamp_start) as first_event,
    max(timestamp_end) as last_event
FROM activity_segments 
WHERE toDate(timestamp_start) = today()
GROUP BY username
FORMAT Pretty"

# Если пусто - значит агенты не отправляют данные!
```

---

## ✅ Шаг 3: Проверить что API возвращает данные

```bash
# На production сервере, проверить API для конкретного username
curl -s "http://localhost:8081/api/reports/daily/a-kiv?date=$(date +%Y-%m-%d)" | \
  python3 -m json.tool | head -50

# Должны увидеть:
# {
#   "username": "a-kiv",
#   "date": "2025-11-26",
#   "activity_events": [...],  # НЕ пустой массив
#   "applications": [...],      # НЕ пустой массив
#   ...
# }

# Если массивы пустые [] - проблема в GetDailyReport
# Если ошибка - проблема в API endpoint
```

---

## ✅ Шаг 4: Проверить username который использует frontend

**ВАЖНО:** Frontend может запрашивать неправильный username!

Откройте DevTools в браузере (F12) и:

1. **Network tab** → Обновите страницу отчётов
2. Найдите запрос к `/api/reports/daily/...`
3. Посмотрите какой **username** используется в URL

**Возможные проблемы:**
- Frontend запрашивает `a.kly` вместо `a-kiv`
- Frontend запрашивает `undefined` или пустой username
- Неправильная дата в запросе

---

## ✅ Шаг 5: Проверить логи сервера

```bash
# Посмотреть последние запросы к API
docker logs monitoring-server --tail 100 | grep "GET /api/reports/daily"

# Должны увидеть:
# GET /api/reports/daily/a-kiv?date=2025-11-26 200 OK
# или
# GET /api/reports/daily/a.kly?date=2025-11-26 200 OK (неправильный username!)

# Проверить ошибки (если они есть)
docker logs monitoring-server --tail 200 | grep -i error
```

---

## ✅ Шаг 6: Проверить dashboard stats

```bash
# Проверить что dashboard stats работает
curl -s "http://localhost:8081/api/dashboard/stats" | python3 -m json.tool

# Должно вернуть:
# {
#   "total_employees": 5,
#   "active_now": 2,
#   "idle": 1,
#   "offline": 2,
#   ...
# }

# Если total_employees = 0 - значит нет данных в employees таблице
```

---

## 🎯 Частые причины пустых отчётов:

### 1. **Username mismatch** (самая частая!)
- ✅ Agent отправляет: `a-kiv`
- ❌ Frontend запрашивает: `a.kly`
- **Решение:** Исправить frontend на lovable.dev

### 2. **Миграции не применены**
- ❌ Таблица `application_categories` не существует
- **Решение:** Применить миграции вручную (MIGRATION_SETUP_INSTRUCTIONS.md)

### 3. **Нет данных в activity_segments**
- ❌ Агенты не отправляют данные
- ❌ Агенты отправляют, но данные не записываются
- **Решение:** Проверить логи агента, проверить `/api/events/batch` endpoint

### 4. **GetDailyReport использует старую таблицу**
- ❌ Код всё ещё запрашивает `activity_events` вместо `activity_segments`
- **Решение:** Обновить server binary (server/server) на production

### 5. **Неправильная дата**
- ❌ Frontend запрашивает будущую дату
- ❌ Timezone mismatch (UTC vs MSK)
- **Решение:** Проверить параметр `date` в Network tab

---

## 🚀 Быстрая диагностика (30 секунд):

```bash
# На production сервере, одна команда для всех проверок:
echo "=== 1. Миграции ==="
docker exec monitoring-clickhouse clickhouse-client --database=monitoring \
  --query="SELECT count(*) as categories_count FROM application_categories"

echo -e "\n=== 2. Usernames в БД ==="
docker exec monitoring-clickhouse clickhouse-client --database=monitoring \
  --query="SELECT DISTINCT username FROM activity_segments LIMIT 10"

echo -e "\n=== 3. Данные за сегодня ==="
docker exec monitoring-clickhouse clickhouse-client --database=monitoring \
  --query="SELECT username, count(*) FROM activity_segments WHERE toDate(timestamp_start)=today() GROUP BY username"

echo -e "\n=== 4. Dashboard Stats ==="
curl -s http://localhost:8081/api/dashboard/stats | python3 -m json.tool

echo -e "\n=== 5. API для a-kiv ==="
curl -s "http://localhost:8081/api/reports/daily/a-kiv?date=$(date +%Y-%m-%d)" | \
  python3 -c "import sys,json; d=json.load(sys.stdin); print(f'Events: {len(d.get(\"activity_events\",[]))}, Apps: {len(d.get(\"applications\",[]))}')"
```

Скопируйте весь блок и запустите - получите полную диагностику!


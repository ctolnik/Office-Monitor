# 🐛 Отладка пустого отчёта

## Проблема:
Frontend показывает пустой отчёт (00:00:00 активное время, 0% продуктивность)
Агент работает без ошибок в логах

## Возможные причины:

### 1️⃣ Данные не попадают в ClickHouse
**Проверка:** Запустить на production сервере:
```bash
docker exec clickhouse clickhouse-client --database=monitoring --query="
SELECT count() FROM activity_events WHERE timestamp > now() - INTERVAL 1 HOUR
"
```

**Если результат 0:**
- Агент отправляет, но сервер не сохраняет в БД
- Проверить логи сервера: `journalctl -u monitoring-server -n 100`

**Если результат > 0:**
- Данные есть, проблема в API или frontend

---

### 2️⃣ Username не совпадает
**Проверка:**
```bash
docker exec clickhouse clickhouse-client --database=monitoring --query="
SELECT DISTINCT username FROM activity_events WHERE timestamp > now() - INTERVAL 24 HOUR
"
```

Сравните username из БД с username в URL отчёта!

**Типичная проблема:**
- В БД: `Administrator` или `DESKTOP-ABC\john`
- В URL frontend: `a.kly` (неправильный username)

---

### 3️⃣ API не возвращает данные
**Проверка:** Прямой запрос к API:
```bash
# На production сервере
curl -s "http://localhost:5000/api/reports/daily/USERNAME?date=2025-11-25" | jq .

# Замените USERNAME на реальный username из шага 2
```

**Ожидаемый результат:** JSON с массивами events, applications, screenshots и т.д.

**Если пустые массивы:**
- Проблема в SQL запросах GetDailyReport
- Проверить логи сервера на ошибки

---

### 4️⃣ Timezone проблема
**Возможная причина:** 
- Сервер в UTC, агент в Europe/Moscow
- Данные есть, но за "другой день"

**Проверка:**
```bash
docker exec clickhouse clickhouse-client --database=monitoring --query="
SELECT 
    toDate(timestamp) as date,
    count() as events
FROM activity_events
WHERE timestamp > now() - INTERVAL 7 DAY
GROUP BY date
ORDER BY date DESC
"
```

Посмотрите на какие даты попадают события!

---

## 🔍 Диагностический скрипт

Создан файл `check_data.sh` - скопируйте на production и запустите:

```bash
scp check_data.sh user@monitor.net.gslaudit.ru:/opt/monitoring/
ssh user@monitor.net.gslaudit.ru
cd /opt/monitoring
./check_data.sh
```

Покажет:
- Сколько событий в каждой таблице за последний час
- Список всех username с данными
- Последние 5 событий

---

## 🎯 Следующие шаги:

1. Запустить `check_data.sh` на production
2. Проверить реальный username в БД
3. Проверить что frontend использует правильный username
4. Проверить API запрос curl'ом
5. Прислать результаты для анализа

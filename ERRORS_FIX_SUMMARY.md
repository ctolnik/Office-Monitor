# ✅ Исправление ошибок сервера

**Дата:** 26 ноября 2025  
**Проблемы в логах:**
1. `converting UInt64 to *int is unsupported`
2. `Unknown table expression identifier 'monitoring.application_categories'`

---

## 🔴 Проблема 1: Ошибка типов (UInt64 → int)

### Ошибка в логах:
```
Failed to get total employees, error: clickhouse [ScanRow]: (countDistinct(username)) converting UInt64 to *int is unsupported
Failed to get active now, error: clickhouse [ScanRow]: (countDistinct(username)) converting UInt64 to *int is unsupported
```

### Причина:
- ClickHouse возвращает `UInt64` из `count(DISTINCT username)`
- Код пытался записать в поля типа `int`

### Решение:
✅ Изменены типы в `DashboardStats`:
```go
// БЫЛО:
TotalEmployees    int     `json:"total_employees"`
ActiveNow         int     `json:"active_now"`
Offline           int     `json:"offline"`

// СТАЛО:
TotalEmployees    uint64  `json:"total_employees"`
ActiveNow         uint64  `json:"active_now"`
Offline           uint64  `json:"offline"`
```

---

## 🔴 Проблема 2: Запросы к пустой таблице

### Причина:
- Запросы к `activity_events` (пустая таблица)
- Данные находятся в `activity_segments`

### Решение:
✅ Изменены SQL запросы в `GetDashboardStats`:
```go
// БЫЛО:
FROM monitoring.activity_events 
WHERE timestamp > ?

// СТАЛО:
FROM monitoring.activity_segments 
WHERE timestamp_start > ?
```

---

## 🔴 Проблема 3: Отсутствует таблица application_categories

### Ошибка в логах:
```
Unknown table expression identifier 'monitoring.application_categories'
```

### Решение:
✅ Создан SQL файл: `clickhouse/add_application_categories_table.sql`

Таблица для классификации приложений (productive/neutral/unproductive) с 14 предзаполненными категориями.

---

## 📝 Изменённые файлы:

1. **server/database/models.go** - исправлены типы DashboardStats
2. **server/database/frontend_queries.go** - запросы к activity_segments
3. **clickhouse/add_application_categories_table.sql** (НОВЫЙ) - создание таблицы

---

## 🚀 Deployment на production:

### Шаг 1: Применить SQL миграцию
```bash
# Подключиться к ClickHouse на production
clickhouse-client --host 172.16.0.2 --database monitoring --multiquery < clickhouse/add_application_categories_table.sql

# Проверить что таблица создана
clickhouse-client --host 172.16.0.2 --database monitoring --query "SELECT count(*) FROM monitoring.application_categories"
```

**Ожидаемый результат:** `14` (предзагруженные категории)

### Шаг 2: Задеплоить новый server binary
```bash
# Скопировать новый server
scp server/server user@monitor.net.gslaudit.ru:/opt/monitoring/

# Установить и перезапустить
ssh user@monitor.net.gslaudit.ru
sudo systemctl stop monitoring-server
sudo cp /opt/monitoring/server /usr/local/bin/monitoring-server
sudo systemctl start monitoring-server
sudo systemctl status monitoring-server
```

### Шаг 3: Проверить логи
```bash
# Открыть дашборд и проверить что ошибки исчезли
docker logs -f monitoring-server --tail 50

# Должны исчезнуть:
# ❌ "converting UInt64 to *int"
# ❌ "Unknown table expression identifier 'monitoring.application_categories'"
```

---

## ✅ Ожидаемый результат:

**После деплоя в логах:**
- ✅ Нет ошибок "converting UInt64 to *int"
- ✅ Нет ошибок "Unknown table...application_categories"
- ✅ Dashboard stats работает корректно
- ✅ API /api/reports/daily возвращает данные

---

## 🎯 Итоговые изменения:

✅ **Типы данных:** int → uint64 для счётчиков  
✅ **Источник данных:** activity_events → activity_segments  
✅ **Таблица categories:** Создана с предзаполнением  
✅ **Server скомпилирован:** 43MB, готов к деплою


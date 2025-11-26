# ClickHouse Миграции

## 📋 Структура

Все миграции находятся в директории `clickhouse/` и автоматически применяются при запуске контейнера через `docker-entrypoint-initdb.d/`.

### Файлы миграций:

1. **01-schema.sql** - Схема базы данных
   - Создаёт все таблицы (activity_events, activity_segments, keyboard_events, и т.д.)
   - Создаёт материализованные представления
   - Создаёт индексы
   - **Идемпотентная**: можно запускать многократно безопасно

2. **02-seed-data.sql** - Начальные данные
   - Заполняет application_categories базовыми категориями приложений
   - 80+ предустановленных категорий (IDE, браузеры, офис, игры, и т.д.)
   - **Идемпотентная**: не создаёт дубликаты

---

## 🔧 Применение миграций

### На новой установке (Docker):

```bash
# Миграции применяются автоматически при первом запуске
docker-compose up -d clickhouse

# Проверить логи миграций
docker logs monitoring-clickhouse 2>&1 | grep "migration"
```

### На существующей базе:

```bash
# Применить вручную
cat clickhouse/01-schema.sql | docker exec -i monitoring-clickhouse clickhouse-client --database monitoring
cat clickhouse/02-seed-data.sql | docker exec -i monitoring-clickhouse clickhouse-client --database monitoring
```

### На production сервере:

```bash
# Через docker exec
docker exec -i monitoring-clickhouse clickhouse-client --database monitoring < clickhouse/01-schema.sql
docker exec -i monitoring-clickhouse clickhouse-client --database monitoring < clickhouse/02-seed-data.sql
```

---

## ✅ Проверка после миграции

```bash
# Проверить что все таблицы созданы
docker exec monitoring-clickhouse clickhouse-client --database monitoring \
  -q "SHOW TABLES"

# Должно показать:
# - activity_events
# - activity_segments
# - alerts
# - application_categories
# - agent_configs
# - employees
# - file_copy_events
# - keyboard_events
# - process_catalog
# - screenshot_metadata
# - system_settings
# - usb_events
# + materialized views

# Проверить количество категорий
docker exec monitoring-clickhouse clickhouse-client --database monitoring \
  -q "SELECT count(*) FROM application_categories"

# Должно показать > 80
```

---

## 🐛 Troubleshooting

### Миграции не применились:

```bash
# Проверить логи контейнера
docker logs monitoring-clickhouse 2>&1 | tail -100

# Поиск ошибок
docker logs monitoring-clickhouse 2>&1 | grep -i error
```

### Таблица не создалась:

```bash
# Применить миграцию вручную с выводом ошибок
docker exec -i monitoring-clickhouse clickhouse-client --database monitoring \
  --multiquery < clickhouse/01-schema.sql
```

### Очистка и повторное применение:

```bash
# ВНИМАНИЕ: Удаляет ВСЕ данные!
docker-compose down -v
docker-compose up -d

# Или удалить конкретную таблицу:
docker exec monitoring-clickhouse clickhouse-client --database monitoring \
  -q "DROP TABLE IF EXISTS application_categories"
```

---

## 📝 Логи миграций

Миграции выводят логи в стандартный вывод контейнера:

```bash
# Просмотр логов миграций
docker logs monitoring-clickhouse 2>&1 | grep -E "migration|Creating|completed"

# Пример успешного вывода:
# =========================================
# Starting schema migration...
# =========================================
# Creating activity_events table...
# Creating activity_segments table...
# ...
# Creating indexes...
# =========================================
# Schema migration completed successfully!
# =========================================
```

---

## 🔄 Идемпотентность

Все миграции **идемпотентные** - их можно запускать многократно:

- `CREATE TABLE IF NOT EXISTS` - не падает если таблица существует
- `ALTER TABLE ADD COLUMN IF NOT EXISTS` - не падает если колонка существует
- `CREATE MATERIALIZED VIEW IF NOT EXISTS` - не падает если view существует
- `ADD INDEX IF NOT EXISTS` - не падает если индекс существует

---

## 📚 История миграций

### v1.0 (2025-11-26):
- ✅ Объединены все миграции в 2 файла
- ✅ Добавлено логирование
- ✅ Исправлены опечатки в docker-compose.yml
- ✅ Добавлена идемпотентность
- ✅ Добавлена таблица application_categories
- ✅ Seed data с 80+ категориями приложений

### Устаревшие файлы (можно удалить):
- ❌ init.sql
- ❌ migrations.sql
- ❌ add_application_categories_table.sql
- ❌ add_activity_segments.sql
- ❌ migration_add_activity_fields.sql

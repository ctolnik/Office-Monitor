# ClickHouse Миграции

## Автоматическое применение миграций при старте

Все SQL файлы в директории `clickhouse/` автоматически применяются при старте ClickHouse контейнера через механизм `/docker-entrypoint-initdb.d/`.

### Порядок применения:

1. **01-init.sql** (`init.sql`) - Базовые таблицы
   - activity_events, keyboard_events, file_copy_events
   - usb_events, screenshot_metadata, alerts
   - employees, agent_configs
   - activity_segments, process_catalog
   - Materialized views: daily_activity_summary, program_usage_daily

2. **02-migrations.sql** (`migrations.sql`) - Дополнительные таблицы
   - application_categories (старая версия с 5 категориями)
   - system_settings
   - Индексы для оптимизации

3. **03-categories.sql** (`add_application_categories_table.sql`) - Обновление categories
   - Пересоздание application_categories с 3 категориями (productive/neutral/unproductive)
   - Предзаполнение 14 приложениями

### Важно:

- ✅ Все миграции используют `IF NOT EXISTS` - безопасны для повторного запуска
- ✅ Применяются автоматически при `docker-compose up`
- ✅ Порядок важен (префиксы 01, 02, 03)

### Ручное применение (если нужно):

```bash
# На production сервере
docker exec -i monitoring-clickhouse clickhouse-client --database=monitoring < clickhouse/migrations.sql

# Или используя bash скрипт
cd clickhouse && bash apply_on_production.sh
```

### Проверка применённых миграций:

```bash
# Посмотреть все таблицы
docker exec monitoring-clickhouse clickhouse-client --query="SHOW TABLES FROM monitoring"

# Проверить application_categories
docker exec monitoring-clickhouse clickhouse-client --database=monitoring --query="SELECT count(*) FROM application_categories"
```

### При добавлении новой миграции:

1. Создать файл с префиксом по порядку: `04-new-migration.sql`
2. Добавить в `docker-compose.yml`:
   ```yaml
   - ./clickhouse/04-new-migration.sql:/docker-entrypoint-initdb.d/04-new-migration.sql
   ```
3. Использовать `CREATE TABLE IF NOT EXISTS` для идемпотентности
4. Пересоздать контейнер: `docker-compose up -d --force-recreate clickhouse`


# ✅ Настройка автоматического применения миграций

**Дата:** 26 ноября 2025  
**Проблема:** Миграции не применялись автоматически при старте ClickHouse

---

## 🔴 Что было:

В `docker-compose.yml` монтировался только `init.sql`:
```yaml
volumes:
  - ./clickhouse/init.sql:/docker-entrypoint-initdb.d/init.sql
```

**Результат:**
- ❌ `migrations.sql` не применялся → нет `application_categories`
- ❌ `add_application_categories_table.sql` не применялся
- ❌ Ошибки "Unknown table...application_categories" в логах

---

## ✅ Что исправлено:

Добавлены все миграции в `docker-compose.yml`:
```yaml
volumes:
  - clickhouse_data:/var/lib/clickhouse
  - ./clickhouse/init.sql:/docker-entrypoint-initdb.d/01-init.sql
  - ./clickhouse/migrations.sql:/docker-entrypoint-initdb.d/02-migrations.sql
  - ./clickhouse/add_application_categories_table.sql:/docker-entrypoint-initdb.d/03-categories.sql
```

**Префиксы 01, 02, 03** задают порядок выполнения!

---

## 📋 Порядок применения миграций:

1. **01-init.sql** - Базовые таблицы
   - activity_events, keyboard_events, usb_events
   - activity_segments + materialized views
   - employees, alerts, screenshots

2. **02-migrations.sql** - Дополнительные таблицы
   - application_categories (5 категорий: productive/unproductive/neutral/communication/system)
   - system_settings
   - Индексы для оптимизации

3. **03-categories.sql** - Обновление categories
   - Пересоздание application_categories (3 категории: productive/neutral/unproductive)
   - 14 предзаполненных категорий

---

## 🚀 Deployment на production:

### Вариант 1: Ручное применение (СЕЙЧАС)

Применить миграции вручную БЕЗ пересоздания контейнера:

```bash
# На production сервере
cd /opt/monitoring

# Применить migrations.sql
docker exec -i monitoring-clickhouse clickhouse-client --database=monitoring \
  < clickhouse/migrations.sql

# Применить categories
docker exec -i monitoring-clickhouse clickhouse-client --database=monitoring \
  < clickhouse/add_application_categories_table.sql

# Проверить что таблица создана
docker exec monitoring-clickhouse clickhouse-client --database=monitoring \
  --query="SELECT count(*) FROM application_categories"
```

**Результат:** Таблица создана, ошибки исчезли, контейнер НЕ пересоздавался!

---

### Вариант 2: Автоматическое применение (при следующем рестарте)

После обновления `docker-compose.yml` на production:

```bash
# Скачать новый docker-compose.yml
git pull origin main

# Пересоздать контейнер ClickHouse (данные сохранятся)
docker-compose up -d --force-recreate clickhouse

# Миграции применятся автоматически при старте!
```

⚠️ **Важно:** Используйте `--force-recreate` только для ClickHouse, НЕ для всех сервисов!

---

## 📊 Проверка результата:

```bash
# Все таблицы в БД
docker exec monitoring-clickhouse clickhouse-client \
  --query="SHOW TABLES FROM monitoring"

# Количество категорий (должно быть 14)
docker exec monitoring-clickhouse clickhouse-client --database=monitoring \
  --query="SELECT count(*) FROM application_categories"

# Посмотреть категории
docker exec monitoring-clickhouse clickhouse-client --database=monitoring \
  --query="SELECT process_name, category FROM application_categories WHERE is_active=1 FORMAT Pretty"
```

---

## 🎯 Итого:

✅ **docker-compose.yml** обновлён - миграции применяются автоматически  
✅ **clickhouse/README_MIGRATIONS.md** создан - документация по миграциям  
✅ **Ручное применение** доступно для production БЕЗ рестарта  
✅ **Автоматическое применение** при следующем `docker-compose up`

---

## 🔄 При добавлении новых миграций:

1. Создать файл: `clickhouse/04-new-feature.sql`
2. Добавить в docker-compose.yml:
   ```yaml
   - ./clickhouse/04-new-feature.sql:/docker-entrypoint-initdb.d/04-new-feature.sql
   ```
3. Использовать `CREATE TABLE IF NOT EXISTS` для идемпотентности
4. Применить вручную ИЛИ пересоздать контейнер


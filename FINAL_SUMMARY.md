# ✅ РЕШЕНИЕ: Нормальные миграции ClickHouse

## 📋 ЧТО СДЕЛАНО:

### 1. Объединены миграции (5 → 2 файла)

**Было:**
```
clickhouse/
├── init.sql
├── migrations.sql
├── add_application_categories_table.sql
├── add_activity_segments.sql
└── migration_add_activity_fields.sql
```

**Стало:**
```
clickhouse/
├── 01-schema.sql         ← ВСЯ схема БД
├── 02-seed-data.sql      ← Начальные данные (80+ категорий)
├── README_MIGRATIONS.md  ← Документация
└── TEST_MIGRATIONS.sh    ← Тестовый скрипт
```

### 2. Исправлен docker-compose.yml

**Было (с опечатками):**
```yaml
volumes:
  - ./clickhouse/init.sql:/docker-entrypoint-initdb.d/01-init.sql
  - ./clickhouse/migrations.sql:/docker-entrypoint-initdb.d/02-migrations.sql
  - ./clickhouse/add_application_categories_table.sql:/docker-entrypoint-initdb.d/03-categories.sql
  - ./clickhouse/add_activity_segments.sql:/docker-entrypoint-initdb.d/04-categories.sqladd_activity_segments  ❌
  - ./clickhouse/migration_add_activity_fields.sql:/docker-entrypoint-initdb.d/05-categories.sqladd_activity_segments  ❌
```

**Стало (чисто):**
```yaml
volumes:
  - ./clickhouse/01-schema.sql:/docker-entrypoint-initdb.d/01-schema.sql
  - ./clickhouse/02-seed-data.sql:/docker-entrypoint-initdb.d/02-seed-data.sql
```

### 3. Добавлено логирование

Миграции теперь выводят прогресс:
```sql
\echo '========================================='
\echo 'Starting schema migration...'
\echo '========================================='
\echo 'Creating activity_events table...'
...
\echo 'Schema migration completed successfully!'
```

Логи видны в контейнере:
```bash
docker logs monitoring-clickhouse 2>&1 | grep migration
```

### 4. Идемпотентность

Все операции безопасны при повторном выполнении:
- ✅ `CREATE TABLE IF NOT EXISTS`
- ✅ `ALTER TABLE ADD COLUMN IF NOT EXISTS`
- ✅ `CREATE MATERIALIZED VIEW IF NOT EXISTS`
- ✅ `ADD INDEX IF NOT EXISTS`

### 5. Тестовый скрипт

`clickhouse/TEST_MIGRATIONS.sh` проверяет:
- ✅ ClickHouse доступен
- ✅ База данных monitoring существует
- ✅ Все 12 таблиц созданы
- ✅ 80+ категорий приложений загружены
- ✅ 3 materialized views созданы
- ✅ Индексы созданы

---

## 🚀 КАК ПРИМЕНИТЬ НА PRODUCTION:

### Вариант 1: Пересоздать контейнер (чистая установка)

```bash
docker-compose stop clickhouse
docker-compose rm -f clickhouse
docker volume rm office-monitor_clickhouse_data
docker-compose up -d clickhouse

# Проверить
docker logs monitoring-clickhouse 2>&1 | grep -E "migration|completed"
```

### Вариант 2: Применить на существующей базе (сохраняет данные)

```bash
cat clickhouse/01-schema.sql | docker exec -i monitoring-clickhouse clickhouse-client --database monitoring
cat clickhouse/02-seed-data.sql | docker exec -i monitoring-clickhouse clickhouse-client --database monitoring

# Проверить
docker exec monitoring-clickhouse clickhouse-client --database monitoring \
  -q "SELECT count(*) FROM application_categories"
```

### Вариант 3: Использовать тестовый скрипт

```bash
./clickhouse/TEST_MIGRATIONS.sh

# Вывод:
# ✅ ClickHouse доступен
# ✅ База данных monitoring существует
# ✅ Найдено таблиц: 12
# ✅ Найдено категорий: 85
# ✅ ВСЕ ПРОВЕРКИ ПРОЙДЕНЫ!
```

---

## ✅ ПОСЛЕ ПРИМЕНЕНИЯ:

1. ✅ Таблица `application_categories` создана
2. ✅ 80+ категорий приложений (IDE, браузеры, офис, мессенджеры, игры)
3. ✅ API `/api/categories` работает (не 500!)
4. ✅ "Справочник программ" работает
5. ✅ Отчёты показывают категории
6. ✅ Логи чистые (нет "Unknown table")

---

## 📂 ФАЙЛЫ:

**Основные:**
- ✅ `clickhouse/01-schema.sql` - вся схема БД
- ✅ `clickhouse/02-seed-data.sql` - начальные данные
- ✅ `docker-compose.yml` - исправлен

**Документация:**
- ✅ `clickhouse/README_MIGRATIONS.md` - как работают миграции
- ✅ `DEPLOY_MIGRATIONS.md` - инструкция для production
- ✅ `clickhouse/TEST_MIGRATIONS.sh` - тестирование

**Устаревшие (можно удалить):**
- ❌ `clickhouse/init.sql`
- ❌ `clickhouse/migrations.sql`
- ❌ `clickhouse/add_application_categories_table.sql`
- ❌ `clickhouse/add_activity_segments.sql`
- ❌ `clickhouse/migration_add_activity_fields.sql`

---

## 🎯 ГОТОВО!

Выберите один из 3 вариантов выше и примените миграции.

**Время выполнения:** 3-5 минут  
**Результат:** Справочник программ заработает! 🚀


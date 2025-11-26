# 🚀 Инструкция по применению миграций на production

## ✅ ЧТО ИСПРАВЛЕНО:

1. **Объединены миграции** - теперь вместо 5 файлов только 2:
   - `01-schema.sql` - вся схема базы данных
   - `02-seed-data.sql` - начальные данные (80+ категорий)

2. **Исправлены опечатки в docker-compose.yml**:
   - Было: `04-categories.sqladd_activity_segments` ❌
   - Стало: `01-schema.sql` ✅

3. **Добавлено логирование** - все миграции пишут в stdout контейнера

4. **Идемпотентность** - можно запускать многократно безопасно

5. **Тестовый скрипт** - `clickhouse/TEST_MIGRATIONS.sh` проверяет всё

---

## 📋 ПЛАН ДЕЙСТВИЙ НА PRODUCTION:

### Вариант 1: Пересоздать контейнер (рекомендуется для чистой установки)

```bash
# 1. Остановить и удалить контейнер ClickHouse
docker-compose stop clickhouse
docker-compose rm -f clickhouse

# 2. Удалить volume (ВНИМАНИЕ: удалит все данные!)
docker volume rm office-monitor_clickhouse_data

# 3. Запустить заново (миграции применятся автоматически)
docker-compose up -d clickhouse

# 4. Проверить логи миграций
docker logs monitoring-clickhouse 2>&1 | grep -E "migration|Creating|completed"

# 5. Проверить что таблица создана
docker exec monitoring-clickhouse clickhouse-client --database monitoring \
  -q "SELECT count(*) FROM application_categories"
```

### Вариант 2: Применить миграции на существующей базе (сохраняет данные)

```bash
# 1. Применить схему
cat clickhouse/01-schema.sql | docker exec -i monitoring-clickhouse clickhouse-client --database monitoring

# 2. Применить seed data
cat clickhouse/02-seed-data.sql | docker exec -i monitoring-clickhouse clickhouse-client --database monitoring

# 3. Проверить
docker exec monitoring-clickhouse clickhouse-client --database monitoring \
  -q "SELECT count(*) FROM application_categories"

# Должно показать > 80
```

### Вариант 3: Использовать тестовый скрипт

```bash
# Скрипт проверит и применит миграции автоматически
./clickhouse/TEST_MIGRATIONS.sh

# Вывод покажет:
# ✅ ClickHouse доступен
# ✅ База данных monitoring существует
# ✅ Найдено таблиц: 12
# ✅ Найдено категорий: 85
# ✅ ВСЕ ПРОВЕРКИ ПРОЙДЕНЫ!
```

---

## 🔍 ПРОВЕРКА ПОСЛЕ ПРИМЕНЕНИЯ:

```bash
# 1. Проверить таблицы
docker exec monitoring-clickhouse clickhouse-client --database monitoring -q "SHOW TABLES"

# Должно показать:
# activity_events
# activity_segments
# alerts
# application_categories  ← ВАЖНО!
# agent_configs
# employees
# file_copy_events
# keyboard_events
# process_catalog
# screenshot_metadata
# system_settings
# usb_events
# + materialized views

# 2. Проверить категории
docker exec monitoring-clickhouse clickhouse-client --database monitoring \
  -q "SELECT category, count(*) FROM application_categories GROUP BY category"

# Должно показать:
# productive       40
# communication    7
# neutral          12
# unproductive     15
# entertainment    3

# 3. Проверить API
curl http://monitor.net.gslaudit.ru/api/categories | jq

# Должен вернуть массив категорий (не ошибку 500!)

# 4. Проверить логи сервера
docker logs monitoring-server --tail 50 | grep categories

# Не должно быть ошибок "Unknown table"
```

---

## 📝 ЛОГИ МИГРАЦИЙ:

```bash
# Просмотр логов миграций
docker logs monitoring-clickhouse 2>&1 | grep -A 5 "migration"

# Пример успешного вывода:
# =========================================
# Starting schema migration...
# =========================================
# Creating activity_events table...
# Creating activity_segments table...
# Creating application_categories table...
# ...
# Schema migration completed successfully!
# =========================================
```

---

## ⚠️ TROUBLESHOOTING:

### Миграции не применились:

```bash
# Применить вручную с выводом ошибок
docker exec -i monitoring-clickhouse clickhouse-client --database monitoring \
  --multiquery < clickhouse/01-schema.sql

# Если ошибка - посмотреть детали
docker logs monitoring-clickhouse 2>&1 | grep -i error
```

### "Unknown table expression identifier":

Значит таблица не создалась. Применить миграции вручную (Вариант 2).

### Категории не загрузились:

```bash
# Проверить что таблица пустая
docker exec monitoring-clickhouse clickhouse-client --database monitoring \
  -q "SELECT count(*) FROM application_categories"

# Если 0 - применить seed data
cat clickhouse/02-seed-data.sql | docker exec -i monitoring-clickhouse clickhouse-client --database monitoring
```

---

## ✅ ОЖИДАЕМЫЙ РЕЗУЛЬТАТ:

После применения миграций:

1. ✅ Таблица `application_categories` создана
2. ✅ 80+ категорий приложений загружены
3. ✅ API `/api/categories` возвращает данные (не 500)
4. ✅ "Справочник программ" работает
5. ✅ Отчёты показывают категории (productive/neutral/etc)
6. ✅ Логи не содержат ошибок "Unknown table"

---

## 🎯 ГОТОВО!

Выполните один из вариантов выше и проверьте результат.

**Время выполнения:** 3-5 минут

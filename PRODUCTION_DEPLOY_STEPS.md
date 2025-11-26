# 🚀 ИНСТРУКЦИЯ ПО УСТАНОВКЕ НА PRODUCTION

## ❌ КРИТИЧЕСКИЕ ПРОБЛЕМЫ:

### 1. Таблица `application_categories` не создана
**Из логов:**
```
"error":"code: 60, message: Unknown table expression identifier 'monitoring.application_categories'"
```

### 2. Frontend submodule не затянут (пустая директория)

---

## ✅ ЧТО НУЖНО СДЕЛАТЬ НА PRODUCTION:

### Шаг 1: Затянуть frontend submodule

```bash
# На production сервере
cd /path/to/Office-Monitor
git submodule update --init --recursive
git submodule update --remote frontend
ls -la frontend/  # Проверить что там есть файлы
```

**Что должно быть:**
```
frontend/
├── src/
├── public/
├── package.json
└── README.md
```

---

### Шаг 2: Создать таблицу application_categories

```bash
# Выполнить на production сервере
cat CREATE_CATEGORIES_TABLE.sql | docker exec -i clickhouse clickhouse-client --database monitoring
```

**Или через clickhouse-client:**
```bash
docker exec -it clickhouse clickhouse-client --database monitoring

# Внутри clickhouse-client:
CREATE TABLE IF NOT EXISTS monitoring.application_categories (
    id UUID DEFAULT generateUUIDv4(),
    process_name String,
    process_pattern String,
    category Enum8(
        'productive' = 1, 
        'unproductive' = 2, 
        'neutral' = 3, 
        'communication' = 4, 
        'entertainment' = 5
    ),
    created_at DateTime DEFAULT now(),
    updated_at DateTime DEFAULT now(),
    created_by String DEFAULT '',
    updated_by String DEFAULT '',
    is_active UInt8 DEFAULT 1
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (id);

-- Добавить базовые категории
INSERT INTO monitoring.application_categories 
(id, process_name, process_pattern, category, created_by, is_active) 
VALUES
(generateUUIDv4(), 'code.exe', '.*', 'productive', 'system', 1),
(generateUUIDv4(), 'excel.exe', '.*', 'productive', 'system', 1),
(generateUUIDv4(), 'winword.exe', '.*', 'productive', 'system', 1),
(generateUUIDv4(), 'teams.exe', '.*', 'communication', 'system', 1),
(generateUUIDv4(), 'chrome.exe', '.*', 'neutral', 'system', 1);

-- Проверить
SELECT * FROM monitoring.application_categories FINAL;
```

---

### Шаг 3: Перезапустить сервер

```bash
# На production
docker-compose restart monitoring-server
docker logs monitoring-server --tail 50
```

---

### Шаг 4: Проверить что всё работает

```bash
# 1. Проверить таблицу
docker exec -it clickhouse clickhouse-client --database monitoring \
  -q "SELECT count(*) FROM monitoring.application_categories"

# Должно быть > 0

# 2. Проверить API
curl http://monitor.net.gslaudit.ru/api/categories | jq

# Должен вернуть список категорий

# 3. Проверить отчёты
curl "http://monitor.net.gslaudit.ru/api/reports/daily/a-kiv?date=2025-11-26" | jq '.applications[0]'

# Должен показать category (productive/neutral/etc)
```

---

## 🔍 ПРОВЕРКА FRONTEND:

После того как затянули submodule:

```bash
cd frontend
cat package.json  # Проверить что это React/TypeScript проект
```

**Если нужно собрать frontend:**
```bash
cd frontend
npm install
npm run build
# Файлы появятся в dist/ или build/
```

---

## ✅ ОЖИДАЕМЫЙ РЕЗУЛЬТАТ:

После всех шагов:

1. ✅ Frontend submodule затянут и содержит файлы
2. ✅ Таблица `application_categories` создана
3. ✅ API `/api/categories` возвращает список приложений
4. ✅ Отчёты показывают категории приложений (productive/neutral/etc)
5. ✅ "Справочник программ" работает без ошибок

---

## 🐛 ЕСЛИ ЧТО-ТО НЕ РАБОТАЕТ:

**Frontend пустой:**
```bash
git submodule status
# Если показывает минус перед frontend - значит не инициализирован
git submodule update --init --recursive
```

**Таблица не создалась:**
```bash
# Удалить и создать заново
docker exec -it clickhouse clickhouse-client --database monitoring \
  -q "DROP TABLE IF EXISTS monitoring.application_categories"
# Затем выполнить CREATE TABLE из CREATE_CATEGORIES_TABLE.sql
```

**Логи показывают ошибки:**
```bash
docker logs monitoring-server --tail 100 | grep -i error
```


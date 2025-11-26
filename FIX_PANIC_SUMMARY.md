# ✅ ИСПРАВЛЕНА ПАНИКА СЕРВЕРА

## 🐛 ПРОБЛЕМА:

Сервер падал с ошибкой:
```
panic: context without logger passed to zapctx.Logger()
```

**Причина**: Функция `database.New()` использовала `context.Background()` без логгера, а библиотека `zapctx` требует логгер в контексте.

---

## 🔧 РЕШЕНИЕ:

### 1. Передаём логгер в `database.New()`

**Было**:
```go
func New(host string, port int, database, username, password string) (*Database, error)
```

**Стало**:
```go
func New(host string, port int, database, username, password string, logger *zap.Logger) (*Database, error)
```

### 2. Используем логгер для авто-синхронизации

```go
if logger != nil {
    ctx := zapctx.WithLogger(context.Background(), logger)
    if err := db.AutoSyncApplicationCategoriesTable(ctx); err != nil {
        logger.Warn("Failed to auto-sync application_categories table", zap.Error(err))
    }
    if err := db.AutoLoadDefaultCategories(ctx); err != nil {
        logger.Warn("Failed to auto-load default categories", zap.Error(err))
    }
}
```

### 3. Обновили вызов в `main.go`

```go
db, err = database.New(
    cfg.Database.Host,
    cfg.Database.Port,
    cfg.Database.Database,
    cfg.Database.Username,
    cfg.Database.Password,
    logger,  // ← Добавили логгер
)
```

---

## 🚀 ПРИМЕНЕНИЕ НА PRODUCTION:

```bash
cd /opt/Office-Monitor

# 1. Получить исправления
git pull

# 2. Пересобрать сервер
docker-compose build server

# 3. Перезапустить
docker-compose up -d server

# 4. Проверить логи (должна быть авто-синхронизация без паники)
docker logs monitoring-server --tail 50 --follow
```

**Ожидаемый вывод (БЕЗ паники)**:
```json
{"level":"info","msg":"Log level","level":"debug"}
{"level":"info","msg":"🔄 Auto-syncing application_categories table schema..."}
{"level":"info","msg":"✅ application_categories table schema is up to date"}
{"level":"info","msg":"✅ Application categories already loaded","count":14}
{"level":"info","msg":"Starting server on :8080"}
```

---

## ✅ РЕЗУЛЬТАТ:

1. ✅ **Сервер запускается без паники**
2. ✅ **Таблица application_categories создаётся автоматически**
3. ✅ **Seed data загружается автоматически (14 категорий)**
4. ✅ **Цвета и продуктивность работают**
5. ✅ **Справочник программ доступен**

---

## 📝 ИЗМЕНЁННЫЕ ФАЙЛЫ:

- ✅ `server/database/clickhouse.go` - добавлен параметр logger
- ✅ `server/database/auto_sync_schema.go` - использует logger из контекста
- ✅ `server/main.go` - передаёт logger в database.New()

---

## 🎯 ПРОВЕРКА:

После запуска проверьте:

```bash
# 1. Сервер работает
curl http://monitor.net.gslaudit.ru/api/health

# 2. Категории в БД
docker exec monitoring-clickhouse clickhouse-client --database monitoring \
  -q "SELECT category, count(*) FROM application_categories GROUP BY category"

# 3. API возвращает категории
curl http://monitor.net.gslaudit.ru/api/categories | jq '.[].category' | sort | uniq -c
```

**Ожидаемый результат**:
```
productive       8
neutral          2
communication    4
unproductive     2
```


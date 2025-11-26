# ✅ Решение проблем с цветами и продуктивностью

## 🔧 ЧТО ИСПРАВЛЕНО:

### 1. **Автоматическая синхронизация схемы БД**

Теперь при запуске сервера **автоматически**:
- ✅ Создается таблица `application_categories` (если отсутствует)
- ✅ Добавляются индексы
- ✅ Загружаются базовые категории (если таблица пуста)

**Файл**: `server/database/auto_sync_schema.go`

```go
// AutoSyncApplicationCategoriesTable - создает таблицу при старте
// AutoLoadDefaultCategories - загружает seed data если пусто
```

### 2. **Категоризация через БД вместо hardcode**

**Было** (hardcoded списки):
```go
func categorizeApplication(processName, windowTitle string) string {
    productive := []string{"code.exe", "excel.exe", ...}
    // ...
}
```

**Стало** (использует таблицу):
```go
func (db *Database) categorizeApplication(ctx context.Context, processName, windowTitle string) string {
    category, err := db.MatchProcessToCategory(ctx, processName, windowTitle)
    if err == nil && category != "neutral" {
        return category // Из БД!
    }
    // Fallback для браузеров (GitHub, YouTube, etc)
}
```

---

## 🚀 КАК ПРИМЕНИТЬ:

### На production сервере:

```bash
cd /opt/Office-Monitor

# 1. Получить новый код
git pull

# 2. Пересобрать сервер
docker-compose build server

# 3. Перезапустить
docker-compose restart server

# 4. Проверить логи
docker logs monitoring-server --tail 30
```

**Ожидаемый вывод**:
```
🔄 Auto-syncing application_categories table schema...
✅ application_categories table schema is up to date
✅ Application categories already loaded count=10
```

---

## ✅ РЕЗУЛЬТАТ:

После перезапуска:

1. **Цвета заработают**:
   - Productive apps → Зеленый
   - Unproductive → Красный
   - Communication → Синий
   - Neutral → Серый

2. **Продуктивность НЕ 0%**:
   - Excel, Code, PowerShell → считаются продуктивными
   - Chrome с GitHub → продуктивный
   - Chrome с YouTube → непродуктивный

3. **Справочник программ работает**:
   - Можно добавлять новые категории
   - Изменения сразу применяются

---

## 🎯 ПРЕИМУЩЕСТВА АВТО-СИНХРОНИЗАЦИИ:

✅ **Не нужны миграции вручную** - схема создается автоматически
✅ **Работает на любом окружении** - dev/prod
✅ **Идемпотентно** - можно запускать многократно
✅ **Безопасно** - не падает если таблица уже есть
✅ **Seed data автоматом** - базовые категории всегда доступны

---

## 📝 ФАЙЛЫ:

**Новые**:
- ✅ `server/database/auto_sync_schema.go` - авто-синхронизация

**Изменены**:
- ✅ `server/database/clickhouse.go` - вызов синхронизации при старте
- ✅ `server/database/frontend_queries.go` - использование БД для категорий

---

## 🔍 ПРОВЕРКА:

```bash
# Проверить что категории в БД
docker exec monitoring-clickhouse clickhouse-client --database monitoring \
  -q "SELECT category, count(*) FROM application_categories GROUP BY category"

# Должно показать:
# productive       4
# neutral          2
# communication    4
# unproductive     2

# Проверить API
curl http://monitor.net.gslaudit.ru/api/categories | jq '.[] | {name: .process_name, category: .category}'
```


# Production Deployment Guide

**Дата:** 24 ноября 2025  
**Версия:** После исправления ошибок 400 и 500

---

## 🚀 Быстрый старт

### Шаг 1: Обновить server binary

```bash
# На локальной машине (где собрали server)
scp server/server user@monitor.net.gslaudit.ru:/opt/monitoring/

# На production сервере
ssh user@monitor.net.gslaudit.ru
sudo systemctl stop monitoring-server
sudo cp /opt/monitoring/server /usr/local/bin/monitoring-server
sudo systemctl start monitoring-server
sudo systemctl status monitoring-server
```

### Шаг 2: Применить миграции ClickHouse

```bash
# На production сервере
cd /opt/monitoring

# Применить init.sql (содержит ВСЕ таблицы включая activity_segments)
docker exec -i clickhouse clickhouse-client --database=monitoring < clickhouse/init.sql

# Применить дополнительные миграции
docker exec -i clickhouse clickhouse-client --database=monitoring < clickhouse/migrations.sql

# Проверить что таблица создана
docker exec clickhouse clickhouse-client --database=monitoring --query="SHOW TABLES"
```

### Шаг 3: Проверить логи

```bash
# Логи сервера
sudo journalctl -u monitoring-server -f -n 50

# Логи агента (на клиентской машине)
tail -f C:\monitoring-agent\logs\agent.log

# Ожидаем увидеть:
# ✅ POST /api/events/batch succeeded (200)
# ✅ Нет ошибок 400 "No valid events"
# ✅ Нет ошибок 500 на activity segments
```

---

## 📋 Что исправлено

### Проблема 1: Error 400 "No valid events in batch"

**Причина:** Сервер обрабатывал только `type="activity"`, игнорировал file/keyboard/usb

**Решение:** ✅ Добавлена обработка всех типов событий в `server/main.go`

**Файлы:** `server/main.go` (функция receiveBatchEventsHandler)

### Проблема 2: Error 500 на activity segments

**Причина:** Таблица `monitoring.activity_segments` не создана в ClickHouse

**Решение:** ✅ Таблица уже есть в `clickhouse/init.sql`, просто нужно применить миграции

**Файлы:** `clickhouse/init.sql` (строки 143-209)

---

## 📂 Структура миграций

```
clickhouse/
├── init.sql                          # Основные таблицы (ГЛАВНЫЙ ФАЙЛ)
│   ├── activity_events               # События активности
│   ├── keyboard_events               # Клавиатурные события
│   ├── file_copy_events              # Копирование файлов
│   ├── usb_events                    # USB события
│   ├── screenshot_metadata           # Метаданные скриншотов
│   ├── alerts                        # Алерты
│   ├── activity_segments             # ⭐ Сегменты активности (active/idle/offline)
│   ├── process_catalog               # Каталог процессов (friendly names)
│   ├── agent_configs                 # Конфигурация агентов
│   ├── employees                     # Сотрудники
│   ├── daily_activity_summary        # Materialized view
│   ├── program_usage_daily           # Materialized view
│   └── индексы для всех таблиц
│
├── migrations.sql                    # Дополнительные миграции
│   ├── application_categories        # Категории приложений
│   ├── system_settings               # Системные настройки
│   └── дополнительные индексы
│
└── migration_add_activity_fields.sql # Мелкие изменения
```

---

## ✅ Checklist перед деплоем

- [ ] Скомпилирован новый server binary (43MB)
- [ ] Server binary скопирован на production
- [ ] Применены миграции init.sql
- [ ] Применены миграции migrations.sql (опционально)
- [ ] Перезапущен monitoring-server
- [ ] Проверены логи сервера (нет ошибок)
- [ ] Проверены логи агента (200 OK на /api/events/batch)
- [ ] Проверено что данные появляются в ClickHouse

---

## 🔍 Проверка работоспособности

### 1. Проверить таблицы в ClickHouse

```bash
docker exec clickhouse clickhouse-client --database=monitoring --query="
SELECT 
    table, 
    engine, 
    total_rows 
FROM system.tables 
WHERE database='monitoring' 
ORDER BY table
"
```

### 2. Проверить что данные пишутся

```bash
# Activity segments
docker exec clickhouse clickhouse-client --database=monitoring --query="
SELECT count() FROM activity_segments WHERE timestamp_start > now() - INTERVAL 1 HOUR
"

# Activity events
docker exec clickhouse clickhouse-client --database=monitoring --query="
SELECT count() FROM activity_events WHERE timestamp > now() - INTERVAL 1 HOUR
"

# File events
docker exec clickhouse clickhouse-client --database=monitoring --query="
SELECT count() FROM file_copy_events WHERE timestamp > now() - INTERVAL 1 HOUR
"
```

### 3. Проверить что нет ошибок в агенте

```bash
# На Windows машине с агентом
tail -20 C:\monitoring-agent\logs\agent.log

# Ожидаем увидеть:
# 2025/11/24 23:00:00 client.go:133: POST /api/events/batch succeeded (200)
# 2025/11/24 23:00:05 activity_tracker_windows.go:219: POST /api/activity/segment succeeded (200)
```

---

## 🛠️ Troubleshooting

### Ошибка: "table already exists"

```bash
# Это нормально! IF NOT EXISTS защищает от дублирования
# Просто проигнорируйте
```

### Ошибка: "Unknown table activity_segments"

```bash
# Проверьте что применили init.sql
docker exec clickhouse clickhouse-client --database=monitoring --query="SHOW TABLES LIKE 'activity_segments'"

# Если пусто - применяем миграцию заново
docker exec -i clickhouse clickhouse-client --database=monitoring < clickhouse/init.sql
```

### Агент всё ещё возвращает 400/500

```bash
# 1. Убедитесь что перезапустили server
sudo systemctl restart monitoring-server

# 2. Проверьте версию server binary
/usr/local/bin/monitoring-server --version

# 3. Проверьте логи сервера
sudo journalctl -u monitoring-server -n 100
```

---

## 📞 Поддержка

Если возникли проблемы:

1. Проверьте логи сервера: `journalctl -u monitoring-server -f`
2. Проверьте логи агента: `tail -f C:\monitoring-agent\logs\agent.log`
3. Проверьте что ClickHouse запущен: `docker ps | grep clickhouse`
4. Проверьте подключение к ClickHouse: `docker exec clickhouse clickhouse-client --query="SELECT 1"`

---

**Успешного деплоя! 🚀**

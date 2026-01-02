# Развёртывание и миграции

## Требования

- Docker и Docker Compose
- 4 GB RAM минимум
- 50 GB свободного места

---

## Быстрый старт (чистая установка)

```bash
git clone <repo>
cd office-monitor

# Создать .env файл
cp .env.example .env
```

### Обязательные переменные в .env

```bash
# ClickHouse
CLICKHOUSE_USER=monitor_user
CLICKHOUSE_PASSWORD=your_secure_password

# MinIO (хранилище скриншотов)
MINIO_ROOT_USER=admin
MINIO_ROOT_PASSWORD=your_secure_password

# Grafana (если используется)
CLICKHOUSE_PASSWORD=your_secure_password
```

```bash
# Запустить все сервисы
docker-compose up -d
```

Миграции ClickHouse применяются автоматически при первом запуске.

---

## Перезапуск после удаления данных

Если база данных была удалена (очищены volumes), выполните:

```bash
# 1. Остановить все сервисы
docker-compose down

# 2. Пересоздать volumes и запустить
docker-compose up -d

# 3. Проверить что миграции применились
docker exec monitoring-clickhouse clickhouse-client --database monitoring \
  -q "SHOW TABLES"
```

Ожидаемые таблицы:
- activity_segments
- application_categories
- employees
- process_catalog
- screenshot_metadata
- usb_events
- file_copy_events
- keyboard_events
- alerts

---

## Обновление сервера

```bash
# 1. Получить изменения
cd office-monitor
git pull

# 2. Пересобрать и перезапустить сервер
docker-compose build server
docker-compose up -d server

# 3. Проверить логи
docker logs -f monitoring-server
```

---

## Применение миграций вручную

Миграции находятся в `clickhouse/`:
- `01-schema.sql` - схема таблиц (идемпотентная)
- `02-seed-data.sql` - справочник категорий приложений
- `03-grafana-views.sql` - views для Grafana (опционально)

```bash
# Применить схему
docker exec -i monitoring-clickhouse clickhouse-client --database monitoring \
  --multiquery < clickhouse/01-schema.sql

# Применить seed data
docker exec -i monitoring-clickhouse clickhouse-client --database monitoring \
  --multiquery < clickhouse/02-seed-data.sql

# Применить views для Grafana
docker exec -i monitoring-clickhouse clickhouse-client --database monitoring \
  --multiquery < clickhouse/03-grafana-views.sql
```

---

## Проверка состояния

### ClickHouse
```bash
# Статус контейнера
docker ps | grep clickhouse

# Health check
docker exec monitoring-clickhouse clickhouse-client -q "SELECT 1"

# Количество записей
docker exec monitoring-clickhouse clickhouse-client --database monitoring \
  -q "SELECT count() FROM activity_segments"
```

### Server
```bash
# Логи
docker logs -f monitoring-server

# Health endpoint
curl http://localhost:5000/health
```

### MinIO
```bash
# Web интерфейс
# http://localhost:9001
# Логин/пароль из .env (MINIO_ROOT_USER/MINIO_ROOT_PASSWORD)
```

---

## Troubleshooting

### Сервер не запускается
```bash
# Проверить логи
docker logs monitoring-server 2>&1 | tail -50

# Проверить подключение к ClickHouse
docker exec monitoring-server wget -qO- http://clickhouse:8123/ping
```

### Миграции не применились
```bash
# Проверить логи ClickHouse
docker logs monitoring-clickhouse 2>&1 | grep -i error

# Применить вручную
docker exec -i monitoring-clickhouse clickhouse-client --database monitoring \
  --multiquery < clickhouse/01-schema.sql
```

### База не создаётся
```bash
# Создать базу вручную
docker exec monitoring-clickhouse clickhouse-client \
  -q "CREATE DATABASE IF NOT EXISTS monitoring"
```

---

## Структура docker-compose

| Сервис | Контейнер | Порт | Описание |
|--------|-----------|------|----------|
| clickhouse | monitoring-clickhouse | 8123, 9000 | ClickHouse база данных |
| minio | monitoring-minio | 9000, 9001 | Хранилище файлов (скриншоты) |
| server | monitoring-server | 5000 | Go API сервер |
| nginx | monitoring-nginx | 80 | Reverse proxy |

---

## MinIO (хранилище)

Скриншоты хранятся в MinIO:
- Bucket: `screenshots`
- Retention: 30 дней (настраивается)
- Web UI: http://localhost:9001

```bash
# Проверить buckets
docker exec monitoring-minio mc ls local/
```

---

## Перезапуск nginx

После изменения конфигурации nginx:

```bash
docker exec monitoring-nginx nginx -s reload
# или
docker-compose restart nginx
```

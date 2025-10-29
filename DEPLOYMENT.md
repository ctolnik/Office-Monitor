# Office-Monitor - Инструкция по развертыванию

Полное развертывание системы мониторинга сотрудников с веб-интерфейсом.

## Архитектура

```
┌─────────────┐
│   Browser   │
└──────┬──────┘
       │ :80
┌──────▼──────────────┐
│  Nginx (reverse)    │
│  - Frontend (/)     │
│  - API proxy (/api) │
└──────┬──────────────┘
       │
   ┌───┴────┬────────────┐
   │        │            │
┌──▼──┐  ┌─▼─────┐  ┌───▼───────┐
│Front│  │Backend│  │ClickHouse │
│ end │  │  API  │  │ + MinIO   │
└─────┘  └───────┘  └───────────┘
```

## Компоненты

1. **Frontend** (React + TypeScript) - Веб-интерфейс администратора
2. **Backend** (Go/Gin) - REST API сервер
3. **ClickHouse** - База данных для событий мониторинга
4. **MinIO** - S3-хранилище для скриншотов и файлов
5. **Nginx** - Reverse proxy для фронтенда и API

## Быстрый старт

### 1. Клонирование репозитория с submodule

```bash
git clone --recurse-submodules git@github.com:ctolnik/Office-Monitor.git
cd Office-Monitor

# Если уже склонировали без --recurse-submodules:
git submodule update --init --recursive
```

### 2. Конфигурация

Создайте `.env` файл в корне проекта:

```bash
# Database
CLICKHOUSE_PASSWORD=your_secure_password_here

# MinIO Storage
MINIO_ROOT_USER=admin
MINIO_ROOT_PASSWORD=your_secure_minio_password
```

Настройте frontend `.env.production`:

```bash
cd frontend
cp .env.example .env.production

# Отредактируйте .env.production:
# VITE_API_URL=/api  (для production за nginx)
# VITE_API_KEY=your-api-key
```

### 3. Запуск всех сервисов

```bash
docker-compose up -d
```

Сервисы будут доступны:
- **Веб-интерфейс**: http://localhost
- **API**: http://localhost/api
- **MinIO Console**: http://localhost:9101
- **ClickHouse HTTP**: http://localhost:8123

### 4. Проверка статуса

```bash
docker-compose ps

# Проверка healthcheck
docker-compose ps | grep healthy

# Логи
docker-compose logs -f nginx
docker-compose logs -f server
docker-compose logs -f frontend
```

## Разработка

### Frontend (локально)

```bash
cd frontend
npm install
npm run dev  # Запуск на http://localhost:5173
```

Для локальной разработки создайте `.env`:
```env
VITE_API_URL=http://localhost:8080
VITE_API_KEY=dev-key
```

### Backend (локально)

```bash
cd server
go run main.go  # Запуск на :8080
```

Убедитесь, что ClickHouse и MinIO запущены:
```bash
docker-compose up -d clickhouse minio minio-init
```

## Обновление

### Обновление frontend

Frontend находится в отдельном git репозитории (submodule):

```bash
cd frontend
git pull origin main
cd ..
git add frontend
git commit -m "Update frontend submodule"
git push
```

### Обновление backend

```bash
cd server
# Внесите изменения
git add .
git commit -m "Update backend"
git push
```

### Пересборка Docker образов

```bash
# Полная пересборка
docker-compose build --no-cache

# Пересборка только frontend
docker-compose build frontend

# Пересборка только backend
docker-compose build server
```

## Production Deployment

### 1. Настройка HTTPS (рекомендуется)

Обновите `nginx/conf.d/default.conf`:

```nginx
server {
    listen 443 ssl http2;
    server_name your-domain.com;
    
    ssl_certificate /etc/nginx/ssl/cert.pem;
    ssl_certificate_key /etc/nginx/ssl/key.pem;
    
    # ... остальная конфигурация
}

server {
    listen 80;
    server_name your-domain.com;
    return 301 https://$server_name$request_uri;
}
```

Добавьте сертификаты в `docker-compose.yml`:

```yaml
nginx:
  volumes:
    - ./nginx/conf.d:/etc/nginx/conf.d:ro
    - ./nginx/ssl:/etc/nginx/ssl:ro  # SSL сертификаты
    - ./frontend/dist:/usr/share/nginx/html:ro
```

### 2. Настройка переменных окружения

Используйте безопасные пароли:

```bash
# Генерация случайного пароля
openssl rand -base64 32
```

### 3. Backup базы данных

```bash
# Backup ClickHouse
docker exec monitoring-clickhouse clickhouse-client \
  --query "BACKUP DATABASE monitoring TO Disk('backups', 'backup.zip')"

# Backup MinIO (AWS CLI)
docker run --rm -it \
  --entrypoint=/bin/sh \
  minio/mc \
  -c "mc mirror myminio/screenshots /backups/screenshots"
```

## Troubleshooting

### Frontend не загружается

```bash
# Проверьте логи nginx
docker-compose logs nginx

# Убедитесь, что frontend собран
cd frontend
npm run build
ls -la dist/

# Пересоберите frontend контейнер
docker-compose build frontend
docker-compose up -d frontend
```

### API возвращает 404

```bash
# Проверьте nginx конфигурацию
docker exec monitoring-nginx nginx -t

# Проверьте backend
docker-compose logs server

# Проверьте, что backend запущен на порту 8080
docker exec monitoring-server netstat -tulpn | grep 8080
```

### CORS ошибки

Убедитесь, что в `server/main.go` добавлен CORS middleware с правильными origins:

```go
router.Use(cors.New(cors.Config{
    AllowOrigins: []string{"http://localhost", "http://localhost:80"},
    // ...
}))
```

### ClickHouse не принимает подключения

```bash
# Проверьте healthcheck
docker-compose ps clickhouse

# Проверьте логи
docker-compose logs clickhouse

# Подключитесь вручную
docker exec -it monitoring-clickhouse clickhouse-client
```

## Мониторинг

### Логи

Все логи доступны через docker-compose:

```bash
# Все сервисы
docker-compose logs -f

# Только ошибки
docker-compose logs --tail=100 | grep ERROR

# Конкретный сервис
docker-compose logs -f server
```

### Healthchecks

```bash
# Nginx
curl http://localhost/health

# Backend
curl http://localhost/api/health  # если есть

# ClickHouse
docker exec monitoring-clickhouse clickhouse-client --query "SELECT 1"
```

### Метрики

- Frontend bundle size: проверяйте `frontend/dist/` после сборки
- Backend: добавьте Prometheus metrics endpoint
- ClickHouse: используйте system tables (system.query_log, system.metrics)

## Масштабирование

### Horizontal Scaling Backend

```yaml
server:
  deploy:
    replicas: 3
```

### Load Balancing

Настройте nginx upstream для балансировки:

```nginx
upstream backend {
    least_conn;
    server server-1:8080;
    server server-2:8080;
    server server-3:8080;
}
```

## Безопасность

1. **Используйте HTTPS** в production
2. **Смените пароли** ClickHouse и MinIO
3. **Настройте API ключи** для агентов
4. **Ограничьте доступ** к MinIO Console
5. **Регулярно обновляйте** зависимости
6. **Настройте firewall** для Docker сети

## Полезные команды

```bash
# Остановка всех сервисов
docker-compose down

# Остановка с удалением volumes (ВНИМАНИЕ: потеря данных!)
docker-compose down -v

# Просмотр использования ресурсов
docker stats

# Очистка неиспользуемых образов
docker system prune -a

# Обновление submodule (frontend)
git submodule update --remote frontend

# Просмотр изменений в submodule
git diff --submodule
```

## Поддержка

Проблемы и предложения: https://github.com/ctolnik/Office-Monitor/issues

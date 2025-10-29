# Office-Monitor - Быстрый старт

## За 5 минут до запуска

### 1. Клонируйте с frontend submodule

```bash
git clone --recurse-submodules git@github.com:ctolnik/Office-Monitor.git
cd Office-Monitor
```

### 2. Создайте `.env` файл

```bash
cat > .env << 'EOF'
CLICKHOUSE_PASSWORD=SecurePassword123
MINIO_ROOT_USER=admin
MINIO_ROOT_PASSWORD=MinIOPassword456
EOF
```

### 3. Настройте frontend

```bash
cat > frontend/.env.production << 'EOF'
VITE_API_URL=/api
VITE_ENABLE_SCREENSHOTS=true
VITE_ENABLE_KEYLOGGER=true
VITE_ENABLE_DLP=true
VITE_AGENTS_REFRESH_INTERVAL=30000
VITE_DASHBOARD_REFRESH_INTERVAL=60000
VITE_API_KEY=your-api-key-here
EOF
```

### 4. Запустите все сервисы

```bash
docker-compose up -d
```

### 5. Откройте веб-интерфейс

```
http://localhost
```

## Что работает

✅ **Веб-интерфейс**: http://localhost
- Dashboard с live статистикой
- Управление агентами (настройка мониторинга)
- Управление сотрудниками (CRUD + согласие)
- Детальные отчеты по активности с:
  - Timeline активности (24h визуализация)
  - Скриншоты (галерея с lightbox)
  - Клавиатурные события (форматированный просмотр с Backspace, Ctrl+A/C/V)
  - USB устройства (с ссылками на скопированные файлы)
  - Файловые операции (с DLP алертами красным цветом)
- Алерты (с фильтрацией и разрешением)
- Экспорт в PDF/Excel

✅ **API**: http://localhost/api
- Прием событий от агентов
- REST endpoints для фронтенда

✅ **Хранилище**:
- ClickHouse (события мониторинга)
- MinIO (скриншоты и файлы)

## Проверка статуса

```bash
docker-compose ps

# Должны быть running:
# - monitoring-clickhouse (healthy)
# - monitoring-minio (healthy)
# - monitoring-server (running)
# - monitoring-frontend (running)
# - monitoring-nginx (healthy)
```

## Логи

```bash
# Все сервисы
docker-compose logs -f

# Только nginx + server
docker-compose logs -f nginx server

# Ошибки
docker-compose logs --tail=100 | grep -i error
```

## Остановка

```bash
# Остановка без удаления данных
docker-compose down

# Остановка с удалением данных (ОСТОРОЖНО!)
docker-compose down -v
```

## Если что-то не работает

### Frontend не открывается

```bash
# Проверьте nginx
docker-compose logs nginx

# Пересоберите frontend
docker-compose build frontend
docker-compose up -d frontend nginx
```

### API не работает

```bash
# Проверьте backend
docker-compose logs server

# Проверьте CORS в server/main.go
# Должен быть cors.New с AllowOrigins
```

### База данных не работает

```bash
# Проверьте ClickHouse
docker-compose logs clickhouse

# Войдите в консоль
docker exec -it monitoring-clickhouse clickhouse-client

# SELECT version()
```

## Далее

Полная документация: [DEPLOYMENT.md](./DEPLOYMENT.md)

- Настройка HTTPS
- Production deployment
- Backup и восстановление
- Масштабирование
- Мониторинг и алерты

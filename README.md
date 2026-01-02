# Office Monitor - Система мониторинга активности сотрудников

Система мониторинга компьютерной активности для офиса (30 сотрудников).

## Возможности

- Отслеживание активных окон и приложений
- Категоризация приложений (продуктивные/непродуктивные)
- Периодические скриншоты
- Мониторинг USB устройств
- Детектирование массового копирования файлов
- Хранение данных 6 месяцев

## Компоненты

| Компонент | Технология | Описание |
|-----------|------------|----------|
| Сервер | Go + Gin | REST API, обработка данных |
| База данных | ClickHouse | Time-series хранение |
| Хранилище | MinIO | Скриншоты и файлы |
| Агент | Go (Windows) | Сбор данных на ПК |
| Аналитика | Grafana | Дашборды и отчёты |

## Документация

| Документ | Описание |
|----------|----------|
| [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) | Развёртывание и миграции |
| [docs/AGENT_SETUP.md](docs/AGENT_SETUP.md) | Установка агента на Windows |
| [docs/AI_CONTEXT.md](docs/AI_CONTEXT.md) | Контекст для AI-агентов |
| [docs/LOGGING.md](docs/LOGGING.md) | Настройка логирования |
| [grafana/README.md](grafana/README.md) | Настройка Grafana дашбордов |

## Быстрый старт

```bash
# Клонировать репозиторий
git clone <repo>
cd office-monitor

# Настроить переменные окружения
cp .env.example .env
# Отредактировать .env

# Запустить
docker-compose up -d
```

Сервер будет доступен на порту 5000.

## Структура проекта

```
├── server/           # Go backend
│   ├── handlers/     # HTTP handlers
│   └── database/     # ClickHouse queries
├── agent/            # Windows agent
│   └── monitoring/   # Monitoring modules
├── clickhouse/       # SQL migrations
├── grafana/          # Dashboard JSON
└── docs/             # Documentation
```

## Лицензия

Внутреннее использование. Требуется согласие сотрудников на мониторинг.

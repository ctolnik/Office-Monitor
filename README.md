# Office Monitor
Система мониторинга активности сотрудников: Go backend, Windows agent, ClickHouse, MinIO и frontend на React.
## Что в этом репозитории
- `server/` — backend API и бизнес-логика.
- `agent/` — Windows агент (service + session-helper).
- `clickhouse/` — SQL схема/сиды/views.
- `frontend/` — отдельный репозиторий `office-visor-ru`, подключённый как **git submodule**.
- `docs/` — каноничная документация проекта.
## Быстрый старт (локально)
```bash
git clone git@github.com:ctolnik/Office-Monitor.git
cd Office-Monitor
git submodule update --init --recursive
docker-compose up -d
```
После запуска:
- UI через nginx: `http://localhost`
- Backend API: `http://localhost:8081`
- MinIO console: `http://localhost:9101`
## Документация
Вся актуальная документация собрана в `docs/`.
Точка входа: `docs/README.md`.
## Важно про frontend
`frontend/` — это submodule. Для корректной сборки на сервере и локально нужно выполнять обновление submodule после `git pull`.
Полный рабочий flow (push/pull, docker-01): `docs/REPOSITORY_FLOW.md`.
## Лицензия
Внутреннее использование. Необходимо соблюдать локальные требования по уведомлению сотрудников о мониторинге.

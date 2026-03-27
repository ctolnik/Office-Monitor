# Развёртывание и обновление
## Требования
- Docker и Docker Compose.
- Доступ к GitHub (`Office-Monitor` + `office-visor-ru`).
- Для production обновлений: доступ к `docker-01` и каталогу `/opt/Office-Monitor`.
## Чистая установка
```bash
git clone git@github.com:ctolnik/Office-Monitor.git
cd Office-Monitor
git submodule update --init --recursive
docker-compose up -d
```
## Регулярное обновление на docker-01
```bash
cd /opt/Office-Monitor
git fetch origin --prune
git pull --ff-only origin main
git submodule sync --recursive
git submodule update --init --recursive --checkout
docker-compose build
docker-compose up -d
```
Это обязательная последовательность, иначе `frontend/` может оказаться не на том commit.
## Проверка после обновления
```bash
git rev-parse HEAD
git rev-parse origin/main
git submodule status --recursive
docker-compose ps
```
## Миграции ClickHouse вручную (если нужно)
```bash
docker exec -i monitoring-clickhouse clickhouse-client --database monitoring \
  --multiquery < clickhouse/01-schema.sql

docker exec -i monitoring-clickhouse clickhouse-client --database monitoring \
  --multiquery < clickhouse/02-seed-data.sql

docker exec -i monitoring-clickhouse clickhouse-client --database monitoring \
  --multiquery < clickhouse/03-grafana-views.sql
```
## Минимальный troubleshooting
### Не собирается frontend
Проверьте, что выполнены команды submodule:
```bash
git submodule sync --recursive
git submodule update --init --recursive --checkout
```
### Не поднимается backend
```bash
docker logs monitoring-server --tail 100
docker logs monitoring-clickhouse --tail 100
```
### Проверка health backend
```bash
curl -f http://localhost:8081/health
```

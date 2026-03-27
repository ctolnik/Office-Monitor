# AI Context (актуальный)
Этот файл фиксирует стабильный контекст для AI-агентов при работе с репозиторием.
## Источник истины
- Backend/agent: текущий репозиторий `Office-Monitor` (`main`).
- Frontend: отдельный репозиторий `office-visor-ru` как submodule в `frontend/`.
- Production-узел: `/opt/Office-Monitor` на `docker-01`.
## Обязательные правила работы с кодом
1. Любые правки в frontend делаются внутри `frontend/`, commit/push туда выполняется **до** commit в superproject.
2. После изменения frontend в superproject фиксируется новый указатель submodule (`git add frontend` + commit).
3. На сервере после `git pull --ff-only` обязательно выполнять:
   - `git submodule sync --recursive`
   - `git submodule update --init --recursive --checkout`
Полный регламент: `docs/REPOSITORY_FLOW.md`.
## Каноничная документация
- `docs/README.md` — индекс.
- `docs/DEPLOYMENT.md` — развёртывание/обновление.
- `docs/REPOSITORY_FLOW.md` — git/submodule flow.
- `docs/AGENT_SETUP.md` — установка агента.
- `docs/LOGGING.md` — логирование.
## Legacy backup каталоги (проверять перед удалением)
### На docker-01
- `/opt/Office-Monitor-submodule-backup-20260327_142149`
- `/opt/Office-Monitor-metadata-backup-20260327_142906`
- Общие маски на будущее:
  - `/opt/Office-Monitor-submodule-backup-*`
  - `/opt/Office-Monitor-metadata-backup-*`
### Локально
- `/Users/ikokv/dev/go/Office-Monitor-metadata-backup-20260327_172906`
- `/Users/ikokv/dev/go/Office-Monitor-legacy-frontend-backup-20260327_174003`
- Общая маска:
  - `/Users/ikokv/dev/go/Office-Monitor-*-backup-*`
## Быстрые проверки консистентности
```bash
git rev-parse HEAD
git rev-parse origin/main
git submodule status --recursive
git status --short --branch
```

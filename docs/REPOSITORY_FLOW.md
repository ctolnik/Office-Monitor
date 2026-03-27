# Git/Submodule flow: как не допускать рассинхронизации
## Текущая модель
- Основной репозиторий: `Office-Monitor` (ветка `main`).
- Frontend: отдельный репозиторий `office-visor-ru`, подключён как submodule в `frontend/`.
- Сервер `docker-01` обновляется только из GitHub через `git pull --ff-only`.
## Почему раньше «разъезжалось»
1. В `main` попадал указатель submodule на commit, которого ещё не было в удалённом `office-visor-ru`.
2. Были legacy-остатки (локальные директории, stale `.gitmodules`, stale `.git/modules/*`), которые маскировали реальное состояние.
3. На сервере делался только `git pull` без `git submodule update --init --recursive`.
## Правильный flow при изменениях в backend + frontend
### 1) Изменения в `frontend/` (submodule)
```bash
cd frontend
git checkout main
git pull --ff-only origin main
# ... правки ...
git add -A
git commit -m "..." 
git push origin main
```
### 2) Фиксация нового указателя submodule в `Office-Monitor`
```bash
cd ..
git add frontend
git commit -m "Update frontend submodule pointer"
git push origin main
```
Важно: сначала push в `office-visor-ru`, потом commit/push в `Office-Monitor`.
## Правильный pull на docker-01
```bash
cd /opt/Office-Monitor
git fetch origin --prune
git pull --ff-only origin main
git submodule sync --recursive
git submodule update --init --recursive --checkout
docker-compose build
docker-compose up -d
```
## Проверка консистентности
```bash
# Локально
git rev-parse HEAD
git rev-parse origin/main
git submodule status --recursive

# На docker-01
git rev-parse HEAD
git rev-parse origin/main
git submodule status --recursive
```
`HEAD` и `origin/main` должны совпадать, рабочее дерево — чистое.
## Политика безопасности изменений
- На сервере не выполнять ручные правки в рабочем каталоге.
- Если нужен hotfix на сервере: отдельная ветка, commit, push, merge в `main`.
- Перед крупной уборкой создавать backup-ветку (`backup/pre-cleanup-*`).

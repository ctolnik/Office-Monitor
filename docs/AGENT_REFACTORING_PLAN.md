# План рефакторинга Windows Agent

## Статус: Этапы 1-3 выполнены

## Проблема

Windows сервис работает в Session 0 и не имеет доступа к UI пользователя:
- `GetForegroundWindow()` возвращает "unknown"
- `GetLastInputInfo()` возвращает данные Session 0, не пользователя
- Keylogger hooks не перехватывают ввод из пользовательской сессии

## Решение: Единый Session Helper + единый канал доставки (IPC)

### Термины и совместимость артефактов
- `agent-svc.exe` — Windows Service (Session 0), оркестратор и единственная точка отправки данных на сервер.
- `agent-sh.exe` — **историческое имя** бинаря helper ("screenSHoot"), сохраняем для обратной совместимости с инсталляциями/скриптами.
- `session-helper` — **роль/компонент**: универсальный helper, который запускается в пользовательской сессии и выполняет весь UI-зависимый функционал.

### Ключевой принцип (SRP/DRY)
- Helper **не делает HTTP-запросов к серверу**.
- Service отвечает за: буферизацию, ретраи, `X-API-Key`, батч-отправку, единый httpclient.

### Архитектура

```
┌─────────────────────────────────────────────────────────────────┐
│                    Windows Service (Session 0)                   │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │ USB Monitor │  │ File Monitor│  │ Session Manager         │  │
│  │ (system)    │  │ (system)    │  │ - Track user sessions   │  │
│  └─────────────┘  └─────────────┘  │ - Launch/stop helpers   │  │
│                                     │ - Health monitoring     │  │
│  ┌──────────────────────────────┐  └─────────────────────────┘  │
│  │ Event Buffer & Server Sender │                               │
│  │ - Buffer events              │   ┌─────────────────────────┐ │
│  │ - Retry logic                │◄──│ Named Pipe Server       │ │
│  │ - Send to monitoring server  │   │ (IPC from helpers)      │ │
│  └──────────────────────────────┘   └─────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                              ▲
                              │ Named Pipe IPC (JSON events)
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│               Session Helper (User Session 1, 2, ...)            │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │ Activity Tracker│  │ Screenshot      │  │ Keylogger       │  │
│  │ - Foreground    │  │ - Capture screen│  │ - Browser input │  │
│  │ - Idle time     │  │ - Send to server│  │ - Send to pipe  │  │
│  │ - Send to pipe  │  │   (direct)      │  │                 │  │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘  │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ Structured Logger (zap, JSON format)                        ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

### Распределение обязанностей

| Компонент | Session 0 (Сервис) | User Session (Helper) |
|-----------|--------------------|-----------------------|
| USB мониторинг | ✅ | - |
| File мониторинг | ✅ | - |
| Activity tracking | - | ✅ |
| Screenshots | - | ✅ |
| Keylogger | - | ✅ |
| Event buffering | ✅ | - |
| Server communication | ✅ (events) | ✅ (screenshots) |
| Health checks | ✅ | - |
| Session management | ✅ | - |

---

## Этапы реализации

### Принятое решение по IPC
Основной вариант: **Named Pipe + chunked protocol** для скриншотов.
- Малые события (activity/keyboard/heartbeat) отправляются как JSON события по pipe.
- Скриншоты отправляются как поток чанков (binary) + отдельные JSON сообщения метаданных/commit.

Причина: не дублировать HTTP-логику в helper и не плодить несколько независимых каналов доставки.

### Этап 1: Структурированное логирование (zap) — довести до конца
- [x] 1.1 Создать пакет `agent/pkg/logger` с zap конфигурацией
- [x] 1.2 JSON формат с полями: ts, level, component, session_id, username, msg, error
- [ ] 1.3 **Перевести service на `agent/pkg/logger`** (сейчас есть старый `agent/logger` → нужно убрать/задепрекейтить)
- [ ] 1.4 **Единые имена лог-файлов** (service vs helper). Убрать/исправить устаревшие имена вида `session-helper.log`, если бинарь/артефакт называется `agent-sh.exe`.
- [x] 1.5 Ротация логов (lumberjack)

### Этап 2: IPC через Named Pipes (целевой путь)
- [x] 2.1 Создать пакет `agent/pkg/ipc` с протоколом
- [x] 2.2 Определить JSON схемы событий (activity, keyboard, heartbeat)
- [ ] 2.3 Pipe server в сервисе для приёма событий от helper
- [ ] 2.4 Pipe client в helper для отправки событий
- [ ] 2.5 Reconnection logic + backoff + health/heartbeat
- [ ] 2.6 **Chunked protocol для скриншотов**:
  - [ ] JSON meta: `screenshot_begin` (id, ts, size, mime, quality, window/process)
  - [ ] Binary chunks: `screenshot_chunk` (id, offset, data)
  - [ ] JSON commit: `screenshot_commit` (id, sha256)
  - [ ] Ack/err ответы (минимум): `ack`, `error`

Примечание: цель — убрать прямую отправку данных на сервер из helper.

### Этап 3: Унифицированный Session Helper (agent-sh.exe как артефакт)
- [x] 3.1 Создать `agent/cmd/session-helper/main.go`
- [x] 3.2 Перенести Activity Tracker в helper (GetForegroundWindow, GetLastInputInfo)
- [ ] 3.3 Перенести Keylogger hooks в helper (в план)
- [x] 3.4 Интегрировать Screenshot функциональность
- [ ] 3.5 **Убрать прямой HTTP из helper**: отправка только через PipeClient (включая скриншоты через chunked pipe)
- [x] 3.6 Graceful shutdown

### Этап 4: Обновление сервиса (единственная точка доставки)
- [x] 4.1 Удалить ActivityTracker из сервиса
- [x] 4.2 Обновить HelperProcess для запуска session-helper
- [ ] 4.3 PipeServer в сервисе (приём событий)
- [ ] 4.4 Интеграция PipeServer → EventBuffer → `/api/events/batch`
- [ ] 4.5 Мониторинг состояния helper (heartbeat + перезапуск при падении)
- [ ] 4.6 Валидировать `api_key` и обеспечивать его передачу на сервер **только из сервиса**

### Этап 5: Миграция и тестирование
- [ ] 5.1 После стабилизации удалить/задепрекейтить `cmd/screenshot-helper` (всё в универсальном helper)
- [x] 5.2 Обновить сборку (Makefile)
- [ ] 5.3 Тестирование на Windows 10/11 (console + service + RDP)
- [ ] 5.4 Обновить документацию (AGENT_SETUP.md, agent/README.md, WARP.md)

---

## Формат логов (JSON)

```json
{
  "ts": "2026-01-03T10:15:30.123+0300",
  "level": "info",
  "component": "activity_tracker",
  "session_id": "2",
  "username": "a-kiv",
  "msg": "Activity segment sent",
  "process": "chrome.exe",
  "state": "active",
  "duration_sec": 60,
  "total_sent": 15,
  "total_failed": 0
}
```

## Формат IPC событий

### Activity Event
```json
{
  "type": "activity_segment",
  "timestamp_start": "2026-01-03T10:15:00.000+0300",
  "timestamp_end": "2026-01-03T10:16:00.000+0300",
  "duration_sec": 60,
  "state": "active",
  "process_name": "chrome.exe",
  "window_title": "GitHub - Google Chrome",
  "username": "a-kiv",
  "session_id": "2"
}
```

### Keyboard Event
```json
{
  "type": "keyboard_event",
  "timestamp": "2026-01-03T10:15:30.000+0300",
  "process_name": "chrome.exe",
  "window_title": "GitHub - Google Chrome",
  "char_count": 150,
  "username": "a-kiv",
  "session_id": "2"
}
```

---

## Критерии готовности

1. Activity tracker показывает реальное состояние (active/idle/offline)
2. Foreground window определяется корректно
3. Все логи в JSON формате
4. Данные появляются в Grafana/отчётах
5. Единый helper вместо screenshot-helper
6. Документация обновлена

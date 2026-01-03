# План рефакторинга Windows Agent

## Статус: Этапы 1-3 выполнены

## Проблема

Windows сервис работает в Session 0 и не имеет доступа к UI пользователя:
- `GetForegroundWindow()` возвращает "unknown"
- `GetLastInputInfo()` возвращает данные Session 0, не пользователя
- Keylogger hooks не перехватывают ввод из пользовательской сессии

## Решение: Единый Session Helper

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

### Этап 1: Структурированное логирование (zap)
- [x] 1.1 Создать пакет `agent/pkg/logger` с zap конфигурацией
- [x] 1.2 JSON формат с полями: timestamp, level, component, session_id, message, error
- [ ] 1.3 Обновить сервис на использование нового логгера (отложено)
- [x] 1.4 Session helper использует новый логгер
- [x] 1.5 Ротация логов (lumberjack)

### Этап 2: IPC через Named Pipes
- [x] 2.1 Создать пакет `agent/pkg/ipc` с протоколом
- [x] 2.2 Определить JSON схемы событий (activity, keyboard, heartbeat)
- [ ] 2.3 Pipe server в сервисе для приёма событий от helpers (отложено)
- [ ] 2.4 Pipe client в helper для отправки событий (отложено)
- [ ] 2.5 Reconnection logic и health checks (отложено)

Примечание: IPC отложен — session-helper отправляет данные напрямую на сервер.

### Этап 3: Унифицированный Session Helper
- [x] 3.1 Создать `agent/cmd/session-helper/main.go`
- [x] 3.2 Перенести Activity Tracker в helper (GetForegroundWindow, GetLastInputInfo)
- [ ] 3.3 Перенести Keylogger в helper (отложено на будущее)
- [x] 3.4 Интегрировать Screenshot функциональность
- [x] 3.5 Отправка событий напрямую на сервер через HTTP
- [x] 3.6 Graceful shutdown

### Этап 4: Обновление сервиса
- [x] 4.1 Удалить ActivityTracker из сервиса
- [x] 4.2 Обновить HelperProcess для запуска session-helper
- [ ] 4.3 Добавить приём событий через Named Pipe (отложено)
- [ ] 4.4 Интеграция с существующим Event Buffer (отложено)
- [ ] 4.5 Мониторинг состояния helpers (частично)

### Этап 5: Миграция и тестирование
- [ ] 5.1 Можно удалить старый screenshot-helper после тестирования
- [x] 5.2 Обновить сборку (Makefile)
- [ ] 5.3 Тестирование на Windows 10/11
- [ ] 5.4 Документация (AGENT_SETUP.md)

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

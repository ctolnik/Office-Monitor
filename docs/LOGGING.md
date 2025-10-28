# Рекомендации по логированию в Office-Monitor

## Архитектура логирования

Проект использует **structured logging** через библиотеку `zap` с кастомным пакетом `zapctx` для передачи логгера через context.

### Основные компоненты:

1. **zap** - быстрый структурированный логгер
2. **zapctx** - утилита для работы с логгером через context
3. **gin-contrib/zap** - интеграция с Gin framework
4. **Request ID** - уникальный идентификатор для каждого HTTP запроса

## Принципы работы

### 1. Используй контекст из Gin, а НЕ context.Background()

❌ **НЕПРАВИЛЬНО:**
```go
func handler(c *gin.Context) {
    ctx := context.Background()  // Теряется логгер и request_id!
    db.Query(ctx, ...)
}
```

✅ **ПРАВИЛЬНО:**
```go
func handler(c *gin.Context) {
    ctx := c.Request.Context()  // Содержит логгер с request_id
    db.Query(ctx, ...)
}
```

### 2. Структурированное логирование

Всегда используй структурированные поля вместо форматированных строк:

❌ **НЕПРАВИЛЬНО:**
```go
log.Printf("User %s from %s failed", username, computer)
```

✅ **ПРАВИЛЬНО:**
```go
zapctx.Error(ctx, "User authentication failed",
    zap.String("username", username),
    zap.String("computer_name", computer),
)
```

### 3. Уровни логирования

- **Debug** - детальная информация для отладки (query parameters, промежуточные значения)
- **Info** - нормальные операции (успешные запросы, старт/стоп сервисов)
- **Warn** - подозрительные ситуации (медленные запросы, retry, deprecated API)
- **Error** - ошибки требующие внимания (failed queries, connection errors)

### 4. Метрики производительности

В database layer автоматически логируются:
- Время выполнения каждого запроса
- Warning при превышении порогов (100ms для INSERT, 200ms для SELECT)

```go
start := time.Now()
err := db.conn.Exec(ctx, query, args...)
duration := time.Since(start)

if duration > threshold {
    zapctx.Warn(ctx, "Slow query detected",
        zap.Duration("duration", duration),
        zap.String("table", "activity_events"),
    )
}
```

## Примеры использования

### HTTP Handler

```go
func receiveActivityHandler(c *gin.Context) {
    ctx := c.Request.Context()  // Получаем контекст с логгером
    var event database.ActivityEvent
    
    if err := c.ShouldBindJSON(&event); err != nil {
        zapctx.Warn(ctx, "Invalid activity request",
            zap.Error(err),
            zap.String("remote_addr", c.ClientIP()),
        )
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
        return
    }
    
    zapctx.Debug(ctx, "Inserting activity event",
        zap.String("computer_name", event.ComputerName),
        zap.String("username", event.Username),
    )
    
    if err := db.InsertActivityEvent(ctx, event); err != nil {
        zapctx.Error(ctx, "Failed to insert activity event",
            zap.Error(err),
            zap.String("computer_name", event.ComputerName),
        )
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save"})
        return
    }
    
    zapctx.Info(ctx, "Activity event saved successfully",
        zap.String("computer_name", event.ComputerName),
    )
    c.JSON(http.StatusOK, gin.H{"status": "success"})
}
```

### Database Layer

```go
func (db *Database) InsertActivityEvent(ctx context.Context, event ActivityEvent) error {
    query := `INSERT INTO monitoring.activity_events ...`
    
    start := time.Now()
    err := db.conn.Exec(ctx, query, args...)
    duration := time.Since(start)
    
    if err != nil {
        zapctx.Error(ctx, "Failed to insert to ClickHouse",
            zap.Error(err),
            zap.Duration("duration", duration),
            zap.String("table", "activity_events"),
        )
        return err
    }
    
    // Предупреждение о медленных запросах
    if duration > 100*time.Millisecond {
        zapctx.Warn(ctx, "Slow INSERT detected",
            zap.Duration("duration", duration),
            zap.String("table", "activity_events"),
        )
    }
    
    return nil
}
```

### Background Task / Goroutine

Если создаешь goroutine, передавай контекст или создай новый с логгером:

```go
func processBatch(parentCtx context.Context, items []Item) {
    // Добавь метаданные для трейсинга
    ctx := zapctx.WithFields(parentCtx,
        zap.String("batch_id", uuid.New().String()),
        zap.Int("batch_size", len(items)),
    )
    
    zapctx.Info(ctx, "Starting batch processing")
    
    for _, item := range items {
        processItem(ctx, item)
    }
    
    zapctx.Info(ctx, "Batch processing completed")
}
```

## Request ID Tracing

Каждый HTTP запрос получает уникальный `request_id`:

1. Клиент может передать свой ID через заголовок `X-Request-ID`
2. Если заголовок отсутствует, генерируется UUID
3. ID добавляется в каждый лог этого запроса
4. ID возвращается клиенту в response header

Пример лога:
```json
{
  "level": "info",
  "timestamp": "2025-10-26T23:00:00.000Z",
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "method": "POST",
  "path": "/api/activity",
  "msg": "Activity event saved successfully",
  "computer_name": "WS-001",
  "username": "john.doe"
}
```

## Конфигурация логгера

### Development Mode
- Console output с цветными уровнями
- Включены stacktrace для всех ошибок
- Читаемый формат для человека

### Production Mode
- JSON output для агрегаторов (ELK, Splunk)
- Stacktrace только для Error/Fatal
- ISO8601 timestamp
- Оптимизирован по производительности

Переключение через конфиг:
```yaml
server:
  mode: "release"  # или "debug"
```

## Чего НЕ нужно логировать

❌ **Не логируй sensitive data:**
- Пароли, API ключи, токены
- Полный текст из keyboard_events (если включен keylogger)
- Личные данные без маскирования

❌ **Не логируй на уровне Info/Debug:**
- Каждый успешный запрос (для этого есть ginzap middleware)
- Тело больших JSON payload
- Результаты SELECT без необходимости

✅ **Логируй:**
- Ошибки и их контекст
- Медленные запросы
- Важные business events (новый агент, изменение конфига)
- Метрики (количество обработанных записей)

## Best Practices

1. **Всегда передавай context** от Gin handlers до database layer
2. **Используй структурированные поля** (zap.String, zap.Int, zap.Error)
3. **Добавляй duration** для всех операций с внешними системами
4. **Логируй на правильном уровне** (не все Warning, не все Debug)
5. **Добавляй контекстную информацию** (computer_name, username, table)
6. **НЕ используй fmt.Printf или log.Printf** - только zapctx

## Monitoring & Alerts

В будущем можно добавить:
- Экспорт метрик в Prometheus (количество slow queries, error rate)
- Интеграция с ELK stack для агрегации логов
- Алерты на основе error rate по request_id
- Distributed tracing с OpenTelemetry

## Пример полного flow

```
HTTP Request → Gin Middleware (request_id) 
           → Gin Middleware (logger with request_id)
           → Handler (ctx = c.Request.Context())
           → Database Layer (ctx передается дальше)
           → ClickHouse Query (логируется с request_id)
           → Response (request_id в header)
```

Все логи этого запроса будут иметь одинаковый `request_id`, что позволяет легко трейсить весь путь запроса через систему.

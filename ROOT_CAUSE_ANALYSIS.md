# 🔍 Root Cause: Почему отчёт пустой

## ПРОБЛЕМА №1: USERNAME НЕ СОВПАДАЕТ ❌

### Из логов агента (строка 2):
```
Computer: ADM-01, User: a-kiv
```

### Frontend ищет:
```
URL: /reports/daily/a.kly
Username: "a.kly"  ❌ НЕПРАВИЛЬНО!
```

### В БД реально:
```
Username: "a-kiv"  ✅ (из логов агента)
```

**ЭТО РАЗНЫЕ ПОЛЬЗОВАТЕЛИ!**

---

## ПРОБЛЕМА №2: Activity Tracker использует НЕПРАВИЛЬНЫЙ HTTP Client

### Код в activity_tracker_windows.go (строка 63-75):

```go
func NewActivityTracker(...) *ActivityTracker {
    return &ActivityTracker{
        ...
        client: &http.Client{  // ❌ Стандартный http.Client!
            Timeout: 30 * time.Second,
        },
    }
}
```

### Код в sendSegment (строка 211):

```go
resp, err := at.client.Post(url, "application/json", bytes.NewBuffer(data))
if err != nil {
    log.Printf("Failed to send activity segment: %v", err)
    return
}
defer resp.Body.Close()

if resp.StatusCode != http.StatusOK {
    log.Printf("Server returned non-OK status for activity segment: %d", resp.StatusCode)
}
// ❌ НЕТ логирования успеха!
```

**Проблемы:**
1. ❌ Использует стандартный http.Client вместо нашего httpclient (с circuit breaker)
2. ❌ Не логирует успешные запросы
3. ❌ Не использует circuit breaker (может спамить при недоступности сервера)

### Сравните со screenshot_windows.go (правильно):

```go
func NewScreenshotMonitor(..., httpClient *httpclient.Client) *ScreenshotMonitor {
    return &ScreenshotMonitor{
        ...
        client: httpClient,  // ✅ Использует наш httpclient!
    }
}

// При отправке:
err := sm.client.PostMultipart(...)  // ✅ С логированием и circuit breaker
```

---

## ✅ РЕШЕНИЯ:

### Решение №1: Исправить username в frontend

**Краткосрочное:** Открыть отчёт с правильным username
```
URL: /reports/daily/a-kiv  (не a.kly!)
```

**Долгосрочное:** Frontend должен брать список username из API:
```
GET /api/users  → возвращает реальные username из БД
```

### Решение №2: Исправить Activity Tracker

**Изменить agent/monitoring/activity_tracker_windows.go:**

1. Принимать httpclient в конструкторе (как screenshot)
2. Использовать client.PostJSON вместо http.Post
3. Удалить создание собственного http.Client

**Изменить agent/main.go:**

Передать httpClient в NewActivityTracker:
```go
activityTracker = monitoring.NewActivityTracker(
    httpClient,  // ✅ Добавить!
    cfg.Agent.ComputerName,
    os.Getenv("USERNAME"),
    ...
)
```

---

## 🎯 Приоритет исправлений:

1. **СРОЧНО:** Проверить отчёт для username "a-kiv" → проверит есть ли данные в БД
2. **ВАЖНО:** Исправить Activity Tracker использовать httpclient
3. **ЖЕЛАТЕЛЬНО:** Frontend должен брать username из API


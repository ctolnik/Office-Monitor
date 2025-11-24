# API Specification for Frontend Development (lovable.dev)

## Обзор

Это полная спецификация REST API для фронтенда системы мониторинга сотрудников.  
Backend работает на **Go + ClickHouse + MinIO** и предоставляет **50 REST endpoints**.

**Base URL**: `http://your-server:5000/api`

---

## 🔴 КРИТИЧЕСКИ ВАЖНЫЕ ИЗМЕНЕНИЯ (от 24.11.2025)

### ✅ ДОБАВЛЕНЫ НЕДОСТАЮЩИЕ ENDPOINTS

#### 1. GET /api/users
**Новый endpoint для получения списка пользователей**

**Query params**: None

**Response**:
```json
["user1", "user2", "user3"]
```

**Использование**: Заполнение dropdown "Выберите пользователя" в Reports/Activity pages

---

#### 2. GET /api/settings/app-categories
**Alias для `/api/categories` (совместимость с фронтендом)**

**Query params**: None

**Response**: Такой же как `/api/categories`

---

#### 3. GET /api/activity/segments ⚠️ ТОЛЬКО ЧТО ДОБАВЛЕН
**Timeline visualization - получение сегментов активности**

**Query params**:
- `computer_name` (required) - имя компьютера
- `date` (optional) - дата в формате `YYYY-MM-DD`, по умолчанию сегодня

**Request example**:
```http
GET /api/activity/segments?computer_name=PC001&date=2025-11-23
```

**Response**:
```typescript
interface ActivitySegment {
  timestamp_start: string;         // "2025-11-23T09:15:00Z"
  timestamp_end: string;           // "2025-11-23T09:45:00Z"
  duration_sec: number;            // 1800 (30 мин)
  state: "active" | "idle" | "offline";
  process_name: string;            // "chrome.exe"
  window_title: string;            // "Почта — mail.yandex.ru"
  computer_name: string;           // "PC001"
  username: string;                // "ivanov"
}

type ActivitySegmentsResponse = ActivitySegment[];
```

**Использование**: Построение timeline chart (визуализация активности по часам)

**Пример обработки**:
```typescript
// Группировка сегментов по 30-минутным интервалам для timeline
function buildTimeline(segments: ActivitySegment[]) {
  const intervals = Array.from({ length: 48 }, (_, i) => {
    const hour = Math.floor(i / 2);
    const minute = (i % 2) * 30;
    return {
      time: `${hour.toString().padStart(2, '0')}:${minute.toString().padStart(2, '0')}`,
      state: 'offline' as const,
      program: '',
    };
  });
  
  // Map segments to intervals...
  segments.forEach(seg => {
    const startTime = new Date(seg.timestamp_start);
    const intervalIndex = startTime.getHours() * 2 + (startTime.getMinutes() >= 30 ? 1 : 0);
    
    if (intervalIndex < intervals.length) {
      intervals[intervalIndex].state = seg.state;
      intervals[intervalIndex].program = seg.process_name;
    }
  });
  
  return intervals;
}
```

---

## 📊 ПОЛНЫЙ СПИСОК API ENDPOINTS (50 шт)

### Activity Tracking (6 endpoints)

#### POST /api/activity
Прием event'а активности от агента

#### POST /api/events/batch
Batch загрузка events (до 10000 за раз)

#### POST /api/activity/segment
Прием сегмента активности (с состоянием: active/idle/offline)

#### GET /api/activity/recent
Последние события активности

#### GET /api/activity/summary
**Агрегированный отчет за день**

**Query params**:
- `computer_name` (required)
- `date` (optional) - YYYY-MM-DD

**Response**:
```typescript
interface DailyActivitySummary {
  date: string;                    // "2025-11-23"
  computer_name: string;           // "PC001"
  username: string;                // "ivanov"
  active_seconds: number;          // 16080 (4ч 28мин)
  idle_seconds: number;            // 21840 (6ч 4мин)
  offline_seconds: number;         // 48600 (13ч 30мин)
  top_programs: ProgramUsage[];    // Топ-10 программ
}

interface ProgramUsage {
  process_name: string;            // "chrome.exe"
  friendly_name: string;           // "chrome.exe" (пока = process_name)
  duration_sec: number;            // 8100 (2ч 15мин)
  window_titles?: string[];        // ["Почта — mail.yandex.ru", ...]
}
```

⚠️ **ВАЖНО**: `friendly_name` **пока всегда равен `process_name`**. Backend еще не реализовал JOIN с `process_catalog`.

⚠️ **КРИТИЧНО**: Endpoint может вернуть **500 Internal Server Error** если ClickHouse materialized views не инициализированы. Обрабатывайте эту ошибку!

#### GET /api/activity/segments
См. выше (только что добавлен) ⬆️

---

### Dashboard (3 endpoints)

#### GET /api/dashboard/stats
Общая статистика системы

**Response**:
```typescript
{
  total_employees: number;
  active_now: number;
  alerts_count: number;
  // ... другие метрики
}
```

#### GET /api/dashboard/active-now
Список активных сейчас сотрудников

#### GET /api/reports/daily/:username
Дневной отчет для пользователя

**Path params**: `username`

---

### Employees (5 endpoints)

#### GET /api/employees
Список сотрудников (базовый)

#### GET /api/employees/all
Полный список с метаданными

#### POST /api/employees
Создать сотрудника

**Request**:
```json
{
  "computer_name": "PC001",
  "username": "ivanov"
}
```

#### PUT /api/employees/:id
Обновить сотрудника

#### DELETE /api/employees/:id
Удалить сотрудника

---

### Users (1 endpoint) ✅ НОВЫЙ

#### GET /api/users
Уникальные usernames за последние 7 дней

**Response**: `["user1", "user2", "user3"]`

---

### Process Catalog (4 endpoints)

Справочник программ для mapping `process_name` → `friendly_name`

#### GET /api/process-catalog
Получить все записи

**Response**:
```typescript
interface ProcessCatalogEntry {
  id: string;
  friendly_name: string;           // "Google Chrome"
  process_names: string[];         // ["chrome.exe"]
  window_title_patterns: string[]; // ["*mail.yandex.ru*"]
  category: string;                // "browsing" | "work" | "communication" | "development" | "other"
  is_active: boolean;
  created_at: string;              // ISO timestamp
  updated_at: string;              // ISO timestamp
}
```

#### POST /api/process-catalog
Создать запись

**Request**:
```json
{
  "friendly_name": "Google Chrome",
  "process_names": ["chrome.exe"],
  "window_title_patterns": [],
  "category": "browsing"
}
```

#### PUT /api/process-catalog/:id
Обновить запись

#### DELETE /api/process-catalog/:id
Удалить запись

---

### Application Categories (7 endpoints)

Категории приложений для классификации программ

#### GET /api/categories
Список всех категорий

#### GET /api/settings/app-categories ✅ НОВЫЙ
Alias для `/api/categories`

#### POST /api/categories
Создать категорию

#### PUT /api/categories/:id
Обновить категорию

#### DELETE /api/categories/:id
Удалить категорию

#### POST /api/categories/bulk
Массовое обновление категорий

#### GET /api/categories/export
Экспорт в JSON

#### POST /api/categories/import
Импорт из JSON

---

### USB Monitoring (3 endpoints)

#### POST /api/usb/event
Прием USB event от агента

#### GET /api/usb/events
Все USB события

#### GET /api/usb/:username
USB события для пользователя

---

### File Monitoring (3 endpoints)

#### POST /api/file/event
Прием file copy event

#### GET /api/file/events
Все file events

#### GET /api/files/:username
File events для пользователя

---

### Screenshots (3 endpoints)

#### POST /api/screenshot
Upload screenshot (multipart/form-data)

#### GET /api/screenshots/:username
Список скриншотов пользователя

#### GET /api/screenshots/file/:id
Скачать скриншот (image/jpeg)

---

### Keyboard Events (3 endpoints)

⚠️ **GDPR WARNING**: Требует согласие сотрудника

#### POST /api/keyboard/event
Прием keyboard event

#### GET /api/keyboard/events
Все keyboard events

#### GET /api/keyboard/:username
Keyboard events для пользователя

---

### Alerts (3 endpoints)

DLP алерты и уведомления

#### GET /api/alerts
Все алерты

#### GET /api/alerts/unresolved
Нерешенные алерты

#### PUT /api/alerts/:id/resolve
Закрыть алерт

**Request**:
```json
{
  "resolved_by": "admin",
  "notes": "False positive"
}
```

---

### Agents (4 endpoints)

Управление агентами на рабочих станциях

#### GET /api/agents
Список агентов

#### GET /api/agents/:computer_name/config
Конфигурация агента

#### POST /api/agents/:computer_name/config
Обновить конфигурацию

**Request**:
```json
{
  "screenshot_enabled": true,
  "screenshot_interval_minutes": 15,
  "keylogger_enabled": false,
  "usb_monitoring_enabled": true
}
```

#### DELETE /api/agents/:computer_name
Удалить агента

---

### Settings (3 endpoints)

Общие настройки системы

#### GET /api/settings
Получить настройки

#### PUT /api/settings
Обновить настройки

#### POST /api/settings/logo
Upload лого (multipart/form-data)

---

## 🔧 ВАЖНЫЕ ДЕТАЛИ РЕАЛИЗАЦИИ

### Обработка ошибок

```typescript
// ОБЯЗАТЕЛЬНО обрабатывайте 500 ошибки от ClickHouse
const { data, error } = await fetch('/api/activity/summary?...');

if (error?.status === 500) {
  // ClickHouse materialized views не инициализированы
  showError('База данных не готова. Обратитесь к администратору');
}
```

### Формат даты

Везде используется **ISO 8601**: `YYYY-MM-DD` для дат, `YYYY-MM-DDTHH:MM:SSZ` для timestamps

### Пагинация

⚠️ **Пока НЕ РЕАЛИЗОВАНА**. Все endpoints возвращают полные списки.

### CORS

Backend настроен на `Access-Control-Allow-Origin: *` (для development)

---

## 📝 ИЗВЕСТНЫЕ ОГРАНИЧЕНИЯ

1. **Friendly names НЕ РАБОТАЮТ** (см. выше ⬆️)
2. **Materialized views могут быть не готовы** → 500 error
3. **Нет пагинации** для больших списков
4. **Нет WebSocket** → используйте polling для real-time updates

---

## 🎯 РЕКОМЕНДАЦИИ ДЛЯ FRONTEND

### Priority 1 (MVP)
- ✅ Страница Activity Report с фильтрами
- ✅ Summary cards (active/idle/offline)
- ✅ Top Programs table
- ✅ Timeline chart (используйте `/api/activity/segments`) ⬅️ НОВЫЙ ENDPOINT

### Priority 2
- Process Catalog admin panel (CRUD)
- Alerts management
- Employee management

### Priority 3
- Screenshots viewer
- USB/File events timeline
- Advanced filters and search

---

## 📞 КОНТАКТЫ

При обнаружении багов или недостающих endpoints - сообщите backend команде.

**Дата последнего обновления**: 24 ноября 2025  
**Версия API**: 1.0  
**Количество endpoints**: 50

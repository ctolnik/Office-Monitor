# 🎨 Задание для lovable.dev: Цвета диаграмм по категориям

## Текущая проблема:
Круговые диаграммы показывают все приложения **одним цветом** (серым).

## Требуемое решование:
Диаграммы должны раскрашиваться **по категориям приложений**:

### Цветовая схема по категориям:
```typescript
const CATEGORY_COLORS = {
  productive: '#22c55e',     // Зелёный
  unproductive: '#ef4444',   // Красный  
  neutral: '#94a3b8',        // Серый
  communication: '#3b82f6',  // Синий
  entertainment: '#f59e0b'   // Оранжевый
}
```

---

## API Response (что backend уже отдаёт):

### `/api/reports/daily/{username}?date=YYYY-MM-DD`

```json
{
  "applications": [
    {
      "process_name": "chrome.exe",
      "window_title": "GitHub - Google Chrome",
      "duration": 3600,
      "count": 12,
      "category": "neutral"     ← Категория есть!
    },
    {
      "process_name": "code.exe",
      "window_title": "Visual Studio Code",
      "duration": 7200,
      "count": 45,
      "category": "productive"  ← Категория есть!
    }
  ],
  "summary": {
    "productive_time": 7200,      ← Суммарное продуктивное время
    "unproductive_time": 1800,
    "neutral_time": 5400
  }
}
```

---

## Задачи для frontend:

### 1. **Круговая диаграмма "Applications" (по приложениям)**

**Текущее поведение:** Все сегменты серые

**Новое поведение:** 
- Цвет сегмента = цвет категории приложения
- chrome.exe (neutral) → серый
- code.exe (productive) → зелёный
- youtube.com (unproductive) → красный

**Код (примерно):**
```typescript
const appChartData = applications.map(app => ({
  name: app.process_name,
  value: app.duration,
  color: CATEGORY_COLORS[app.category] || '#94a3b8'
}))
```

---

### 2. **Круговая диаграмма "Productivity" (по категориям)**

**Добавить новую диаграмму** или **изменить существующую**:

**Данные для диаграммы:**
```typescript
const productivityData = [
  { 
    name: 'Productive', 
    value: summary.productive_time,
    color: '#22c55e'
  },
  { 
    name: 'Unproductive', 
    value: summary.unproductive_time,
    color: '#ef4444'
  },
  { 
    name: 'Neutral', 
    value: summary.neutral_time,
    color: '#94a3b8'
  }
]
```

---

### 3. **Легенда диаграмм**

Добавить легенду с объяснением категорий:
- 🟢 Productive - Работа (IDE, Office, Email)
- 🔴 Unproductive - Развлечения (YouTube, соц. сети)
- ⚪ Neutral - Браузеры и прочее
- 🔵 Communication - Мессенджеры (Teams, Slack)

---

### 4. **Fallback (если категория отсутствует)**

Если `app.category === 'neutral'` или `undefined`:
- Используйте уникальные цвета для каждого приложения (как сейчас)
- Или серый цвет для всех

---

### 5. **Процент продуктивности**

Показать KPI "Productivity Score":
```typescript
const productivityPercent = (summary.productive_time / summary.total_active_time) * 100

// Отобразить как процент или цветной индикатор:
<Badge color={productivityPercent > 50 ? 'green' : 'red'}>
  {productivityPercent.toFixed(0)}% Productive
</Badge>
```

---

## Тестовые данные:

Для тестирования используйте mock данные:
```json
{
  "applications": [
    {"process_name": "code.exe", "duration": 14400, "category": "productive"},
    {"process_name": "chrome.exe", "duration": 7200, "category": "neutral"},
    {"process_name": "youtube.com", "duration": 1800, "category": "unproductive"},
    {"process_name": "teams.exe", "duration": 3600, "category": "communication"}
  ],
  "summary": {
    "total_active_time": 27000,
    "productive_time": 14400,
    "unproductive_time": 1800,
    "neutral_time": 10800
  }
}
```

---

## Приоритеты:

1. ✅ **Высокий:** Цвета по категориям в круговой диаграмме Applications
2. ✅ **Высокий:** Добавить диаграмму Productivity (productive/unproductive/neutral)
3. ✅ **Средний:** Легенда категорий
4. ✅ **Средний:** KPI "Productivity Score"
5. ⚠️ **Низкий:** Fallback цвета если категория отсутствует

---

## Примеры UI:

### Круговая диаграмма Applications:
```
   🟢 code.exe (14400s)
   ⚪ chrome.exe (7200s)
   🔴 youtube.com (1800s)
   🔵 teams.exe (3600s)
```

### Круговая диаграмма Productivity:
```
   🟢 Productive (53%)
   ⚪ Neutral (40%)
   🔴 Unproductive (7%)
```

---

## Готовый код (псевдокод):

```typescript
// Цвета категорий
const CATEGORY_COLORS = {
  productive: '#22c55e',
  unproductive: '#ef4444',
  neutral: '#94a3b8',
  communication: '#3b82f6',
  entertainment: '#f59e0b'
}

// Данные для диаграммы Applications
const appChartData = report.applications.map(app => ({
  name: app.process_name.replace('.exe', ''),
  value: app.duration,
  fill: CATEGORY_COLORS[app.category] || '#94a3b8'
}))

// Данные для диаграммы Productivity
const productivityChartData = [
  { name: 'Productive', value: report.summary.productive_time, fill: '#22c55e' },
  { name: 'Unproductive', value: report.summary.unproductive_time, fill: '#ef4444' },
  { name: 'Neutral', value: report.summary.neutral_time, fill: '#94a3b8' }
]

// Productivity Score
const productivityScore = (report.summary.productive_time / report.summary.total_active_time) * 100
```

---

Реализуйте эти изменения используя существующие UI компоненты Recharts или Chart.js.


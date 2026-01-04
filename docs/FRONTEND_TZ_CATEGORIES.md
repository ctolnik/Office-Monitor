# Техническое задание: Рефакторинг управления категориями приложений

## Контекст проекта
**Приложение:** Office Monitor — система мониторинга активности сотрудников  
**Frontend:** React + TypeScript + shadcn/ui (git submodule в `frontend/`)  
**Backend API:** Go + Gin (http://monitor.net.gslaudit.ru/api)  
**Цель:** Устранить дублирование функционала между "Справочник программ" и "Категории приложений", реализовать пользовательский CRUD для категорий.

---

## Проблема
Сейчас в UI есть две страницы для управления категоризацией приложений:
1. **"Справочник программ"** (`/settings` → Process Catalog) — правила маппинга процессов к категориям (friendly_name, process_names[], window_title_patterns[], category)
2. **"Категории приложений"** (`/settings` → Application Categories) — правила маппинга процессов к категориям (process_name, process_pattern, category)

Обе страницы делают одно и то же, что вносит путаницу. В backend приоритет у "Справочник программ", а "Категории приложений" используется как fallback.

**Текущая проблема категорий:**
- Категории (productive, unproductive, neutral, communication, entertainment) зашиты в Enum на backend
- Нельзя добавить новую категорию без изменения кода backend
- UI отправляет строковые значения категорий напрямую (hardcode в компонентах)

---

## Целевая архитектура

### Разделение ответственности
1. **"Категории приложений"** — теперь CRUD для **списка категорий** (справочник значений):
   - Создание/редактирование/удаление категорий (например: productive, unproductive, neutral, communication, entertainment, system)
   - Категория = `{id: UUID, key: string, name: string, color?: string, sort_order?: number, is_active: boolean}`
   - **Никакого маппинга к приложениям** на этой странице

2. **"Справочник программ"** — единственное место для **правил категоризации**:
   - Создание/редактирование/удаление правил
   - Поле "Категория" теперь выбирается из списка доступных категорий (dropdown/select)
   - Правило = `{id, friendly_name, process_names[], window_title_patterns[], category_id: UUID, is_active}`

---

## Изменения в Backend API

### Новый эндпоинт: Category Types (список категорий)
```
GET    /api/category-types        # Список всех категорий
POST   /api/category-types        # Создать категорию
PUT    /api/category-types/:id    # Обновить категорию
DELETE /api/category-types/:id    # Удалить (soft delete)
```

**Формат данных:**
```typescript
interface CategoryType {
  id: string;              // UUID
  key: string;             // slug (productive, unproductive, neutral, system, ...)
  name: string;            // Отображаемое имя ("Продуктивно", "Непродуктивно", ...)
  color?: string;          // Цвет для UI (опционально)
  sort_order?: number;     // Порядок отображения (опционально)
  is_active: boolean;      // Активна ли категория
  created_at: string;      // ISO timestamp
  updated_at: string;      // ISO timestamp
}
```

**Пример GET /api/category-types:**
```json
{
  "data": [
    {"id": "uuid-1", "key": "productive", "name": "Продуктивно", "color": "#10b981", "sort_order": 1, "is_active": true, "created_at": "...", "updated_at": "..."},
    {"id": "uuid-2", "key": "unproductive", "name": "Непродуктивно", "color": "#ef4444", "sort_order": 2, "is_active": true, "created_at": "...", "updated_at": "..."},
    {"id": "uuid-3", "key": "neutral", "name": "Нейтрально", "color": "#6b7280", "sort_order": 3, "is_active": true, "created_at": "...", "updated_at": "..."},
    {"id": "uuid-4", "key": "communication", "name": "Коммуникации", "color": "#3b82f6", "sort_order": 4, "is_active": true, "created_at": "...", "updated_at": "..."},
    {"id": "uuid-5", "key": "entertainment", "name": "Развлечения", "color": "#a855f7", "sort_order": 5, "is_active": true, "created_at": "...", "updated_at": "..."},
    {"id": "uuid-6", "key": "system", "name": "Системная", "color": "#64748b", "sort_order": 6, "is_active": true, "created_at": "...", "updated_at": "..."}
  ],
  "total": 6
}
```

### Изменения в Process Catalog API
```
GET    /api/process-catalog        # Список правил
POST   /api/process-catalog        # Создать правило
PUT    /api/process-catalog/:id    # Обновить правило
DELETE /api/process-catalog/:id    # Удалить правило
```

**Изменения в формате данных:**
```typescript
interface ProcessCatalogEntry {
  id: string;                       // UUID
  friendly_name: string;            // Отображаемое имя приложения
  process_names: string[];          // Список имён процессов (["chrome.exe", "msedge.exe"])
  window_title_patterns: string[];  // Паттерны заголовков окон
  category_id: string;              // UUID категории (НОВОЕ ПОЛЕ вместо category: string)
  category?: CategoryType;          // Вложенный объект категории (для отображения в UI)
  is_active: boolean;
  created_at: string;
  updated_at: string;
}
```

**Пример GET /api/process-catalog:**
```json
{
  "data": [
    {
      "id": "rule-uuid-1",
      "friendly_name": "Visual Studio Code",
      "process_names": ["code.exe"],
      "window_title_patterns": [],
      "category_id": "uuid-1",
      "category": {
        "id": "uuid-1",
        "key": "productive",
        "name": "Продуктивно",
        "color": "#10b981"
      },
      "is_active": true,
      "created_at": "...",
      "updated_at": "..."
    }
  ],
  "total": 1
}
```

### Удалённый эндпоинт
```
/api/categories  # Удалить все маршруты (GET/POST/PUT/DELETE)
```
Этот эндпоинт больше не используется. Если frontend обращается к нему — заменить на `/api/category-types` или `/api/process-catalog`.

---

## Требования к Frontend

### 1. Страница "Категории приложений" (Category Types Management)

**Путь:** `/settings/categories` (или текущий путь, где была страница "Категории приложений")

**Функционал:**
- **Список категорий** (таблица/список):
  - Колонки: Name (с индикатором цвета), Key, Active status, Actions
  - Сортировка по `sort_order` (если есть) или по `name`
  - Фильтр: показать только активные / показать все
- **Создание категории:**
  - Кнопка "+ Добавить категорию"
  - Форма: Name (required), Key (required, lowercase, no spaces), Color (optional, color picker), Sort Order (optional, number)
  - Валидация:
    - `key` должен быть уникальным
    - `key` должен быть lowercase, только буквы/цифры/дефисы
    - `name` — обязательное поле
  - При успешном создании — обновить список
- **Редактирование категории:**
  - Кнопка "Редактировать" в строке
  - Форма аналогична созданию (все поля редактируемые)
  - При успешном обновлении — обновить список
- **Удаление категории (soft delete):**
  - Кнопка "Удалить" в строке
  - Подтверждение: "Вы уверены? Правила, использующие эту категорию, не будут удалены, но могут отображаться некорректно."
  - После удаления категория помечается `is_active = false` и исчезает из списка (если фильтр "только активные")
- **UI/UX:**
  - Использовать shadcn/ui компоненты: Table, Dialog, Form, Button, Input, Select
  - Цветовой индикатор категории: маленький кружок/badge с `background-color: category.color`
  - Пустое состояние: если категорий нет, показать placeholder с кнопкой "Создать первую категорию"

**API интеграция:**
```typescript
// Получение списка категорий
const { data: categories } = useQuery({
  queryKey: ['category-types'],
  queryFn: () => api.get('/api/category-types').then(res => res.data.data)
});

// Создание категории
const createMutation = useMutation({
  mutationFn: (newCategory: Omit<CategoryType, 'id' | 'created_at' | 'updated_at'>) =>
    api.post('/api/category-types', newCategory),
  onSuccess: () => queryClient.invalidateQueries(['category-types'])
});

// Обновление категории
const updateMutation = useMutation({
  mutationFn: ({ id, data }: { id: string; data: Partial<CategoryType> }) =>
    api.put(`/api/category-types/${id}`, data),
  onSuccess: () => queryClient.invalidateQueries(['category-types'])
});

// Удаление категории
const deleteMutation = useMutation({
  mutationFn: (id: string) => api.delete(`/api/category-types/${id}`),
  onSuccess: () => queryClient.invalidateQueries(['category-types'])
});
```

---

### 2. Страница "Справочник программ" (Process Catalog)

**Путь:** `/settings/process-catalog` (или текущий путь)

**Изменения:**
- **Поле "Категория" в форме создания/редактирования:**
  - Заменить текущее поле (если там select с hardcoded значениями или input) на **Select/Dropdown**, который грузит список из `/api/category-types`
  - Отображать `category.name` (например, "Продуктивно")
  - Отправлять `category_id` (UUID) в POST/PUT запросах
  - Показывать цветовой индикатор рядом с выбранной категорией
  - Если категорий нет — показать предупреждение: "Сначала создайте категории в разделе 'Категории приложений'"
- **Список правил (таблица):**
  - Колонка "Категория" должна отображать `category.name` с цветовым индикатором
  - Если `category` отсутствует (null/undefined) — показать "—" или "Не указано"
- **Валидация:**
  - При создании/редактировании правила поле `category_id` обязательно
  - Если выбранная категория была удалена (не найдена в списке) — показать warning

**API интеграция:**
```typescript
// Получение списка правил
const { data: rules } = useQuery({
  queryKey: ['process-catalog'],
  queryFn: () => api.get('/api/process-catalog').then(res => res.data.data)
});

// Создание правила
const createMutation = useMutation({
  mutationFn: (newRule: { 
    friendly_name: string; 
    process_names: string[]; 
    window_title_patterns: string[]; 
    category_id: string;  // UUID, а не строка типа "productive"
    is_active: boolean;
  }) => api.post('/api/process-catalog', newRule),
  onSuccess: () => queryClient.invalidateQueries(['process-catalog'])
});

// Обновление правила (аналогично)
```

---

### 3. Удаление/рефакторинг старого кода "Категории приложений"

**Если** в коде есть компоненты/файлы, которые работают с `/api/categories` (старый эндпоинт):
- Найти все обращения к `/api/categories` (GET/POST/PUT/DELETE)
- Удалить или переписать под новую логику:
  - Если это был CRUD категорий — заменить на `/api/category-types`
  - Если это был CRUD правил — заменить на `/api/process-catalog`
- Удалить hardcoded списки категорий из кода (например, `const categories = ['productive', 'unproductive', ...]`)

**Пример мест, где могут быть hardcoded категории:**
- Select/Dropdown компоненты
- Фильтры в отчётах
- Легенды в графиках
- Цветовые маппинги (например, `categoryColors = { productive: '#10b981', ... }`)

**Новый подход:**
- Категории грузятся из `/api/category-types` динамически
- Цвета берутся из `category.color` (если есть)
- Если `color` отсутствует — использовать дефолтный цвет или генерировать случайный

---

### 4. Обновление TypeScript типов

**Создать/обновить файлы типов:**

```typescript
// types/category.ts
export interface CategoryType {
  id: string;
  key: string;
  name: string;
  color?: string;
  sort_order?: number;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

// types/process-catalog.ts
export interface ProcessCatalogEntry {
  id: string;
  friendly_name: string;
  process_names: string[];
  window_title_patterns: string[];
  category_id: string;           // ИЗМЕНЕНО: раньше было category: string
  category?: CategoryType;       // НОВОЕ: вложенный объект
  is_active: boolean;
  created_at: string;
  updated_at: string;
}
```

---

### 5. Проверки для production-ready решения

#### Обработка ошибок
- [ ] При ошибке загрузки категорий (`/api/category-types`) — показать toast/alert с кнопкой "Повторить"
- [ ] При ошибке создания/обновления/удаления — показать понятное сообщение (не просто "500 error")
- [ ] Если backend вернул `404` на `/api/category-types` — показать placeholder: "Категории не найдены. Обратитесь к администратору."

#### Валидация
- [ ] Поле `key` категории: проверка уникальности, lowercase, только [a-z0-9-_]
- [ ] Поле `name` категории: обязательное, минимум 2 символа
- [ ] Поле `category_id` в правиле: обязательное, должно существовать в списке категорий
- [ ] Массивы `process_names` и `window_title_patterns`: хотя бы один элемент должен быть заполнен

#### Loading states
- [ ] Скелетоны/спиннеры при загрузке списков категорий и правил
- [ ] Disabled состояние кнопок во время мутаций (создание/обновление/удаление)

#### Empty states
- [ ] Если категорий нет — показать placeholder с кнопкой "Создать категорию"
- [ ] Если правил нет — показать placeholder с кнопкой "Добавить правило"

#### Responsive design
- [ ] Таблицы должны корректно отображаться на мобильных устройствах (responsive/scroll)
- [ ] Формы должны быть адаптивными

#### Accessibility
- [ ] Все кнопки и инпуты с правильными aria-labels
- [ ] Формы с label для каждого поля
- [ ] Модальные окна с правильным фокусом (trap focus)

#### Оптимизация
- [ ] React Query для кеширования запросов (`staleTime`, `cacheTime`)
- [ ] Оптимистичные обновления при мутациях (если требуется)
- [ ] Debounce для поиска/фильтрации (если добавляется)

#### Тестирование (опционально, но рекомендуется)
- [ ] Юнит-тесты для компонентов форм категорий
- [ ] Интеграционные тесты для API calls (mock `/api/category-types`)

---

## Примеры UI компонентов (псевдокод)

### CategoryTypesList.tsx
```tsx
import { useQuery, useMutation } from '@tanstack/react-query';
import { Table, Button, Dialog, Form } from '@/components/ui';

export function CategoryTypesList() {
  const { data: categories, isLoading } = useQuery(['category-types'], fetchCategories);
  const deleteMutation = useMutation(deleteCategory, {
    onSuccess: () => queryClient.invalidateQueries(['category-types'])
  });

  return (
    <div>
      <div className="flex justify-between items-center mb-4">
        <h2>Категории приложений</h2>
        <Button onClick={() => openCreateDialog()}>+ Добавить категорию</Button>
      </div>
      
      {isLoading ? (
        <Skeleton />
      ) : categories.length === 0 ? (
        <EmptyState message="Категорий ещё нет" actionLabel="Создать" onAction={openCreateDialog} />
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Название</TableHead>
              <TableHead>Ключ</TableHead>
              <TableHead>Статус</TableHead>
              <TableHead>Действия</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {categories.map(cat => (
              <TableRow key={cat.id}>
                <TableCell>
                  <div className="flex items-center gap-2">
                    <div className="w-3 h-3 rounded-full" style={{ backgroundColor: cat.color }} />
                    {cat.name}
                  </div>
                </TableCell>
                <TableCell><code>{cat.key}</code></TableCell>
                <TableCell>{cat.is_active ? 'Активна' : 'Неактивна'}</TableCell>
                <TableCell>
                  <Button onClick={() => openEditDialog(cat)}>Редактировать</Button>
                  <Button variant="destructive" onClick={() => deleteMutation.mutate(cat.id)}>
                    Удалить
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  );
}
```

### ProcessCatalogForm.tsx (фрагмент)
```tsx
export function ProcessCatalogForm({ rule }: { rule?: ProcessCatalogEntry }) {
  const { data: categories } = useQuery(['category-types'], fetchCategories);
  const form = useForm({
    defaultValues: {
      friendly_name: rule?.friendly_name || '',
      process_names: rule?.process_names || [],
      window_title_patterns: rule?.window_title_patterns || [],
      category_id: rule?.category_id || '',
      is_active: rule?.is_active ?? true,
    }
  });

  return (
    <Form {...form}>
      <FormField name="friendly_name" label="Название приложения" required />
      <FormField name="process_names" label="Имена процессов (массив)" required />
      <FormField name="window_title_patterns" label="Паттерны заголовков окон" />
      
      <FormField name="category_id" label="Категория" required>
        <Select value={form.watch('category_id')} onValueChange={val => form.setValue('category_id', val)}>
          {categories?.map(cat => (
            <SelectItem key={cat.id} value={cat.id}>
              <div className="flex items-center gap-2">
                <div className="w-3 h-3 rounded-full" style={{ backgroundColor: cat.color }} />
                {cat.name}
              </div>
            </SelectItem>
          ))}
        </Select>
      </FormField>
      
      <Button type="submit">Сохранить</Button>
    </Form>
  );
}
```

---

## Acceptance Criteria (критерии приемки)

### A. Категории (Category Types)
1. Пользователь видит страницу "Категории приложений" и список категорий загружается из `GET /api/category-types`.
2. Пользователь может создать категорию (POST) и она появляется в списке без перезагрузки страницы.
3. Пользователь может отредактировать категорию (PUT) и изменения отображаются в списке без перезагрузки.
4. Пользователь может удалить категорию (DELETE/soft delete):
   - категория пропадает из списка при фильтре "только активные";
   - при переключении "показать все" — категория видна как неактивная.
5. Поля валидируются:
   - `name` обязателен;
   - `key` обязателен, в формате `^[a-z0-9-_]+$`, lowercase, без пробелов;
   - UI предотвращает отправку формы при ошибках.
6. Ошибки API не приводят к падению страницы: отображается сообщение пользователю + возможность повторить действие.

### B. Справочник программ (Process Catalog)
1. Страница "Справочник программ" загружает правила из `GET /api/process-catalog`.
2. В форме создания/редактирования правила поле "Категория" — dropdown со значениями из `GET /api/category-types`.
3. При сохранении правила отправляется `category_id` (UUID), а не строка.
4. В таблице правил отображается человекочитаемое имя категории (`category.name`) и цвет (если есть).
5. Если категория правила не найдена (удалена/неактивна) — UI показывает предупреждение и предлагает выбрать другую категорию.

### C. Устранение дублирования
1. UI больше не вызывает `/api/categories` (старый эндпоинт) и не содержит hardcoded списка категорий.
2. На странице "Категории приложений" отсутствует маппинг приложения к категории (только CRUD категорий).

---

## Сценарии ручного тестирования (Given/When/Then)

### 1) Создание категории
**Given** пользователь открыл страницу "Категории приложений" и список загрузился  
**When** пользователь нажимает "+ Добавить категорию", вводит `name=Системная`, `key=system`, выбирает цвет и сохраняет  
**Then** категория появляется в списке, без перезагрузки страницы, и доступна в dropdown на странице "Справочник программ".

### 2) Валидация key
**Given** открыта форма создания категории  
**When** пользователь вводит `key=System Category` (пробелы/uppercase) и нажимает "Сохранить"  
**Then** UI показывает ошибку валидации, запрос в backend не отправляется.

### 3) Конфликт key (уникальность)
**Given** существует категория с `key=productive`  
**When** пользователь пытается создать новую категорию с таким же `key=productive`  
**Then** UI показывает ошибку (из backend 409 или ошибка валидации), категория не создается.

### 4) Удаление категории, используемой в правилах
**Given** есть правило в "Справочник программ" с категорией `system`  
**When** пользователь удаляет категорию `system` на странице категорий  
**Then** в списке правил категория отображается как "Не указано"/"Категория удалена" и UI требует выбрать новую категорию при редактировании.

### 5) Создание правила в справочнике программ
**Given** есть минимум 1 категория в `/api/category-types`  
**When** пользователь создает правило, выбирает категорию в dropdown и сохраняет  
**Then** правило появляется в списке, и в колонке "Категория" отображается имя категории.

### 6) Ошибка сети при загрузке категорий
**Given** backend временно недоступен или `/api/category-types` возвращает 500  
**When** пользователь открывает страницу категорий  
**Then** UI показывает ошибку + кнопку "Повторить", приложение не падает.

### 7) Loading/disabled состояния
**Given** пользователь нажал "Сохранить" в форме категории  
**When** запрос выполняется  
**Then** кнопка disabled, показывается индикатор загрузки; повторное нажатие не отправляет дублирующий запрос.

---

## Чеклист перед деплоем

### Backend готовность
- [ ] `/api/category-types` возвращает данные в нужном формате
- [ ] `/api/process-catalog` возвращает `category_id` и вложенный `category` объект
- [ ] Старый эндпоинт `/api/categories` удалён или не используется

### Frontend готовность
- [ ] Страница "Категории приложений" реализована (CRUD категорий)
- [ ] Страница "Справочник программ" обновлена (dropdown категорий из `/api/category-types`)
- [ ] Все обращения к `/api/categories` заменены на `/api/category-types` или `/api/process-catalog`
- [ ] Удалены hardcoded массивы категорий из кода
- [ ] TypeScript типы обновлены (`CategoryType`, `ProcessCatalogEntry`)
- [ ] Обработка ошибок реализована
- [ ] Loading/Empty states реализованы
- [ ] Валидация форм работает
- [ ] UI responsive и доступен (accessibility)

### Тестирование
- [ ] Создание категории работает
- [ ] Редактирование категории работает
- [ ] Удаление категории работает (soft delete)
- [ ] Создание правила с выбором категории работает
- [ ] Редактирование правила с изменением категории работает
- [ ] Удаление правила работает
- [ ] Отчёты корректно отображают категории (проверить на странице отчётов)

---

## Дополнительные замечания для loveable.dev

1. **Использовать существующие компоненты shadcn/ui**: не создавать кастомные компоненты с нуля, если есть готовые (Table, Dialog, Form, Select, Button, Input).
2. **React Query обязательно**: для всех API запросов использовать `useQuery` и `useMutation` с правильной инвалидацией кеша.
3. **Обработка ошибок**: использовать toast/notification для уведомлений (успех/ошибка).
4. **Консистентность**: стиль и UX должны совпадать с остальными страницами настроек.
5. **Комментарии в коде**: добавить JSDoc/комментарии для сложных функций.
6. **Не забыть про типы**: все API ответы должны быть типизированы (TypeScript).

---

## Контакты для вопросов
Если требуются уточнения по API или формату данных — создать issue в репозитории или обратиться к backend-разработчику.

**Backend API base URL:** `http://monitor.net.gslaudit.ru/api`  
**Frontend repo:** `frontend/` (git submodule)  
**Stack:** React, TypeScript, shadcn/ui, React Query, Axios/fetch

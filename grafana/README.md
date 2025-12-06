# Grafana Dashboard для мониторинга активности сотрудников

## Предварительные требования

1. Установленная Grafana (версия 9.0+)
2. Плагин `grafana-clickhouse-datasource`

## Шаг 1: Установка плагина ClickHouse

```bash
grafana-cli plugins install grafana-clickhouse-datasource
systemctl restart grafana-server
```

## Шаг 2: Настройка Datasource

1. Откройте Grafana → Configuration → Data Sources → Add data source
2. Выберите **ClickHouse**
3. Заполните настройки:

| Параметр | Значение |
|----------|----------|
| Name | ClickHouse |
| Server address | IP адрес сервера ClickHouse |
| Server port | 9000 |
| Protocol | Native |
| Username | monitor_user |
| Password | (ваш пароль) |
| Default database | monitoring |

4. Нажмите **Save & Test**

## Шаг 3: Создание SQL Views (опционально)

Выполните SQL из файла `clickhouse/03-grafana-views.sql` в ClickHouse для создания вспомогательных views:

```bash
clickhouse-client --multiquery < clickhouse/03-grafana-views.sql
```

## Шаг 4: Импорт Dashboard

1. Откройте Grafana → Dashboards → Import
2. Загрузите файл `grafana/dashboards/employee-activity.json`
3. Выберите datasource **ClickHouse**
4. Нажмите **Import**

## Описание Dashboard "Активность сотрудника"

### Переменные
- **$employee** — выбор сотрудника из списка

### Панели

#### Сводка за день
- **Активное время** — общее время активной работы
- **Время простоя** — время бездействия (idle)
- **Продуктивное время** — время в продуктивных приложениях
- **Приложений использовано** — количество уникальных приложений

#### Использование приложений
- **Pie chart: Время по приложениям** — распределение времени по приложениям (с дружественными именами)
- **Pie chart: Время по категориям** — продуктивно/непродуктивно/нейтрально/коммуникации/развлечения

#### Детальная статистика
- **Bar chart: Топ приложений** — горизонтальная диаграмма топ-20 приложений

#### Хронология активности
- **Table: Хронология событий** — детальная таблица с временем, приложением, заголовком окна, статусом и категорией

#### Активность по часам
- **Time series** — распределение активности и простоя по часам дня

## Цветовая схема категорий

| Категория | Цвет | Описание |
|-----------|------|----------|
| productive | Зеленый | Рабочие приложения (1C, Excel, IDE) |
| unproductive | Красный | Нерабочие приложения (игры) |
| neutral | Желтый | Нейтральные (проводник, блокнот) |
| communication | Синий | Коммуникации (почта, мессенджеры) |
| entertainment | Фиолетовый | Развлечения (YouTube, соцсети) |

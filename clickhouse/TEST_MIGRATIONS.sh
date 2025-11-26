#!/bin/bash
# Тестирование миграций ClickHouse

echo "========================================="
echo "Testing ClickHouse Migrations"
echo "========================================="
echo ""

# Цвета для вывода
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Проверка что Docker запущен
if ! docker ps > /dev/null 2>&1; then
    echo -e "${RED}❌ Docker не запущен!${NC}"
    exit 1
fi

# Проверка что контейнер существует
if ! docker ps -a | grep -q monitoring-clickhouse; then
    echo -e "${YELLOW}⚠️  Контейнер monitoring-clickhouse не найден${NC}"
    echo "Запускаем docker-compose..."
    docker-compose up -d clickhouse
    sleep 10
fi

# Проверка что контейнер запущен
if ! docker ps | grep -q monitoring-clickhouse; then
    echo -e "${RED}❌ Контейнер monitoring-clickhouse не запущен!${NC}"
    echo "Запускаем..."
    docker-compose start clickhouse
    sleep 10
fi

echo "1️⃣  Проверка подключения к ClickHouse..."
if docker exec monitoring-clickhouse clickhouse-client --query "SELECT 1" > /dev/null 2>&1; then
    echo -e "${GREEN}✅ ClickHouse доступен${NC}"
else
    echo -e "${RED}❌ Не могу подключиться к ClickHouse!${NC}"
    exit 1
fi

echo ""
echo "2️⃣  Проверка базы данных monitoring..."
if docker exec monitoring-clickhouse clickhouse-client --query "SHOW DATABASES" | grep -q monitoring; then
    echo -e "${GREEN}✅ База данных monitoring существует${NC}"
else
    echo -e "${RED}❌ База данных monitoring не найдена!${NC}"
    exit 1
fi

echo ""
echo "3️⃣  Проверка таблиц..."
TABLES=$(docker exec monitoring-clickhouse clickhouse-client --database monitoring --query "SHOW TABLES" | wc -l)
echo -e "   Найдено таблиц: ${GREEN}$TABLES${NC}"

EXPECTED_TABLES=(
    "activity_events"
    "activity_segments"
    "alerts"
    "application_categories"
    "agent_configs"
    "employees"
    "file_copy_events"
    "keyboard_events"
    "process_catalog"
    "screenshot_metadata"
    "system_settings"
    "usb_events"
)

MISSING_TABLES=()
for table in "${EXPECTED_TABLES[@]}"; do
    if docker exec monitoring-clickhouse clickhouse-client --database monitoring \
        --query "EXISTS TABLE monitoring.$table" | grep -q 1; then
        echo -e "   ✅ $table"
    else
        echo -e "   ${RED}❌ $table${NC}"
        MISSING_TABLES+=("$table")
    fi
done

if [ ${#MISSING_TABLES[@]} -gt 0 ]; then
    echo ""
    echo -e "${RED}⚠️  Отсутствующие таблицы:${NC}"
    for table in "${MISSING_TABLES[@]}"; do
        echo "   - $table"
    done
fi

echo ""
echo "4️⃣  Проверка application_categories..."
CATEGORIES_COUNT=$(docker exec monitoring-clickhouse clickhouse-client --database monitoring \
    --query "SELECT count(*) FROM application_categories" 2>/dev/null)

if [ -z "$CATEGORIES_COUNT" ]; then
    echo -e "${RED}❌ Таблица application_categories не найдена или пуста!${NC}"
    echo ""
    echo "Применяю миграции вручную..."
    cat clickhouse/01-schema.sql | docker exec -i monitoring-clickhouse clickhouse-client --database monitoring
    cat clickhouse/02-seed-data.sql | docker exec -i monitoring-clickhouse clickhouse-client --database monitoring
    
    CATEGORIES_COUNT=$(docker exec monitoring-clickhouse clickhouse-client --database monitoring \
        --query "SELECT count(*) FROM application_categories" 2>/dev/null)
fi

if [ "$CATEGORIES_COUNT" -gt 0 ]; then
    echo -e "   ${GREEN}✅ Найдено категорий: $CATEGORIES_COUNT${NC}"
    
    echo ""
    echo "   Распределение по типам:"
    docker exec monitoring-clickhouse clickhouse-client --database monitoring --query \
        "SELECT category, count(*) as count FROM application_categories GROUP BY category ORDER BY category FORMAT Pretty"
else
    echo -e "${RED}❌ Категории не загружены!${NC}"
fi

echo ""
echo "5️⃣  Проверка материализованных представлений..."
VIEWS=$(docker exec monitoring-clickhouse clickhouse-client --database monitoring \
    --query "SHOW TABLES" | grep -E "activity_stats_hourly|daily_activity_summary|program_usage_daily" | wc -l)

if [ "$VIEWS" -eq 3 ]; then
    echo -e "${GREEN}✅ Все 3 materialized views созданы${NC}"
else
    echo -e "${YELLOW}⚠️  Найдено views: $VIEWS (ожидалось 3)${NC}"
fi

echo ""
echo "6️⃣  Проверка индексов..."
INDEXES=$(docker exec monitoring-clickhouse clickhouse-client --database monitoring \
    --query "SELECT count(*) FROM system.data_skipping_indices WHERE database = 'monitoring'")
echo -e "   Найдено индексов: ${GREEN}$INDEXES${NC}"

echo ""
echo "========================================="
echo "Итоговый отчёт:"
echo "========================================="

if [ ${#MISSING_TABLES[@]} -eq 0 ] && [ "$CATEGORIES_COUNT" -gt 50 ] && [ "$VIEWS" -eq 3 ]; then
    echo -e "${GREEN}✅ ВСЕ ПРОВЕРКИ ПРОЙДЕНЫ!${NC}"
    echo ""
    echo "Справочник программ должен работать без ошибок."
    exit 0
else
    echo -e "${RED}❌ ОБНАРУЖЕНЫ ПРОБЛЕМЫ${NC}"
    echo ""
    if [ ${#MISSING_TABLES[@]} -gt 0 ]; then
        echo "- Отсутствуют таблицы: ${MISSING_TABLES[*]}"
    fi
    if [ "$CATEGORIES_COUNT" -lt 50 ]; then
        echo "- Недостаточно категорий: $CATEGORIES_COUNT (ожидалось > 50)"
    fi
    if [ "$VIEWS" -ne 3 ]; then
        echo "- Не все materialized views созданы: $VIEWS (ожидалось 3)"
    fi
    echo ""
    echo "Запустите миграции вручную:"
    echo "  cat clickhouse/01-schema.sql | docker exec -i monitoring-clickhouse clickhouse-client --database monitoring"
    echo "  cat clickhouse/02-seed-data.sql | docker exec -i monitoring-clickhouse clickhouse-client --database monitoring"
    exit 1
fi
